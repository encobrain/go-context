package context

import (
	"fmt"
	"sort"
	"sync/atomic"
	"strings"
	"reflect"
	"runtime"
	
	"github.com/t-yuki/goid"
	
	"github.com/encobrain/go-emitter"
)

// Gets variable from current context
func Get (varName string) interface{} {
	return contextGet(goid.GoID(), gctx).get(varName)
}

// Sets variable to current context
func Set (varName string, value interface{}) {
	contextGet(goid.GoID(), gctx).set(varName, value)
	return
}

// Subscribes on event variable set
func OnSet (varName string) chan emitter.Event {
	return contextGet(goid.GoID(), gctx).onSet(varName)
}

// Removes susbscribe on event variable set
func OffSet (varName string, ch chan emitter.Event) {
	contextGet(goid.GoID(), gctx).offSet(varName, ch)
	return
}

// Runs go routine with context.
// Uses global context if not exists before
func Run (routine func ()) {
	contextGet(goid.GoID(), gctx).run(routine)
}

// Waits for end all sub runs
func Wait () <-chan emitter.Event {
	return contextGet(goid.GoID(), gctx).wait()
}

// Sets global panic handler & returns previous handler.
func SetGPanicHandler (handler func (err interface{})) (prevHandler func(err interface{})) {
	return contextGet(goid.GoID(), gctx).setPanicHandler(handler, false)
}

// Sets local panic handler & returns previous handler.
// It will catch panics in current & all runs from current context
func SetLPanicHandler (handler func (err interface{})) (prevHandler func(err interface{})) {
	return contextGet(goid.GoID(), gctx).setPanicHandler(handler, true)
}

// Adds close handler.
// It will runs after ends all runs in all context
func AddCloseHandler (handler *func()) {
	contextGet(goid.GoID(), gctx).addCloseHandler(handler, false)
}

// Adds local close handler.
// It will runs after ends all runs in current context
func AddLCloseHandler (handler *func()) {
	contextGet(goid.GoID(), gctx).addCloseHandler(handler, true)
}

// removes close handler
func RemoveCloseHandler (handler *func()) {
	contextGet(goid.GoID(), gctx).removeCloseHandler(handler)
}

// Separates current context from parent.
// WARN!!! Should call BEFORE use any Runs.
// Sets in current context new runs & vars & closeHandlers & emitter.
func Separate () {
	ctx := contextGet(goid.GoID(), nil)

	if ctx == nil { panic("Context not runned") }

	ctx.separate()
}


func getRoutineInfo (ctx *context) (file string,line int)  {
	pc := reflect.ValueOf(ctx.routine).Pointer()
	fn := runtime.FuncForPC(pc)

	file,line = fn.FileLine(pc)
	file = strings.Replace(file, ROOTPATH, "", 1)

	return
}

func getRunning (ctx *context, par string) (running []string) {
	ctx.Lock()

	f,l := getRoutineInfo(ctx)

	if par != "" {
		st := " "; if atomic.LoadInt32(&ctx.state) == STATE_RUNNING { st = "▸" }
		running = append(running, fmt.Sprintf("%s %s.%d  %s:%d",st, par,ctx.id, f,l))
	}

	var ids []int64
	var infs = map[int64][]string{}

	for id,c := range ctx.childs {
		ids = append(ids,id)
		infs[id] = getRunning(c, fmt.Sprintf("%s.%d",par,ctx.id))
	}

	sort.Slice(ids, func(i, j int) bool { return ids[i]<ids[j] })

	for _,id := range ids {
		running = append(running, infs[id]...)
	}

	ctx.Unlock()

	return
}

// Gets running contexts as routineID... file:line
func GetRunning () (running []string) {
	ctx := contextGet(goid.GoID(), gctx)

	return getRunning(ctx, "")
}
