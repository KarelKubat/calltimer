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
	// Example output, using the default format calltimer.Table:
	// +------------+--------------+--------------+-------------------+
	// | Timer name |   Total time | Nr. of calls | Average time/call |
	// +------------+--------------+--------------+-------------------+
	// | outer      | 332.675042ms |            2 |      166.337521ms |
	// |   middle1  | 265.168457ms |            6 |       44.194742ms |
	// |     inner  | 533.427539ms |           48 |       11.113073ms |
	// |   middle2  |  67.494167ms |            6 |       11.249027ms |
	// +------------+--------------+--------------+-------------------+
	// Notes:
	// - inner is only reported under middle1, that is the timer's parent/child
	//   relationship
	// - There is no output for dummy-top, as there is no activity.

	// Create some activity in dummySub.
	dummySub.LogDuration(time.Second)

	calltimer.ReportAll(os.Stdout)
	// Example output, which now reports on two root timers:
	// +-------------+--------------+--------------+-------------------+
	// |  Timer name |   Total time | Nr. of calls | Average time/call |
	// +-------------+--------------+--------------+-------------------+
	// | outer       | 332.675042ms |            2 |      166.337521ms |
	// |   middle1   | 265.168457ms |            6 |       44.194742ms |
	// |     inner   | 533.427539ms |           48 |       11.113073ms |
	// |   middle2   |  67.494167ms |            6 |       11.249027ms |
	// +-------------+--------------+--------------+-------------------+
	// +-------------+--------------+--------------+-------------------+
	// |  Timer name |   Total time | Nr. of calls | Average time/call |
	// +-------------+--------------+--------------+-------------------+
	// | dummy-top   |           0s |            0 |                   |
	// |   dummy-sub |           1s |            1 |                1s |
	// +-------------+--------------+--------------+-------------------+
}
