package context

import (
	"testing"
	"time"
)

func TestGetNoRun (T *testing.T) {
   	Get("test")
}

func TestSetNoRun (T *testing.T) {
	Set("test", "test")
}

func testRunHelper (T *testing.T) {
	testv := Get("test")
	
	switch testv.(type) {
		case string:
			if testv != "testOK" {
				T.Errorf("testv incorrect: %s", testv)
			}
		default:
			T.Errorf("Incorrect type of testv: %T", testv)
	}
}

func TestRun (T *testing.T) {
	done := make(chan bool)

	Run(func() {
	 	Set("test", "testOK")

	 	testRunHelper(T)

	 	done<-true
	})

	<-done
}

func TestPanicHandlerSetNoRun (T *testing.T) {
	SetPanicHandler(func(err interface{}) {})
}

func TestPanicHandlerSet (T *testing.T) {
	done := make(chan bool)
	
	Run(func() {
		SetPanicHandler(func(err interface{}) {
			if err != "panic in routine" {
				T.Errorf("Incorrect panic value: %#v", err)
			}

			done <-true
		})


		Run(func() {
			panic("panic in routine")
		})
	})

	<-done
}

func TestPanicHandlerNotSet (t *testing.T) {
	done := make(chan bool)

	Run(func() {
		go func (){
			done<-true
		}()
		
		panic("Panic should prints to stdout with stack")
	})

	<-done
}

func TestPanicHandlerPanics (t *testing.T) {
	done := make(chan bool)

	Run(func() {
		SetPanicHandler(func(err interface{}) {
			go func() { done<-true }()

			panic("Panic in panicHadler should prints to stdout with source panic && stack")
		})
		
		Run(func() {
			panic("panic in routine")
		})
	})

	<-done
}

func TestWait (T *testing.T) {

	done := make(chan bool)
	done2 := make(chan bool)

	Run(func() {
		Run(func() {time.Sleep(time.Millisecond*100)})
		Run(func() {time.Sleep(time.Millisecond*250)})
		Wait()
	})

	Run(func() {
		time.Sleep(time.Millisecond*200)
	})

	go func() {
		select {
			case <-done:
				T.Errorf("Done early")
			case <-time.After(time.Millisecond*250):
				select {
					case <-done:
					case <-time.After(time.Millisecond):
						T.Errorf("Not done")
				}

		}

		close(done2)
	}()

	Wait()
	close(done)
	<-done2
}

func TestAddRemoveCloseHandler (T *testing.T) {
 
	runsCount := 0

	closeHandler := func () {
		if runsCount != 1 {
			T.Errorf("Wrong call queue: %d", runsCount)
		}
		
		runsCount++
	}

	closeHandler2 := func() {
		if runsCount != 0 {
			T.Errorf("Wrong call queue: %d", runsCount)
		}
		
		runsCount++
	}

	closeHandler3 := func() {
		runsCount++
	}

	AddCloseHandler(&closeHandler)
	AddCloseHandler(&closeHandler2)

	Run(func() {
		AddCloseHandler(&closeHandler)
		AddCloseHandler(&closeHandler2)
		AddCloseHandler(&closeHandler3)

		Run(func() {
			RemoveCloseHandler(&closeHandler3)
		})
	})
	
	Wait()
	
	if runsCount != 2 {
		T.Errorf("Runs count incorrect: %d", runsCount)
	}
}

func TestCloseHandlerPanics (T *testing.T) {
	panicHandlerCalled := false

	SetPanicHandler(func(err interface{}) {
		panicHandlerCalled = true

		panic("Panic in panic handler should prints to stdout with stack")
	})

	closeHandler := func() {
		panic("Panic should calls panicHandler")
	}

	AddCloseHandler(&closeHandler)

	Run(func() {})

	Wait()

	if panicHandlerCalled != true {
		T.Errorf("Panic handler not called")
	}
}
