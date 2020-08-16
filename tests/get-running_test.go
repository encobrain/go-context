package tests

import (
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/encobrain/go-context"
)

func routine2 () {
	dur := time.Duration(rand.Intn(20)) * time.Second

	fmt.Printf("Waiting %vs...\n", dur.Seconds())

	<- time.After(dur)

	fmt.Printf("Done\n")
}

func routine1 (i2 int) {
	for i := 0; i < 5; i++ {
		context.Run(routine2)
	}

	switch i2 % 3 {
		case 0:
			return

		case 1:
			<-context.Wait()

		case 2:
			<-context.Wait()
	}
}

func TestFormat (t *testing.T) {
	for i := 0; i < 12; i++ {
		i2 := i

		context.Run(func() {
			routine1(i2)
		})
	}

	done := context.Wait()

	var running string

	for {
		select {
			case <-done:
				return

			case <-time.After(time.Second):
				new := context.GetRunning()

				if new != running {
					fmt.Println(new)

					running = new
				}
		}
	}
}
