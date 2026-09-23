package gate

import (
	"regexp"
	"strconv"
	"strings"
)

// KindCoverage marks a step whose percentage is worth reading out of its
// output for display. It is the only Kind that means anything today.
//
//	gate.Step{Name: "cov", Kind: gate.KindCoverage, Run: "./scripts/check-coverage.sh 90"}
//
// Invariant 12 bounds what this buys: the number is shown, never consulted. A
// coverage step passes or fails on its exit code like every other step, and a
// percentage that will not parse is reported as zero rather than failing a run
// the tool itself said had passed.
const KindCoverage = "coverage"

// coverageWords mark the line a tool puts its summary on. Matched
// case-insensitively against each line.
var coverageWords = []string{"coverage", "cover", "total", "all files"}

// Compiled once; both are read-only.
var (
	percentPattern = regexp.MustCompile(`([0-9]+(?:\.[0-9]+)?)\s*%`)
	numberPattern  = regexp.MustCompile(`[0-9]+(?:\.[0-9]+)?`)
)

// percentIn reads a coverage percentage out of a step's output, or 0.
//
// It picks a line before it picks a number, because the interesting number is
// rarely the first or the last one present. "coverage 92.4% meets the 90%
// gate" wants the first percentage on its line, while a run that prints a
// threshold and then a total wants the later line - one rule cannot be "first"
// or "last" over the whole output and satisfy both.
func percentIn(out string) float64 {
	line := summaryLine(out)
	if line == "" {
		return 0
	}
	if value, ok := firstUnder100(percentPattern, line); ok {
		return value
	}
	// Some reporters tabulate without a % sign at all (istanbul's "All files"
	// row), so fall back to the first plausible number on the chosen line.
	value, _ := firstUnder100(numberPattern, line)
	return value
}

// summaryLine is the last line naming a coverage word, or failing that the
// last line carrying a percentage at all - so an unrecognised tool still
// reports something rather than nothing.
func summaryLine(out string) string {
	named, anyPercent := "", ""
	for _, line := range strings.Split(out, "\n") {
		if containsAny(strings.ToLower(line), coverageWords) {
			named = line
		}
		if strings.Contains(line, "%") {
			anyPercent = line
		}
	}
	if named != "" {
		return named
	}
	return anyPercent
}

// firstUnder100 returns the first match that could be a percentage. The bound
// is what keeps a row of counts - "TOTAL 1204 103 91%" - from reading 1204.
func firstUnder100(pattern *regexp.Regexp, line string) (float64, bool) {
	for _, match := range pattern.FindAllString(line, -1) {
		value, err := strconv.ParseFloat(strings.TrimRight(match, " \t%"), 64)
		if err != nil || value > 100 {
			continue
		}
		return value, true
	}
	return 0, false
}

// containsAny reports whether s holds any of words.
func containsAny(s string, words []string) bool {
	for _, word := range words {
		if strings.Contains(s, word) {
			return true
		}
	}
	return false
}
