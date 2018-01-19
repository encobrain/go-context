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

	Run(func() {
		Run(func() {time.Sleep(time.Millisecond*100)})
		Run(func() {time.Sleep(time.Millisecond*250)})
		Wait()
	})

	Run(func() {
		time.Sleep(time.Millisecond*200)
	})

	go func() {
		Wait()
		done<-true
	}()

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
}
