package context

import (
	"sync"
	"fmt"
	"runtime/debug"
	"sync/atomic"
	
	"github.com/t-yuki/goid"
	"github.com/olebedev/emitter"
)

type context struct {
	panicHandler 	func (err interface{})
  	vars 			map[string]interface{}
  	subRuns 		sync.WaitGroup
  	runs 			*int64
  	closeHandlers	*[]*func()
	emitter 		*emitter.Emitter
	
  	sync.RWMutex
}

var contexts = map[int64]*context{}
var mu 		 = sync.RWMutex{}
var gctx	 = createContext(goid.GoID(), nil)

func createContext (routineID int64, prevContext *context) (ctx *context) {
   	if prevContext != nil {
   		ctx = &context{
   			panicHandler: 	prevContext.panicHandler,
   			vars:			prevContext.vars,
   			runs:			prevContext.runs,
   			closeHandlers:  prevContext.closeHandlers,
   			emitter: 		prevContext.emitter,
		}
	} else {
		runs := int64(0)
		closeHandlers := make([]*func(),0)
		
		ctx = &context{
			vars: 			map[string]interface{}{},
			runs: 			&runs,
			closeHandlers: 	&closeHandlers,
			emitter: 		&emitter.Emitter{},
		}
	}

	mu.Lock()
	contexts[routineID] = ctx
   	mu.Unlock()

   	return 
}

func getContext (routineID int64) (ctx *context) {
	mu.RLock()
	ctx = contexts[routineID]
	mu.RUnlock()

	return
}

func deleteContext (routineID int64) {
  	mu.Lock()
  	delete(contexts, routineID)
  	mu.Unlock()
}

// Gets variable from current context
func Get (name string) interface{} {
   	ctx := getContext(goid.GoID())

	if ctx == nil { ctx = gctx }
   	
	ctx.RLock()
	v := ctx.vars[name]
	ctx.RUnlock()
	
	return v
}

// Sets variable to current context
func Set (name string, value interface{}) {
	ctx := getContext(goid.GoID())

	if ctx == nil { ctx = gctx }

	ctx.Lock()
	ctx.vars[name] = value
	ctx.Unlock()

	ctx.emitter.Emit("SET:"+name, value)
}

// Subscribes on event variable set
func OnSet (name string) <-chan emitter.Event {
	ctx := getContext(goid.GoID())

	if ctx == nil { ctx = gctx }

	return ctx.emitter.On("SET:"+name)
}

// Removes susbscribe on event variable set
func OffSet (name string, ch <-chan emitter.Event) {
	ctx := getContext(goid.GoID())

	if ctx == nil { ctx = gctx }

	ctx.emitter.Off("SET:"+name, ch)
}

// Runs go routine with context.
// Uses global context if not exists before
// affects on context.Wait() and run CloseHandlers.
func Run (routine func ()) {
	pctx := getContext(goid.GoID())
	
	if pctx == nil { pctx = gctx }

	pctx.subRuns.Add(1)
	atomic.AddInt64(pctx.runs, 1)

   	go func() {
		routineID := goid.GoID()
		ctx := createContext(routineID, pctx)
		defer deleteContext(routineID)

		atomic.AddInt64(ctx.runs, 1)

		defer pctx.subRuns.Done()
		
		defer func() {
			runs := atomic.AddInt64(ctx.runs, -1)

			if runs < 0 { panic(fmt.Errorf("Runs count negative : %d", runs)) }

			if runs != 0 { return }

			ctx.Lock(); defer ctx.Unlock()

			l := len(*ctx.closeHandlers)

			if l == 0 { return  }

			l--
			ph := (*ctx.closeHandlers)[l]
			*ctx.closeHandlers = (*ctx.closeHandlers)[:l]

			Run(*ph)

			ctx.subRuns.Wait()
		}()

		defer atomic.AddInt64(pctx.runs,-1)

   		defer func() {
			err := recover()

   			if err == nil { return }

			ctx.RLock()
			panicHandler := ctx.panicHandler
			ctx.RUnlock()

			if panicHandler != nil {
				defer func() {
					perr := recover()

					if perr != nil {
						fmt.Printf("ROUTINE PANIC: %s\n", err)
						fmt.Printf("PANIC_HANDLER's PANIC: %s\n%s\n", perr, debug.Stack())
					}
				}()

				panicHandler(err)
			} else {
				fmt.Printf("UNCAUGHT PANIC: %s\n%s\n", err, debug.Stack())
			}
		}()

   		routine()
	}()
}

// Runs go routine with context.
// Uses global context if not exists before & set new vars.
// Does not affects on context.Wait() and run CloseHandlers.
func RunHeir (routine func()) {
	pctx := getContext(goid.GoID())

	if pctx == nil { pctx = gctx }

	go func() {
		routineID := goid.GoID()
		ctx := createContext(routineID, pctx)
		defer deleteContext(routineID)

		if pctx == gctx { ctx.vars = map[string]interface{}{} }

		defer func() {
			err := recover()

			if err == nil { return }

			ctx.RLock()
			panicHandler := ctx.panicHandler
			ctx.RUnlock()

			if panicHandler != nil {
				defer func() {
					perr := recover()

					if perr != nil {
						fmt.Printf("ROUTINE PANIC: %s\n", err)
						fmt.Printf("PANIC_HANDLER's PANIC: %s\n%s\n", perr, debug.Stack())
					}
				}()

				panicHandler(err)
			} else {
				fmt.Printf("UNCAUGHT PANIC: %s\n%s\n", err, debug.Stack())
			}
		}()

		routine()
	}()
}

// Waits for end all sub runs
func Wait () {
	ctx := getContext(goid.GoID())

	if ctx == nil { ctx = gctx }

	ctx.subRuns.Wait()
}

// Sets panic handler & returns previous handler
func SetPanicHandler (handler func (err interface{})) func(err interface{}) {
	ctx := getContext(goid.GoID())

	if ctx == nil { ctx = gctx }

	ctx.Lock()
	prevHandler := ctx.panicHandler
	ctx.panicHandler = handler
	ctx.Unlock()

	return prevHandler
}

// Adds close handler.
// Close handler will runs after ends all runs in context
func AddCloseHandler (handler *func()) {
	ctx := getContext(goid.GoID())

	if ctx == nil { ctx = gctx }
	
	ctx.Lock(); defer ctx.Unlock()
	
   	for _,h := range *ctx.closeHandlers {
   		if h == handler { return }
	}

	*ctx.closeHandlers = append(*ctx.closeHandlers, handler)
}

// removes close handler
func RemoveCloseHandler (handler *func()) {
	ctx := getContext(goid.GoID())

	if ctx == nil { ctx = gctx }

	ctx.Lock(); defer ctx.Unlock()

	for i,h := range *ctx.closeHandlers {
		if h == handler {
			*ctx.closeHandlers = append((*ctx.closeHandlers)[:i], (*ctx.closeHandlers)[i+1:]...)
			break
		}
	}
}

// Separates current context from parent.
// WARN!!! Should call BEFORE use any Runs.
// Sets in current context new runs & vars & closeHandlers & emitter.
func Separate () {
	ctx := getContext(goid.GoID())

	if ctx == nil { panic("Context not runned") }

	ctx.Lock(); defer ctx.Unlock()

	ctx.vars = map[string]interface{}{}

	atomic.AddInt64(ctx.runs, -1)
	runs := int64(1)
	ctx.runs = &runs

	closeHandlers := make([]*func(),0)
	ctx.closeHandlers = &closeHandlers

	ctx.emitter = &emitter.Emitter{}
}

