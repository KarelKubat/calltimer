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

// Per-timer activity report.
type activity struct {
	level    int
	duration time.Duration
	times    int
	name     string
}

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

var (
	timers = map[string]*Timer{} // Map of timers to avoid duplicate names
	roots  = []*Timer{}          // List of roots to ReportAll()
	mu     sync.Mutex            // Global lock for manipulation of global vars
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

	// Root timer
	if parent == nil {
		p := &Timer{Name: name, Children: []*Timer{}}
		roots = append(roots, p)
		timers[name] = p
		return p, nil
	}

	// Child timer
	t := &Timer{Name: name, Children: []*Timer{}, Parent: parent}
	timers[name] = t
	parent.Children = append(parent.Children, t)
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
	// and on the other root timer "r2".
	calltimer.ReportAll()
*/
func ReportAll(wr io.Writer) {
	rLen := &reportLen{}
	for _, r := range roots {
		r.calculateLengths(rLen, 0)
	}
	if !Active {
		return
	}
	mu.Lock()
	defer mu.Unlock()

	for _, r := range roots {
		r.Report(wr, rLen)
	}
}

/*
Report sends a report for the applicable timer to the passed-in io.Writer. For example:

		main        total 326.627542ms in  1 calls, avg 326.627542ms
	  	  outer     total  326.62675ms in  2 calls, avg 163.313375ms
	        middle  total 260.368249ms in  6 calls, avg  43.394708ms
	          inner total 260.350961ms in 24 calls, avg  10.847956ms

In this case, there is a one-to-one parent/child relationship: main has one child outer, which has one child middle, which has one child inner.
*/
func (t *Timer) Report(wr io.Writer, rLen *reportLen) {
	if !Active {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()

	if rLen == nil {
		rLen = &reportLen{}
		t.calculateLengths(rLen, 0)
	}

	t.report(0, rLen, wr)
}

func (t *Timer) calculateLengths(lengths *reportLen, level int) {
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
