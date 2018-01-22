package context

import (
	"sync"
	"fmt"
	"runtime/debug"
	"sync/atomic"
	
	"github.com/t-yuki/goid"
	"github.com/encobrain/emitter"
)

type context struct {
	panicHandler 	func (err interface{})
  	vars 			map[string]interface{}
  	subRuns 		sync.WaitGroup
  	runs 			*int64
  	closeHandlers	*[]*func()

  	sync.RWMutex
  	emitter.Emitter
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
		}
	} else {
		runs := int64(0)
		closeHandlers := make([]*func(),0)
		
		ctx = &context{
			vars: 			map[string]interface{}{},
			runs: 			&runs,
			closeHandlers: 	&closeHandlers,
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

func Get (name string) interface{} {
   	ctx := getContext(goid.GoID())

	if ctx == nil { ctx = gctx }
   	
	ctx.RLock()
	v := ctx.vars[name]
	ctx.RUnlock()
	
	return v
}

func Set (name string, value interface{}) {
	ctx := getContext(goid.GoID())

	if ctx == nil { ctx = gctx }

	ctx.Lock()
	ctx.vars[name] = value
	ctx.Unlock()

	ctx.Emit("SET:"+name, value)
}

func OnSet (name string) <-chan emitter.Event {
	ctx := getContext(goid.GoID())

	if ctx == nil { ctx = gctx }

	return ctx.On("SET:"+name)
}

func OffSet (name string, ch <-chan emitter.Event) {
	ctx := getContext(goid.GoID())

	if ctx == nil { ctx = gctx }

	ctx.Off("SET:"+name, ch)
}

// Runs go routine with context. Uses global context if not exists before
// affects on context.Wait() and run CloseHandlers
func Run (routine func ()) {
	pctx := getContext(goid.GoID())
	
	if pctx == nil {
		gctx.subRuns.Add(1)
		atomic.AddInt64(gctx.runs,1)
	} else {
		pctx.subRuns.Add(1)
		atomic.AddInt64(pctx.runs,1)
	}
	
   	go func() {
		routineID := goid.GoID()
		ctx := createContext(routineID, pctx)
		defer deleteContext(routineID)

		if pctx == nil { pctx = gctx }

		defer pctx.subRuns.Done()
		
   		defer func() {
			defer func() {
				if atomic.AddInt64(pctx.runs, -1) != 0 { return }

				defer func() {
					err := recover()

					if err == nil { return }

					pctx.RLock()
					panicHandler := pctx.panicHandler
					pctx.RUnlock()

					if panicHandler != nil {
						defer func() {
							perr := recover()

							if perr != nil {
								fmt.Printf("CLOSE_HANDLER's PANIC: %s\n", err)
								fmt.Printf("PANIC_HANDLER's PANIC: %s\n%s\n", perr, debug.Stack())
							}
						}()

						panicHandler(err)
					} else {
						fmt.Printf("CLOSE_HANDLER's PANIC: %s\n%s\n", err, debug.Stack())
					}

				}()

				pctx.Lock(); defer pctx.Unlock()

				l := len(*pctx.closeHandlers)

				for l > 0 {
					l--
					ph := (*pctx.closeHandlers)[l]
					(*ph)()
				}
			}()

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

// Runs go routine with context. Uses global context if not exists before
// Does not affects on context.Wait() and run CloseHandlers
func RunHeir (routine func()) {
	pctx := getContext(goid.GoID())

	go func() {
		routineID := goid.GoID()
		ctx := createContext(routineID, pctx)
		defer deleteContext(routineID)

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

func AddCloseHandler (handler *func()) {
	ctx := getContext(goid.GoID())

	if ctx == nil { ctx = gctx }
	
	ctx.Lock(); defer ctx.Unlock()
	
   	for _,h := range *ctx.closeHandlers {
   		if h == handler { return }
	}

	*ctx.closeHandlers = append(*ctx.closeHandlers, handler)
}

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

