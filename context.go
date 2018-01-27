package context

import (
	"sync"
	"fmt"
	"runtime/debug"
	"sync/atomic"
	
	"github.com/t-yuki/goid"
	"github.com/olebedev/emitter"
)

const EV_RUNS_DONE		 = "runsDone"
const EV_CLOSED 		 = "closed"
const EV_VARS_SET_PREFIX = "SET:"

// event EV_RUNS_DONE
// event EV_CLOSED
type context struct {
	id int64
	parent        *context
	running 	  int32
	panicHandler  *func (err interface{})

	separated	  bool
	vars          map[string]interface{}
  	vars_em       *emitter.Emitter
  	runs          int64
  	closeHandlers *[]*func()

	emitter.Emitter
  	sync.Mutex
}

func (c *context) separate () {
	c.Lock()
	c.separated=true
	c.vars 	= map[string]interface{}{}
	c.vars_em = &emitter.Emitter{}
	ph := *c.panicHandler
	c.panicHandler = &ph
	c.closeHandlers = &[]*func(){}
	c.Unlock()
}

func (c *context) setPanicHandler (handler func(err interface{}), local bool) (prevHandler func(err interface{})) {
	c.Lock()
	prevHandler = *c.panicHandler

	if local {
		c.panicHandler = &handler
	} else {
		*c.panicHandler = handler
	}

	c.Unlock()
	
	return
}

func (c *context) handlePanic (err interface {}) {

	ctx := c
	handler := ctx.panicHandler

	defer func () {
		err = recover()

		if err != nil {
			for ctx != nil && handler == ctx.panicHandler && !ctx.separated {
				ctx = ctx.parent
			}

			if ctx != nil && ctx.panicHandler != nil && handler != ctx.panicHandler {
				ctx.handlePanic(err)
			} else {
				(*getDefaultPanicHandler())(err)
			}
		}
	}()
	
	(*handler)(err)
}

func (c *context) get (varName string) (value interface{}) {
	c.Lock()
	value = c.vars[varName]
	c.Unlock()
	return 
}

func (c *context) set (varName string, value interface{}) {
	c.Lock()
	c.vars[varName] = value
	c.Unlock()

	<-c.vars_em.Emit(EV_VARS_SET_PREFIX+varName, value)
}

func (c *context) onSet (varName string) <-chan emitter.Event {
	return c.vars_em.On(EV_VARS_SET_PREFIX+varName)
}

func (c *context) offSet (varName string, ch <-chan emitter.Event) {
	c.vars_em.Off(EV_VARS_SET_PREFIX+varName, ch)
}

func (c *context) addCloseHandler (handler *func(), local bool) {
	c.Lock(); defer c.Unlock()

	if local {
		if c.closeHandlers == c.parent.closeHandlers {
			hs := make([]*func(),0)
			c.closeHandlers = &hs
		}
	}

	for _,h := range *c.closeHandlers {
		if h == handler { return }
	}

	*c.closeHandlers = append(*c.closeHandlers, handler)
}

func (c *context) removeCloseHandler (handler *func()) {
	c.Lock(); defer c.Unlock()

	for i,h := range *c.closeHandlers {
		if h == handler {
			*c.closeHandlers = append((*c.closeHandlers)[:i], (*c.closeHandlers)[i+1:]...)
			break
		}
	}
}

func (c *context) close () (closing bool) {
	c.Lock(); defer c.Unlock()

	if c.closeHandlers == c.parent.closeHandlers { return } // not root

	chl := len(*c.closeHandlers)

	if chl != 0 {
		chl--
		h := (*c.closeHandlers)[chl]
		*c.closeHandlers = (*c.closeHandlers)[:chl]

		c.run(*h)
		return true
	}

	return
}

func (c *context) wait () {
	evRunsDone := c.Once(EV_RUNS_DONE)

	go func() {
		if atomic.LoadInt64(&c.runs) == 0 {
			c.Emit(EV_RUNS_DONE)
		}
	}()

	<-evRunsDone
}

func (c *context) run (routine func()) {
	atomic.AddInt64(&c.runs, 1)

	go func() {
		routineID := goid.GoID()
		ctx := contextCreate(routineID, c)
		defer contextDelete(routineID)
		
		defer atomic.StoreInt32(&ctx.running, 0)

		defer ctx.end()
		
		defer func() {
			err := recover()
			if err != nil { ctx.handlePanic(err) }
		}()

		ctx.running = 1
		routine()
	}()
}

func (c *context) end () {
	if atomic.LoadInt64(&c.runs) != 0 { return }

 	if c.close() { return }

 	<-c.Emit(EV_CLOSED)

 	if c == gctx { return }

 	par := c.parent

 	runs := atomic.AddInt64(&par.runs, -1)

 	if runs == 0 { <-par.Emit(EV_RUNS_DONE) }

	if atomic.LoadInt32(&par.running) == 0 { par.end() }
}




var contexts 	= map[int64]*context{}
var contexts_mu = sync.RWMutex{}

func getDefaultPanicHandler () *func (err interface{}) {
	handler := func (err interface{}) {
		fmt.Printf("UNCAUGHT PANIC: %s\n%s\n", err, debug.Stack())
	}

	return &handler
}

var gctx 		= contextCreate(goid.GoID(), &context{
	panicHandler:   getDefaultPanicHandler(),
	vars:			map[string]interface{}{},
	vars_em: 		&emitter.Emitter{},
	closeHandlers:	&[]*func(){},
})

func contextCreate (routineID int64, parCtx *context) (ctx *context) {
	ctx = &context{
		id : routineID,
		parent			: parCtx,
		panicHandler	: parCtx.panicHandler,
		vars			: parCtx.vars,
		vars_em			: parCtx.vars_em,
		closeHandlers	: parCtx.closeHandlers,
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
func OnSet (varName string) <-chan emitter.Event {
	return contextGet(goid.GoID(), gctx).onSet(varName)
}

// Removes susbscribe on event variable set
func OffSet (varName string, ch <-chan emitter.Event) {
	contextGet(goid.GoID(), gctx).offSet(varName, ch)
	return
}

// Runs go routine with context.
// Uses global context if not exists before
func Run (routine func ()) {
	contextGet(goid.GoID(), gctx).run(routine)
}

// Waits for end all sub runs
func Wait () {
	contextGet(goid.GoID(), gctx).wait()
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
