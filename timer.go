/*
Package calltimer implements very simple instrumentation to track and report the time spent in your code. The disadvantage is that to use it, you must add calls to your own code. The advantage is, that it's also available in situations where you don't have access to OS level performance tools.
*/
package calltimer

import (
	"errors"
	"fmt"
	"io"
	"sync"
	"time"
)

// String lengths over all roots
type reportLen struct {
	leaderLen int // String length of indentation + name
	totalLen  int // String length of total duration
	callsLen  int // String length of # of calls
	avgLen    int // String length of average duration
}

/*
Timer holds timing information and is constructed using New() or MustNew().
*/
type Timer struct {
	Name         string        // Timer name
	TotalElapsed time.Duration // Total duration
	CalledTimes  int           // Number of invocations
	Parent       *Timer        // Parent, nil when this is a root timer
	Children     []*Timer      // Dependent children
	mu           sync.Mutex    // Per-timer lock
}

/*
OutputFormat defines how Report or ReportAll present data.
*/
type Format int

const (
	Table     Format = iota // Present data as a table
	PlainText               // Present data in somewhat readable text format
	CSV                     // Present data as semicolon-separated values

	leaderLabel = "Timer name"
	totalLabel  = "Total time"
	callsLabel  = "Nr. of calls"
	avgLabel    = "Average time/call"
)

var (
	timers       = map[string]*Timer{}         // Map of timers to avoid duplicate names
	roots        = []*Timer{}                  // List of roots to ReportAll()
	mu           sync.Mutex                    // Global lock for manipulation of global vars
	OutputFormat Format                = Table // Current output format, defaults to Table

)

/*
Active defaults to true. When set to false, no timing is recorded and no reports are generated.
*/
var Active = true

/*
New creates a Timer. The passed-in name must be unique. When parent is nil, the timer is considered a root timer, meaning that ReportAll() picks it up.
*/
func New(name string, parent *Timer) (*Timer, error) {
	if !Active {
		return nil, nil
	}
	mu.Lock()
	defer mu.Unlock()

	// Name must exist and can't be redefined
	_, ok := timers[name]
	if name == "" {
		return nil, errors.New("can't create a timer without a name")
	}
	if ok {
		return nil, fmt.Errorf("timer %q is already defined", name)
	}

	t := &Timer{Name: name, Children: []*Timer{}, Parent: parent}
	timers[name] = t
	if parent == nil {
		roots = append(roots, t)
	} else {
		parent.Children = append(parent.Children, t)
	}
	return t, nil
}

/*
MustNew wraps New and panics upon error. The typical usage is:

	  var (
		callerTimer = calltimer.MustNew("caller", nil)          // a root timer
		calleeTimer = calltimer.MustNew("callee", callerTimer)  // a child of callerTimer
	  )
*/
func MustNew(name string, parent *Timer) *Timer {
	if !Active {
		return nil
	}

	t, err := New(name, parent)
	if err != nil {
		panic(fmt.Sprintf("TIMER PANIC: %v", err))
	}
	return t
}

