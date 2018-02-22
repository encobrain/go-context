package context

import (
	"sync"
	"sync/atomic"
	
	"github.com/t-yuki/goid"
	"github.com/encobrain/go-emitter"
)

const EV_RUNS_DONE		 = "runs.done"
const EV_VARS_SET_PREFIX = "SET."

const (
	STATE_CLOSED  int32 = iota
	STATE_RUNNING
	STATE_ENDED
)

// event EV_RUNS_DONE
// event EV_CLOSED
type context struct {
	id 			  int64
	parent        *context
	childs 		  map[int64]*context
	childs_mu 	  sync.RWMutex
	routine 	  func()
	state 	  	  int32 
	panicHandler  *func (err interface{})

	separated	  bool
	vars          *sync.Map
  	vars_em       *emitter.Emitter
  	runs          int64
  	closeHandlers *[]*func()

	emitter.Emitter
  	sync.RWMutex
}

func (c *context) separate () {
	c.Lock()
	c.separated=true
	c.vars 	= &sync.Map{}
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
	value,_ = c.vars.Load(varName)
	return 
}

func (c *context) set (varName string, value interface{}) (setStatus chan emitter.EmitStatus) {
	c.vars.Store(varName, value)

	return c.vars_em.Emit(EV_VARS_SET_PREFIX+varName, value)
}

func (c *context) onSet (varName string) chan emitter.Event {
	return c.vars_em.On(EV_VARS_SET_PREFIX+varName)
}

func (c *context) offSet (varName string, ch chan emitter.Event) {
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

func (c *context) wait () (done chan emitter.Event) {
	defer func() { recover() }()

	done = c.Once(EV_RUNS_DONE)

	//fmt.Println("W", c.id, atomic.LoadInt64(&c.runs))

	if atomic.LoadInt64(&c.runs) == 0 { close(done) }

	return 
}

func (c *context) run (routine func()) {
	atomic.AddInt64(&c.runs, 1)

	//fmt.Println("C", c.id, "runs", runs)

	go func() {
		routineID := goid.GoID()
		ctx := contextCreate(routineID, c)
		ctx.routine = routine
		c.childsAdd(routineID, ctx)

		//f,l := getRoutineInfo(ctx)
		//fmt.Println("R", c.id, ctx.id, f,l)

		defer contextDelete(routineID)
		defer ctx.end()
		defer atomic.StoreInt32(&ctx.state, STATE_ENDED)

		defer func() {
			err := recover()
			if err != nil { ctx.handlePanic(err) }
		}()

		ctx.state = STATE_RUNNING
		routine()
	}()
}

func (c *context) end () {
	if atomic.LoadInt64(&c.runs) != 0 { return }

 	if c.close() { return }
 	if !atomic.CompareAndSwapInt32(&c.state, STATE_ENDED, STATE_CLOSED) { return }

 	if c == gctx { return }

 	par := c.parent
 	runs := atomic.AddInt64(&par.runs,-1)
	par.childsRemove(c.id)

	//fmt.Println("P", par.id, "runs", runs)

 	if runs < 0 { panic("runs<0") }
 	
	if runs == 0 {
		//fmt.Println("ST", par.id, atomic.LoadInt32(&par.state) )
		if atomic.LoadInt32(&par.state) == STATE_ENDED {
			par.end()
		} else {
			<-par.Emit(EV_RUNS_DONE)
		}
	}
}

func (c *context) childsAdd (routineID int64, ctx *context) {
	c.childs_mu.Lock()
	c.childs[routineID] = ctx
	c.childs_mu.Unlock()
}

func (c *context) childsRemove (routineID int64) {
	c.childs_mu.Lock()
	delete(c.childs, routineID)
	c.childs_mu.Unlock()
}

func (c *context) childsCount () (count int) {
	c.childs_mu.Lock()
	count = len(c.childs)
	c.childs_mu.Unlock()

	return 
}





