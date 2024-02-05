// file: test/timer1/main.go
package main

import (
	"os"
	"sync"
	"time"

	"github.com/KarelKubat/calltimer"
)

var (
	outerTimer   = calltimer.MustNew("outer", nil)
	middle1Timer = calltimer.MustNew("middle1", outerTimer)
	middle2Timer = calltimer.MustNew("middle2", outerTimer)
	innerTimer   = calltimer.MustNew("inner", middle1Timer)

	delay = time.Millisecond * 10
)

// Estimated runtime: 10ms
func inner() {
	defer innerTimer.LogSince(time.Now())
	time.Sleep(delay)
}

// Estimated runtime: 4x the runtime of inner, so 40ms
func middle1() {
	defer middle1Timer.LogSince(time.Now())
	for i := 0; i < 4; i++ {
		inner()
	}
}

// Estimated runtime: the runtime of inner, so 10ms
// inner() gets invoked 4x, but in parallel - counts as one.
func middle2() {
	defer middle2Timer.LogSince(time.Now())
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			inner()
			wg.Done()
		}()
	}
	wg.Wait()
}

// Estimated runtime: 3x the runtime of middle1, plus 3x the runtime of middle2
// so 3x40ms + 3x10ms = 150ms
func outer() {
	defer outerTimer.LogSince(time.Now())
	for i := 0; i < 3; i++ {
		middle1()
	}
	for i := 0; i < 3; i++ {
		middle2()
	}
}

func main() {
	for i := 0; i < 2; i++ {
		outer()
	}
	calltimer.ReportAll(os.Stdout)

	// Example output:
	// outer     total 327.427417ms in  2 calls, avg 163.713708ms
	//   middle1 total 261.320251ms in  6 calls, avg  43.553375ms
	//     inner total 524.047914ms in 48 calls, avg  10.917664ms
	//   middle2 total  66.100376ms in  6 calls, avg  11.016729ms}
	// Note that inner is only reported under middle1; it's set up as a child
	// of middle1. Even though it's also called by middle2.
}
