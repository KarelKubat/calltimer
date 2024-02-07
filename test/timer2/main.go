// file: test/timer2/main.go
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
	// Another root timer, just a dummy.
	dummyTop := calltimer.MustNew("dummy-top", nil)
	dummySub := calltimer.MustNew("dummy-sub", dummyTop)

	// Create some activity in the timers outerTimer, and hence in
	// middle1Timer, middle2Timer and in innerTimer.
	for i := 0; i < 2; i++ {
		outer()
	}

	calltimer.ReportAll(os.Stdout)
	// Example output:
	// 	outer       total 328.324125ms in  2 calls, avg 164.162062ms
	// 	  middle1   total 262.224458ms in  6 calls, avg  43.704076ms
	// 	    inner   total 524.960087ms in 48 calls, avg  10.936668ms
	// 	  middle2   total  66.091876ms in  6 calls, avg  11.015312ms
	// Notes:
	// - inner is only reported under middle1, that is the timer's parent/child
	//   relationship
	// - There is no output for dummy-top, as there is no activity.

	// Create some activity in dummySub.
	dummySub.LogDuration(time.Second)

	calltimer.ReportAll(os.Stdout)
	// Example output, which now reports on two root timers:
	// 	outer       total  329.21975ms in  2 calls, avg 164.609875ms
	// 	  middle1   total 263.650418ms in  6 calls, avg  43.941736ms
	// 	    inner   total 524.486669ms in 48 calls, avg  10.926805ms
	// 	  middle2   total  65.561709ms in  6 calls, avg  10.926951ms
	//  dummy-top   total           0s in  0 calls
	// 	  dummy-sub total           1s in  1 calls, avg           1s
}
