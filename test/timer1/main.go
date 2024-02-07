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

	// Default:
	// calltimer.OutputFormat = calltimer.Table
	calltimer.ReportAll(os.Stdout)
	// Example output:
	// +-------------+--------------+--------------+-------------------+
	// |  Timer name |   Total time | Nr. of calls | Average time/call |
	// +-------------+--------------+--------------+-------------------+
	// | main        | 333.963583ms |            1 |      333.963583ms |
	// |   outer     | 333.962417ms |            2 |      166.981208ms |
	// |     middle  | 267.721834ms |            6 |       44.620305ms |
	// |       inner |  267.69479ms |           24 |       11.153949ms |
	// +-------------+--------------+--------------+-------------------+

	calltimer.OutputFormat = calltimer.PlainText
	calltimer.ReportAll(os.Stdout)
	// Example output:
	// main        total 333.963583ms in  1 calls, avg 333.963583ms
	//   outer     total 333.962417ms in  2 calls, avg 166.981208ms
	//     middle  total 267.721834ms in  6 calls, avg  44.620305ms
	// 	     inner total  267.69479ms in 24 calls, avg  11.153949ms

	calltimer.OutputFormat = calltimer.CSV
	calltimer.ReportAll(os.Stdout)
	// Example output:
	// Timer;Total;Calls;Average
	// main;333.963583ms;1;333.963583ms
	// outer;333.962417ms;2;166.981208ms
	// middle;267.721834ms;6;44.620305ms
	// inner;267.69479ms;24;11.153949ms
}
