package context

import (
	"testing"
	"time"
	"runtime/debug"
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
	done := make(chan int, 10)

	SetPanicHandler(func(err interface{}) {
		T.Errorf("PANIC: %s\nStack: %s", err, debug.Stack())
	})

	closeHandler := func () {
		done<- 5
		go close(done)
	}

	closeHandler2 := func() {
		done<- 4
	}

	closeHandler3 := func() {
		done<- 0
	}

	AddCloseHandler(&closeHandler)
	AddCloseHandler(&closeHandler2)

	Run(func() {
		done<- 1
		AddCloseHandler(&closeHandler)
		AddCloseHandler(&closeHandler2)
		AddCloseHandler(&closeHandler3)

		Run(func() {
			done<- 3
			RemoveCloseHandler(&closeHandler3)
		})

		done<- 2
	})

	si := 1
	for i := range done {
		if i != si { T.Fatalf("Incorrect run queue: %d != %d", i, si) }
		si++
	}
}

func TestCloseHandlerPanics (T *testing.T) {
	done := make(chan int, 10)

	SetPanicHandler(func(err interface{}) {
		done<- 3
		
		go close(done)

		panic("Panic in panic handler should prints to stdout with stack")
	})

	closeHandler := func() {
		done<- 2
		panic("Panic should calls panicHandler")
	}

	AddCloseHandler(&closeHandler)

	Run(func() {
		done<- 1
	})

	si := 1
	for i := range done {
		if i != si { T.Fatalf("Incorrect run queue: %d != %d", i, si) }
		si++
	}

	time.Sleep(time.Millisecond*50)
}

func TestSeparate (T *testing.T) {
 	done := make(chan int)

 	Run(func() {
		Set("testVar", "foo")

		close1 := func() {
			done<- 5
			close(done)
		}

		AddCloseHandler(&close1)

		Run(func() {
			done<- 2

			Separate()

			testVar := Get("testVar")

			if testVar != nil {
				T.Fatalf("Not separated")
			}

			close2 := func () {
				done<- 3
			}

			AddCloseHandler(&close2)
		})

		done<- 1

		Wait()

		done<- 4
	})

	si := 1
	for i := range done {
		if i != si { T.Fatalf("Incorrect run queue: %d != %d", i, si) }
		si++
	}
}
