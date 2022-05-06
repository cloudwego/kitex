// +build gc,go1.13,!go1.19

package goid

type p struct {
	id int32 // Here is pid
}

type m struct {
	g0      uintptr // goroutine with scheduling stack
	morebuf gobuf   // gobuf arg to morestack
	divmod  uint32  // div/mod denominator for arm - known to liblink

	// Fields not known to debuggers.
	procid     uint64       // for debuggers, but offset not hard-coded
	gsignal    uintptr      // signal-handling g
	goSigStack gsignalStack // Go-allocated signal handling stack
	sigmask    sigset       // storage for saved signal mask
	tls        [6]uintptr   // thread-local storage (for x86 extern register)
	mstartfn   func()
	curg       uintptr // current running goroutine
	caughtsig  uintptr // goroutine running during fatal signal
	p          *p      // attached p for executing go code (nil if not executing go code)
}
