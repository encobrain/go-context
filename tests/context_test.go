package tests

import (
	"testing"
	"time"
	"math/rand"
	"runtime/debug"
	"fmt"

	. "github.com/encobrain/go-context"
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


func TestPanicHandlerSet (T *testing.T) {
	done := make(chan int)

	SetGPanicHandler(func(err interface{}) {
		done<- 6

		if err != "panic in LPanicHandler" {
			T.Errorf("Incorrect panic value: %#v", err)
		}

		go close(done)

		panic("panic in root should prints to stdout with stack")
	})

	Run(func() {
		done<- 1

		SetLPanicHandler(func(err interface{}) {
			done<- 5

			if err != "panic in LGPanicHandler" {
				T.Errorf("Incorrect panic value: %#v", err)
			}

			panic("panic in LPanicHandler")
		})

		Run(func() {
			done<- 2

			SetLPanicHandler(func(err interface{}) {  // should be replaced by next GHandler
				done<- 0
			})

			Run(func() {
				done<-3

				SetGPanicHandler(func(err interface{}) {
					done<- 4

					if err != "panic in routine" {
						T.Errorf("Incorrect panic value: %#v", err)
					}

					panic("panic in LGPanicHandler")
				})

				panic("panic in routine")
			})
		})
	})

	si := 1
	for i := range done {
		fmt.Println("done", i)
		
		if i != si { T.Errorf("Incorrect run queue: %d != %d", i, si) }
		si++
	}

	time.Sleep(time.Millisecond*10)
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
		SetGPanicHandler(func(err interface{}) {
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

	step := make(chan int)
	
	Run(func() {
		Run(func() {time.Sleep(time.Millisecond*10); step<- 2 })
		Run(func() {
			Run(func() {time.Sleep(time.Millisecond*25); step<- 5; close(done2) })
			Run(func() {time.Sleep(time.Millisecond*5); step<- 1 })
			Run(func() {time.Sleep(time.Millisecond*15); step<- 3 })
			fmt.Println("exit2")
		})
		fmt.Println("exit1")
	})

	Run(func() { time.Sleep(time.Millisecond*20); step<- 4 })

	go func() {
		select {
			case <-done:
				T.Fatalf("Done early")
			case <-time.After(time.Millisecond*25):
				select {
					case <-done:
						fmt.Println("Done")
					case <-time.After(time.Millisecond*5):
						T.Fatalf("Not done")
				}

		}
	}()

	go func() {
		si := 1
		for i := range step {
			fmt.Println("step", i)

			if i != si { T.Errorf("Incorrect run queue: %d != %d", i, si) }
			si++
		}
	}()

	<-Wait()
	close(done)
	<-done2
}

func TestRandomWait (T *testing.T) {

	levels := 5

	var run func(int)

	run = func (level int) {
		if level>0 {
			subruns := rand.Int31n(10)+3

			for subruns>0 {
				Run(func() {
					run(level-1)
				})

				subruns--
			}

			<-Wait()
		}

		wait := time.Duration(rand.Int63n(100)*int64(time.Millisecond))+100
		time.Sleep(wait)
	}


	run(levels)

	<-Wait()
}

func TestAddRemoveCloseHandler (T *testing.T) {
	done := make(chan int, 10)

	SetGPanicHandler(func(err interface{}) {
		T.Errorf("PANIC: %s\nStack: %s", err, debug.Stack())
	})

	closeHandler := func () {
		done<- 4
		go close(done)
	}

	closeHandler2 := func() {
		done<- 3
	}

	closeHandler3 := func() {
		done<- 5
	}

	Run(func() {
		done<- 1
		AddCloseHandler(&closeHandler)
		AddCloseHandler(&closeHandler2)
		AddCloseHandler(&closeHandler3)

		Run(func() {
			done<- 2
			RemoveCloseHandler(&closeHandler3)
		})
	})

	si := 1
	for i := range done {
		fmt.Println("done", i)
		if i != si { T.Fatalf("Incorrect run queue: %d != %d", i, si) }
		si++
	}
}

func TestCloseHandlerPanics (T *testing.T) {
	done := make(chan int, 10)

	SetGPanicHandler(func(err interface{}) {
		done<- 3

		if err != "Panic should calls panicHandler" {
			T.Errorf("Incorrect panic value: %s", err)
		}

		go close(done)

		panic("Panic in panic handler should prints to stdout with stack")
	})

	closeHandler := func() {
		done<- 2
		panic("Panic should calls panicHandler")
	}

	Run(func() {
		AddCloseHandler(&closeHandler)
		
		done<- 1
	})

	si := 1
	for i := range done {
		fmt.Println("done", i)
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
			done<- 4
			close(done)
		}

		AddCloseHandler(&close1)

		Run(func() {
			done<- 1

			Separate()

			testVar := Get("testVar")

			if testVar != nil {
				T.Fatalf("Not separated")
			}

			close2 := func () {
				done<- 2
			}

			AddCloseHandler(&close2)
		})

		<-Wait()

		done<- 3
	})

	si := 1
	for i := range done {
		fmt.Println("done", i)
		if i != si { T.Fatalf("Incorrect run queue: %d != %d", i, si) }
		si++
	}
}

func TestSeparatePanic (T *testing.T) {
	done := make(chan int)

	Run(func() {
		done<- 1

		SetGPanicHandler(func(err interface{}) {
			if err != "panic in context" {
				T.Errorf("Incorrect err: %s", err)
			}

			done<- 9
			close(done)
		})

		Run(func() {
			done<- 3
			Separate()

			Run(func() {
				done<- 5
				SetGPanicHandler(func(err interface{}) {
					if err != "panic in separate context" {
						T.Errorf("Incorrect err: %s", err)
					}

					done<- 7
				})
				
				Run(func() {
					done<- 6
					panic("panic in separate context")
				})
			})
			
			done<- 4
		})

		done<- 2

		<-Wait()

		done<- 8

		Run(func() {
			Separate()
			panic("panic in context")
		})
	})

	si := 1
	for i := range done {
		if i != si { T.Errorf("Incorrect run queue: %d != %d", i, si) }
		fmt.Println("done", i)
		si++
	}
}
