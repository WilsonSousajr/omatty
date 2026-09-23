// Package sample is a fixture. It is under testdata, so nothing compiles it;
// internal/crap parses it as source. Each function's cyclomatic complexity is
// stated above it, and those numbers are what the tests assert.
package sample

// CC 1: no branches at all.
func simple() bool { return true }

// CC 5: if, &&, for, if.
func branchy(n int, ok bool) int {
	if n > 0 && ok {
		return 1
	}
	for i := 0; i < n; i++ {
		if i == 3 {
			return i
		}
	}
	return 0
}

// CC 3: the range and the if are inside a closure, and they still count. A
// function's branches are its branches wherever they are written, and the
// coverage side attributes closure statements to the enclosing function too -
// so anything else would put the two halves of the metric out of step.
func withClosure(xs []int) func() int {
	return func() int {
		total := 0
		for _, x := range xs {
			if x > 0 {
				total += x
			}
		}
		return total
	}
}

// CC 1, and zero statements. A function with nothing in it cannot be tested,
// so it must not read as untested.
func empty() {}
