package context

import (
	"sync"
	"fmt"
	"runtime/debug"
	
	"github.com/t-yuki/goid"
)

type context struct {
	panicHandler 	func (err interface{})
  	vars 			map[string]interface{}
  	subRuns 		sync.WaitGroup

  	sync.RWMutex
}

var contexts = map[int64]*context{}
var mu 		 = sync.RWMutex{}

func createContext (routineID int64, prevContext *context) (ctx *context) {
   	if prevContext != nil {
   		ctx = &context{
   			panicHandler: 	prevContext.panicHandler,
   			vars:			prevContext.vars,
		}
	} else {
		ctx = &context{
			vars: map[string]interface{}{},
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

	if ctx == nil { ctx = createContext(routineID, nil) }

	return 
}

func deleteContext (routineID int64) {
  	mu.Lock()
  	delete(contexts, routineID)
  	mu.Unlock()
}

func Get (name string) interface{} {
   	ctx := getContext(goid.GoID())
   	
	ctx.RLock()
	v := ctx.vars[name]
	ctx.RUnlock()
	
	return v
}

func Set (name string, value interface{}) {
	ctx := getContext(goid.GoID())

	ctx.Lock()
	ctx.vars[name] = value
	ctx.Unlock()
}

// Runs go routine with context. Creates context if not exists before
func Run (routine func ()) {
	ctx := getContext(goid.GoID())
	
	ctx.subRuns.Add(1)

   	go func() {
		routineID := goid.GoID()

		defer ctx.subRuns.Done()

		ctx = createContext(routineID, ctx)
		
   		defer func() {
			err := recover()

   			if err != nil {
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
			}
			
			deleteContext(routineID)
		}()

		mu.Lock()
		contexts[routineID] = ctx
		mu.Unlock()

   		routine()
	}()
}

func Wait () {
	ctx := getContext(goid.GoID())

	ctx.subRuns.Wait()
}

// Sets panic handler & returns previous handler
func SetPanicHandler (handler func (err interface{})) func(err interface{}) {
	ctx := getContext(goid.GoID())

	ctx.Lock()
	prevHandler := ctx.panicHandler
	ctx.panicHandler = handler
	ctx.Unlock()

	return prevHandler
}

