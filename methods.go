package context

import (
	"fmt"
	"reflect"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync/atomic"

	"github.com/encobrain/go-emitter"
)

// Gets variable from current context
func Get(varName string) interface{} {
	return contextGet(getGID(), gctx).get(varName)
}

// Sets variable to current context
func Set(varName string, value interface{}) (setStatus chan emitter.EmitStatus) {
	return contextGet(getGID(), gctx).set(varName, value)
}

// Subscribes on event variable set
func OnSet(varName string) chan emitter.Event {
	return contextGet(getGID(), gctx).onSet(varName)
}

// Removes susbscribe on event variable set
func OffSet(varName string, ch chan emitter.Event) {
	contextGet(getGID(), gctx).offSet(varName, ch)
	return
}

// Runs go routine with context.
// Uses global context if not exists before
func Run(routine func()) {
	contextGet(getGID(), gctx).run(routine)
}

// Waits for end all sub runs
func Wait() <-chan emitter.Event {
	return contextGet(getGID(), gctx).wait()
}

// Sets global panic handler & returns previous handler.
func SetGPanicHandler(handler func(err interface{})) (prevHandler func(err interface{})) {
	return contextGet(getGID(), gctx).setPanicHandler(handler, false)
}

// Sets local panic handler & returns previous handler.
// It will catch panics in current & all runs from current context
func SetLPanicHandler(handler func(err interface{})) (prevHandler func(err interface{})) {
	return contextGet(getGID(), gctx).setPanicHandler(handler, true)
}

// Adds close handler.
// It will runs after ends all runs in all context
func AddCloseHandler(handler *func()) {
	contextGet(getGID(), gctx).addCloseHandler(handler, false)
}

// Adds local close handler.
// It will runs after ends all runs in current context
func AddLCloseHandler(handler *func()) {
	contextGet(getGID(), gctx).addCloseHandler(handler, true)
}

// removes close handler
func RemoveCloseHandler(handler *func()) {
	contextGet(getGID(), gctx).removeCloseHandler(handler)
}

// Separates current context from parent.
// WARN!!! Should call BEFORE use any Runs.
// Sets in current context new runs & vars & closeHandlers & emitter.
func Separate() {
	ctx := contextGet(getGID(), nil)

	if ctx == nil {
		panic("Context not runned")
	}

	ctx.separate()
}

func getRoutineInfo(ctx *context) (file string, line int) {
	pc := reflect.ValueOf(ctx.routine).Pointer()
	fn := runtime.FuncForPC(pc)

	file, line = fn.FileLine(pc)
	file = strings.Replace(file, ROOTPATH, "", 1)

	return
}

var filelinesRe = regexp.MustCompile("/.*?:\\d+")

func getRunning(ctx *context, par string, stacks map[uint64]string) (running []string) {
	ctx.Lock()
	defer ctx.Unlock()

	if par != "" {
		f, l := getRoutineInfo(ctx)
		st := " "
		stack := ""

		if atomic.LoadInt32(&ctx.state) == STATE_RUNNING {
			st = "▸"
			stack = strings.Join(filelinesRe.FindAllString(stacks[ctx.id], -1), "\n     ")

			stack = "\n     " + strings.Replace(stack, ROOTPATH, "", -1)
		}

		running = append(running, fmt.Sprintf("%s %s.%d  %s:%d%s", st, par, ctx.id, f, l, stack))
	}

	var ids []uint64
	var infs = map[uint64][]string{}

	ctx.childs_mu.Lock()
	defer ctx.childs_mu.Unlock()

	for id, c := range ctx.childs {
		ids = append(ids, id)
		infs[id] = getRunning(c, fmt.Sprintf("%s.%d", par, ctx.id), stacks)
	}

	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })

	for _, id := range ids {
		running = append(running, infs[id]...)
	}

	return
}

var stackpathsRe = regexp.MustCompile("(?s)goroutine (\\d+) \\[.*?]:\\n(.*?)\\n\\n")

// Gets running contexts as routineID... file:line
func GetRunning() (running []string) {
	ctx := contextGet(getGID(), gctx)

	buf := make([]byte, 1024)

	for {
		l := runtime.Stack(buf, true)
		if l != len(buf) {
			buf = buf[:l]
			break
		}

		buf = make([]byte, len(buf)*2)
	}

	stacks := map[uint64]string{}

	for _, m := range stackpathsRe.FindAllStringSubmatch(string(buf)+"\n", -1) {
		id, _ := strconv.ParseUint(m[1], 10, 64)
		stacks[id] = m[2]
	}

	return getRunning(ctx, "", stacks)
}
