package context

import (
	"strings"
	"sync"
	"fmt"
	"runtime"
	"runtime/debug"

	"github.com/t-yuki/goid"
	
	"github.com/encobrain/go-emitter"
)

var ROOTPATH 	= ""
var contexts 	= map[int64]*context{}
var contexts_mu = sync.RWMutex{}
var gctxID		= goid.GoID()
var gctx 		*context

func init () {
	var ok bool
	_,ROOTPATH,_,ok = runtime.Caller(0)

	if !ok { panic("Cant get root path") }

	ROOTPATH = strings.Replace(ROOTPATH, "github.com/encobrain/go-context/context.go", "", 1)

	gctx = contextCreate(gctxID, &context{
		childs: 		map[int64]*context{},
		panicHandler:   getDefaultPanicHandler(),
		vars:			&sync.Map{},
		vars_em: 		&emitter.Emitter{},
		closeHandlers:	&[]*func(){},
	})

	gctx.state = STATE_RUNNING
}

func getDefaultPanicHandler () *func (err interface{}) {
	handler := func (err interface{}) {
		fmt.Printf("UNCAUGHT PANIC: %s\n%s\n", err, debug.Stack())
	}

	return &handler
}

func contextCreate (routineID int64, parCtx *context) (ctx *context) {
	ctx = &context{
		id : routineID,
		parent			: parCtx,
		childs 			: map[int64]*context{},
		panicHandler	: parCtx.panicHandler,
		vars			: parCtx.vars,
		vars_em			: parCtx.vars_em,
		closeHandlers	: parCtx.closeHandlers,
	}

	if parCtx.id == gctxID {
		ctx.closeHandlers = &[]*func(){}
	}

	contexts_mu.Lock()
	contexts[routineID] = ctx
	contexts_mu.Unlock()

	return
}

func contextGet (routineID int64, defaultCtx *context) (ctx *context) {
	contexts_mu.RLock()
	ctx = contexts[routineID]
	contexts_mu.RUnlock()

	if ctx == nil { ctx = defaultCtx }

	return
}

func contextDelete (routineID int64) {
	contexts_mu.Lock()
	delete(contexts, routineID)
	contexts_mu.Unlock()
}
