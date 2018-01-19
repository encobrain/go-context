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

  	sync.RWMutex
}

var contexts = map[int64]*context{}
var mu sync.RWMutex

func Get (name string) interface{} {
   	mu.RLock()
   	ctx := contexts[goid.GoID()]
	mu.RUnlock()

   	if ctx == nil { panic("Context not created") }

	ctx.RLock()
	v := ctx.vars[name]
	ctx.RUnlock()
	
	return v
}

func Set (name string, value interface{}) {
	mu.RLock()
	ctx := contexts[goid.GoID()]
	mu.RUnlock()

	if ctx == nil { panic("Context not created") }

	ctx.Lock()
	ctx.vars[name] = value
	ctx.Unlock()
}

// Runs go routine with context. Creates context if not exists before
func Run (routine func ()) {

   	mu.RLock()
   	ctx := contexts[goid.GoID()]
   	mu.RUnlock()

   	go func() {
		routineID := goid.GoID()

		if ctx == nil {
			ctx = &context{
				vars: map[string]interface{}{},
			}
		}
		
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

			mu.Lock()
			delete(contexts, routineID)
			mu.Unlock()
		}()

		mu.Lock()
		contexts[routineID] = ctx
		mu.Unlock()

   		routine()
	}()
}

// Sets panic handler & returns previous handler
func SetPanicHandler (handler func (err interface{})) func(err interface{}) {
	mu.RLock()
	ctx := contexts[goid.GoID()]
	mu.RUnlock()

	if ctx == nil { panic("Context not created") }

	ctx.Lock()
	prevHandler := ctx.panicHandler
	ctx.panicHandler = handler
	ctx.Unlock()

	return prevHandler
}

