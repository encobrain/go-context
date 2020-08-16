package context

import (
	"fmt"
	"path/filepath"
	"regexp"
	"runtime"
	"runtime/debug"
	"sync"

	"github.com/encobrain/go-emitter"
)

var LIBDIR_RE *regexp.Regexp
var contexts = map[uint64]*context{}
var contexts_mu = sync.RWMutex{}
var gctxID = getGID()
var gctx *context

func init() {
	var ok bool
	_, libDir, _, ok := runtime.Caller(0)

	if !ok {
		panic("Cant get root path")
	}

	libDir = filepath.Dir(libDir)

	LIBDIR_RE = regexp.MustCompile(regexp.QuoteMeta(libDir) + "/\\w+\\.go:\\d+\\n\\s*")

	gctx = contextCreate(gctxID, &context{
		childs:        map[uint64]*context{},
		panicHandler:  getDefaultPanicHandler(),
		vars:          &sync.Map{},
		vars_em:       &emitter.Emitter{},
		closeHandlers: &[]*func(){},
	})

	gctx.state = STATE_RUNNING
}

func getDefaultPanicHandler() *func(err interface{}) {
	handler := func(err interface{}) {
		fmt.Printf("UNCAUGHT PANIC: %s\n%s\n", err, debug.Stack())
	}

	return &handler
}

func contextCreate(routineID uint64, parCtx *context) (ctx *context) {
	ctx = &context{
		id:            routineID,
		parent:        parCtx,
		childs:        map[uint64]*context{},
		panicHandler:  parCtx.panicHandler,
		vars:          parCtx.vars,
		vars_em:       parCtx.vars_em,
		closeHandlers: parCtx.closeHandlers,
	}

	if parCtx.id == gctxID {
		ctx.closeHandlers = &[]*func(){}
	}

	contexts_mu.Lock()
	contexts[routineID] = ctx
	contexts_mu.Unlock()

	return
}

func contextGet(routineID uint64, defaultCtx *context) (ctx *context) {
	contexts_mu.RLock()
	ctx = contexts[routineID]
	contexts_mu.RUnlock()

	if ctx == nil {
		ctx = defaultCtx
	}

	return
}

func contextDelete(routineID uint64) {
	contexts_mu.Lock()
	delete(contexts, routineID)
	contexts_mu.Unlock()
}
