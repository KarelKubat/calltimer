// file: test/timer1/main.go
package main

import (
	"os"
	"time"

	"github.com/KarelKubat/calltimer"
)

var (
	mainTimer   = calltimer.MustNew("main", nil)
	outerTimer  = calltimer.MustNew("outer", mainTimer)
	middleTimer = calltimer.MustNew("middle", outerTimer)
	innerTimer  = calltimer.MustNew("inner", middleTimer)

	delay = time.Millisecond * 10
)

// Estimated runtime: 10ms
func inner() {
	defer innerTimer.LogSince(time.Now())
	time.Sleep(delay)
}

// Estimated runtime: 4x the runtime of inner, so 40ms
func middle() {
	defer middleTimer.LogSince(time.Now())
	for i := 0; i < 4; i++ {
		inner()
	}
}

// Estimated runtime: 3x (runtime of middle + 10ms), so 150ms
func outer() {
	defer outerTimer.LogSince(time.Now())
	for i := 0; i < 3; i++ {
		time.Sleep(delay)
		middle()
	}
}

// Estimated runtime: 2x the runtime of outer, so 300ms
func main() {
	start := time.Now()
	for i := 0; i < 2; i++ {
		outer()
	}
	mainTimer.LogSince(start)
	calltimer.ReportAll(os.Stdout)

	// Example output:
	// main        total 328.434958ms in  1 calls, avg 328.434958ms
	//   outer     total 328.433792ms in  2 calls, avg 164.216896ms
	// 	   middle  total 262.526374ms in  6 calls, avg  43.754395ms
	//   	 inner total 262.508876ms in 24 calls, avg  10.937869ms
}