/*
LogDuration adds the passed-in duration to the timer's TotalElapsed and increments the timer's CalledTimes counter. It is probably not that useful, given that LogSince() is more intuitive.
*/
func (t *Timer) LogDuration(d time.Duration) {
	if !Active {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()

	t.TotalElapsed += d
	t.CalledTimes++
}

/*
LogSince adds the duration since a given start to the timer's TotalElapsed and increments the timer's CalledTimes counter. For example, in the below snippet only doSomethingThatWeWantToTrack() is tracked.

To track the spent time in a separate function, defer is ideal:

	var myFunctimer = calltimer.MustNew("myFunc", nil)

	func myFunc() {
		defer myFunctimer.LogSince(time.Now())
		doSomeInterestingStuff()
	}

To limit what's tracked, the following can be used:

	func myFunc() {
		doSomethingNotInteresting()

		start := time.Now()
		doSomeInterestingStuff()
		myFuncTimer.LogDuration(time.Since(start))

		doMoreNotInterestingStuff()
	}
*/
func (t *Timer) LogSince(tstart time.Time) {
	if !Active {
		return
	}

	t.LogDuration(time.Since(tstart))
}

/*
ReportAll sends reports of all root timers (i.e., those which don't have a parent) to the passed-in io.Writer.

Example:

	r1 := calltimer.MustNew("r1", nil)
	c1 := calltimer.MustNew("c1", r1)
	r2 := calltimer.MustNew("r2", nil)

	// This reports on root timer "r1" together with its child timer "c1",
	// and on the other root timer "r2". Root timers without activity are
	// not reported.
	calltimer.ReportAll()
*/
func ReportAll(wr io.Writer) {
	if !Active {
		return
	}
	mu.Lock()
	defer mu.Unlock()

	rLen := &reportLen{}
	for _, r := range roots {
		r.calculateLengths(rLen, 0)
	}

	for _, r := range roots {
		r.reportWithFormatting(wr, rLen)
	}
}

/*
Report sends a report for the applicable timer to the passed-in io.Writer. For example:

		main        total 326.627542ms in  1 calls, avg 326.627542ms
	  	  outer     total  326.62675ms in  2 calls, avg 163.313375ms
	        middle  total 260.368249ms in  6 calls, avg  43.394708ms
	          inner total 260.350961ms in 24 calls, avg  10.847956ms

In this case, there is a one-to-one parent/child relationship: main has one child outer, which has one child middle, which has one child inner.

Timers that have no logged activity are not reported.
*/
func (t *Timer) Report(wr io.Writer) {
	if !Active || !t.hasActivity() {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()

	rLen := &reportLen{}
	t.calculateLengths(rLen, 0)

	t.report(0, rLen, wr)
}

func (t *Timer) reportWithFormatting(wr io.Writer, rLen *reportLen) {
	if !t.hasActivity() {
		return
	}
	t.report(0, rLen, wr)
}

func (t *Timer) calculateLengths(lengths *reportLen, level int) {
	if !t.hasActivity() {
		return
	}
	lengths.leaderLen = max(lengths.leaderLen, level*2+len(t.Name))
	lengths.totalLen = max(lengths.totalLen, len(fmt.Sprintf("%v", t.TotalElapsed)))
	lengths.callsLen = max(lengths.callsLen, len(fmt.Sprintf("%v", t.CalledTimes)))
	if t.CalledTimes > 0 {
		lengths.avgLen = max(lengths.avgLen,
			len(fmt.Sprintf("%v", t.TotalElapsed/time.Duration(t.CalledTimes))))
	}
	for _, c := range t.Children {
		c.calculateLengths(lengths, level+1)
	}
}

func (t *Timer) report(lev int, rLen *reportLen, wr io.Writer) {
	switch OutputFormat {
	case Table:
		t.reportTable(lev, rLen, wr)
	case PlainText:
		t.reportPlainText(lev, rLen, wr)
	case CSV:
		t.reportCSV(lev, wr)
	}
}

func (t *Timer) reportTable(lev int, rLen *reportLen, wr io.Writer) {
	ruler := func(rLen *reportLen) {
		fmt.Fprint(wr, "+")
		for i := 0; i < rLen.leaderLen+2; i++ {
			fmt.Fprint(wr, "-")
		}
		fmt.Fprint(wr, "+")
		for i := 0; i < rLen.totalLen+2; i++ {
			fmt.Fprint(wr, "-")
		}
		fmt.Fprint(wr, "+")
		for i := 0; i < rLen.callsLen+2; i++ {
			fmt.Fprint(wr, "-")
		}
		fmt.Fprint(wr, "+")
		for i := 0; i < rLen.avgLen+2; i++ {
			fmt.Fprint(wr, "-")
		}
		fmt.Fprintln(wr, "+")
	}
	if lev == 0 {
		rLen.leaderLen = max(rLen.leaderLen, len(leaderLabel))
		rLen.totalLen = max(rLen.totalLen, len(totalLabel))
		rLen.callsLen = max(rLen.callsLen, len(callsLabel))
		rLen.avgLen = max(rLen.avgLen, len(avgLabel))

		ruler(rLen)
		fmt.Fprintf(wr, "| %*s | %*s | %*s | %*s |\n",
			rLen.leaderLen, leaderLabel,
			rLen.totalLen, totalLabel,
			rLen.callsLen, callsLabel,
			rLen.avgLen, avgLabel)
		ruler(rLen)
	}
	fmt.Fprint(wr, "| ")
	for i := 0; i < lev; i++ {
		fmt.Fprint(wr, "  ")
	}
	fmt.Fprint(wr, t.Name)
	for printed := lev*2 + len(t.Name); printed <= rLen.leaderLen; printed++ {
		fmt.Fprint(wr, " ")
	}

	var avg string
	if t.CalledTimes > 0 {
		avg = fmt.Sprintf("%v", t.TotalElapsed/time.Duration(t.CalledTimes))
	}
	fmt.Fprintf(wr, "| %*v | %*v | %*v |\n",
		rLen.totalLen, t.TotalElapsed,
		rLen.callsLen, t.CalledTimes,
		rLen.avgLen, avg)

	for _, c := range t.Children {
		c.reportTable(lev+1, rLen, wr)
	}

	if lev == 0 {
		ruler(rLen)
	}
}

func (t *Timer) reportPlainText(lev int, rLen *reportLen, wr io.Writer) {
	for i := 0; i < lev; i++ {
		fmt.Fprint(wr, "  ")
	}
	fmt.Fprint(wr, t.Name)
	for printed := lev*2 + len(t.Name); printed <= rLen.leaderLen; printed++ {
		fmt.Fprint(wr, " ")
	}
	fmt.Fprintf(wr, "total %*v in %*v calls",
		rLen.totalLen, t.TotalElapsed, rLen.callsLen, t.CalledTimes)
	if t.CalledTimes > 0 {
		fmt.Fprintf(wr, ", avg %*v",
			rLen.avgLen, t.TotalElapsed/time.Duration(t.CalledTimes))
	}
	fmt.Fprintln(wr)

	for _, c := range t.Children {
		c.report(lev+1, rLen, wr)
	}
}

func (t *Timer) reportCSV(lev int, wr io.Writer) {
	if lev == 0 {
		fmt.Fprintln(wr, "Timer;Total;Calls;Average")
	}
	fmt.Fprintf(wr, "%v;%v;%v;", t.Name, t.TotalElapsed, t.CalledTimes)
	if t.CalledTimes > 0 {
		fmt.Fprintf(wr, "%v", t.TotalElapsed/time.Duration(t.CalledTimes))
	}
	fmt.Fprintln(wr)

	for _, c := range t.Children {
		c.reportCSV(lev+1, wr)
	}
}

func (t *Timer) hasActivity() bool {
	for _, c := range t.Children {
		if c.hasActivity() {
			return true
		}
	}
	return t.TotalElapsed > 0
}
