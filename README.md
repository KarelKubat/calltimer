# calltimer

Package `calltimer` implemenents instrumentation that can be called from Go code in order to estimate the spent time.

- The tracked time is only the duration (spent in a call, or spent in a block of code). The package doesn't track other performance-related timings, like CPU time, or I/O. Use the performance tools of your operating system for that purpose.
- The package generates a report which displays the total spent time, number of invocations, and average time per invocation.
- Reporting can group results in a tree-like structure: the display of a timer can be set under a parent.

## API

### Defining timers

Instrumenting Go code typically starts by defining timers:

```go
func main() {
    // a "root" timer, not attached to a parent
    topTimer, err := calltimer.New("mytimer", nil)
    if err != nil { ... }

    // a "child" timer, attached to parent topTimer
    subTimer, err := calltimer.New("subtimer", topTimer)
    if err != nil { ... }
}
```

Errors occur when, e.g., the name of a timer was already used. Such names must be unique.

Defining `subTimer` as a child of `topTimer` has only the effect that in reporting the `subTimer`s output is displayed under `topTimer` and indented. If you don't care about such grouping suggestions, then you can just as well define `subTimer` with a `nil` parent.

Instead of `calltimer.New()`, one may use `calltimer.MustNew()`, which panics upon an error.  This is typically handy for globals:

```go
var (
    topTimer = calltimer.MustNew("mytimer", nil)
    subtTmer = calltimer.MustNew("subtimer", topTimer)
)
```

### Logging the spent time

Catching what happened is added to functions. Typically:

```go
func top() {
    defer topTimer.LogSince(time.Now()
    sub()
}

func sub() {
    defer subTimer.LogSince(time.Now())
    // do some interesting stuff
}
```

In order to not capture the entire invocation of a function, but only a part of it, the following can be applied:

```go
func sub() {
    // do some uninteresting stuff

    start := time.Now()
    // do some interesting stuff
    subTimer.LogSince(start)

    // do more uninteresting stuff
}
```

### Reporting

To generate a report, `calltimer.ReportAll()` is called. This outputs reports for all "root" timers and for their child timers.

```go
func main() {
    // Call top(), which in turn calls sub()
    top()

    // Report
    calltimer.ReportAll(io.Stdout)
}
```

Instead of reporting on all root timers, one can generate reports for only specific timers (and their children), as in `subTimer.Report(os.Stdout)`.

### Disabling sampling and reporting

After testing and evaluating, the code that drives duration sampling and reporting can be left in place, though reduced to no-ops:

```go
calltimer.Active = false
// Now, all timer-related functions don't do anything
// and calltimer.ReportAll() is also a no-op.
```

## Examples

### Example 1: Linear calling

In this example, `main()` calls `outer()`, which calls `middle()`, which calls `inner()` -- all in a linear fashion.

```go
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
```

### Example 2: An `inner()` function is called from 2 places

```go
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
	// 	outer    total 329.379542ms in  2 calls, avg 164.689771ms
	//   middle1 total 264.478584ms in  6 calls, avg  44.079764ms
	//   middle2 total  64.893792ms in  6 calls, avg  10.815632ms
	//     inner total 522.944044ms in 48 calls, avg  10.894667ms
}
```

In this example the report tries to group the output according to how the timers are set up (`middle1` and `middle2` are the children of `outer`).

It's the developers' responsibility to define this structure. While running, the package `calltimer` has no idea of the actual call tree. Note however that this structure is only for display purposes (i.e., the indentation level of the report); it doesn't affect the collected data.