package context

import (
	"testing"

	"github.com/encobrain/go-tester"
)

var gtester *tester.Tester

func init () {
	gtester = tester.ParseDefaultFlags()
}

// @Tester:ignore
func TestAll (T *testing.T) {
	gtester.Test(T)
}