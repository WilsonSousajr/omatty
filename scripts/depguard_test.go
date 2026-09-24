// These tests guard the depguard rules in .golangci.yml (#260). They exist
// because depguard can only fail in one direction: it catches an import that
// breaks a rule, never a rule that has quietly stopped describing the code.
// AGENTS.md claimed for three milestones that internal/ui was the only package
// importing bubbletea, and nothing noticed that internal/termwrap does too.
package scripts_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// execAllowed are the packages permitted to shell out. Kept here as well as in
// .golangci.yml so the two can be compared: a rule and the code it governs can
// drift apart silently, and TestDepguard_ExecAllowlistMatchesReality is the
// only thing that would say so. internal/termwrap earns its place by naming
// *exec.Cmd in a signature without ever constructing one - a distinction
// depguard cannot draw.
var execAllowed = []string{"detach", "forge", "gate", "golist", "notify", "supervisor", "termwrap", "vcs"}

// Regression, issue #260: invariant 4 fences bubbleterm inside internal/termwrap,
// and AGENTS.md:68 said internal/ui was the only package importing bubbletea.
// That was false - termwrap imports it in four production files, legitimately,
// because bubbleterm is itself a bubbletea component and termwrap.Terminal
// returns tea.Cmd. A rule written from the prose would have reddened CI on its
// first push.
func TestDepguard_AllowsTermwrapToImportBubbletea(t *testing.T) {
	exempt := exemptedPackages(t, "bubbletea")

	for _, pkg := range []string{"ui", "termwrap"} {
		if !exempt[pkg] {
			t.Errorf("internal/%s may not import bubbletea, but it does; "+
				"depguard would fail every build", pkg)
		}
	}
}

// bubbleterm is the narrower fence: invariant 4 puts the whole blast radius of a
// pre-1.0 dependency inside one package, so termwrap is the only exemption.
func TestDepguard_FencesBubbletermToTermwrapAlone(t *testing.T) {
	exempt := exemptedPackages(t, "bubbleterm")

	if !exempt["termwrap"] {
		t.Error("internal/termwrap cannot import bubbleterm; it owns the seam")
	}
	delete(exempt, "termwrap")
	for pkg := range exempt {
		t.Errorf("internal/%s is exempt from the bubbleterm rule; invariant 4 "+
			"keeps the blast radius inside termwrap alone", pkg)
	}
}

// The allowlist must name the packages that actually shell out - no more, no
// fewer. depguard fails a package that shells out without permission, but an
// allowlist that has grown past reality is invisible to it: the rule stays green
// while the invariant erodes. This test is the erosion detector, and it is meant
// to fire the moment someone adds a package to either side.
func TestDepguard_ExecAllowlistMatchesReality(t *testing.T) {
	want := map[string]bool{}
	for _, pkg := range execAllowed {
		want[pkg] = true
	}

	for pkg := range realExecImporters(t) {
		if !want[pkg] {
			t.Errorf("internal/%s imports os/exec but is not in the allowlist; "+
				"shelling out is a capability, so say so in .golangci.yml "+
				"and in AGENTS.md's Dependencies section", pkg)
		}
		delete(want, pkg)
	}
	for pkg := range want {
		t.Errorf("the allowlist permits internal/%s to shell out, but it does "+
			"not; drop it rather than leaving the fence wider than the code", pkg)
	}
}

// .golangci.yml must agree with execAllowed, or the list above guards nothing.
func TestDepguard_ExecRuleMatchesTheAllowlist(t *testing.T) {
	exempt := exemptedPackages(t, "subprocess")

	for _, pkg := range execAllowed {
		if !exempt[pkg] {
			delete(exempt, pkg)
			t.Errorf("internal/%s is in execAllowed but not exempt in "+
				".golangci.yml's subprocess rule", pkg)
			continue
		}
		delete(exempt, pkg)
	}
	for pkg := range exempt {
		t.Errorf("internal/%s is exempt in .golangci.yml but absent from "+
			"execAllowed", pkg)
	}
}

// Invariant 4's other half, which depguard structurally cannot see: git is
// reached by a string literal handed to exec, not by an import, so no import
// rule can fence it. internal/vcs owns the git CLI and nothing else may name it.
func TestNoGitOutsideVcs(t *testing.T) {
	root := repoRoot(t)

	for _, path := range productionFiles(t, root) {
		if strings.HasPrefix(path, filepath.Join("internal", "vcs")) {
			continue
		}
		b, err := os.ReadFile(filepath.Join(root, path))
		if err != nil {
			t.Fatal(err)
		}
		if line, found := codeLineNaming(string(b), `"git"`); found {
			t.Errorf(`%s names "git": %s`+"\n"+
				"invariant 4 routes the git CLI through internal/vcs alone", path, line)
		}
	}
}

// The gh CLI's twin of the rule above (#310): internal/forge owns gh, so a
// second package naming it would be a second, unreviewed reader of the forge.
func TestNoGhOutsideForge(t *testing.T) {
	root := repoRoot(t)

	for _, path := range productionFiles(t, root) {
		if strings.HasPrefix(path, filepath.Join("internal", "forge")) {
			continue
		}
		b, err := os.ReadFile(filepath.Join(root, path))
		if err != nil {
			t.Fatal(err)
		}
		if line, found := codeLineNaming(string(b), `"gh"`); found {
			t.Errorf(`%s names "gh": %s`+"\n"+
				"the gh CLI is reached through internal/forge alone", path, line)
		}
	}
}

// realExecImporters asks the toolchain which of our packages import os/exec,
// rather than grepping for the string - an import block is the compiler's
// answer, and a grep would count the word in a comment.
func realExecImporters(t *testing.T) map[string]bool {
	t.Helper()
	cmd := exec.Command("go", "list", "-f", "{{.ImportPath}} {{join .Imports \" \"}}", "./internal/...")
	cmd.Dir = repoRoot(t)

	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("go list: %v", err)
	}
	found := map[string]bool{}
	for _, line := range strings.Split(string(out), "\n") {
		path, imports, ok := strings.Cut(line, " ")
		if ok && slicesContains(strings.Fields(imports), "os/exec") {
			found[shortName(path)] = true
		}
	}
	return found
}

// shortName turns an import path into the package name a depguard glob uses:
// the first segment under internal/, so internal/watcher/e2e answers "watcher".
func shortName(importPath string) string {
	_, rest, found := strings.Cut(importPath, "/internal/")
	if !found {
		return ""
	}
	name, _, _ := strings.Cut(rest, "/")
	return name
}

// slicesContains is here rather than slices.Contains only to keep this file
// readable next to the Go version the module pins; behaviour is identical.
func slicesContains(haystack []string, needle string) bool {
	for _, s := range haystack {
		if s == needle {
			return true
		}
	}
	return false
}

// productionFiles lists the repo-relative .go files the gate governs, excluding
// tests - a test may legitimately drive git to build a fixture repository.
func productionFiles(t *testing.T, root string) []string {
	t.Helper()
	var found []string
	for _, dir := range []string{"internal", "cmd"} {
		err := filepath.WalkDir(filepath.Join(root, dir), func(path string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") ||
				strings.HasSuffix(path, "_test.go") {
				return err
			}
			rel, relErr := filepath.Rel(root, path)
			found = append(found, rel)
			return relErr
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	return found
}

// codeLineNaming returns the first non-comment line containing needle. Comments
// are skipped because invariant 4 is about what the code does, and the packages
// that must not run git are exactly the ones most likely to explain why.
func codeLineNaming(src, needle string) (string, bool) {
	for _, line := range strings.Split(src, "\n") {
		code := strings.TrimSpace(line)
		if code == "" || strings.HasPrefix(code, "//") {
			continue
		}
		if strings.Contains(code, needle) {
			return code, true
		}
	}
	return "", false
}

// exemptedPackages reads the internal packages one depguard rule lets through,
// from the `!**/internal/<name>/**` globs in its files list.
func exemptedPackages(t *testing.T, rule string) map[string]bool {
	t.Helper()
	exempt := map[string]bool{}
	for _, line := range ruleBlock(t, rule) {
		if name, ok := negatedInternal(line); ok {
			exempt[name] = true
		}
	}
	return exempt
}

// ruleBlock returns the lines of one depguard rule, from `<rule>:` up to the
// next key at the same indentation. Read as text rather than parsed as YAML so
// that guarding the config costs the module no new dependency.
func ruleBlock(t *testing.T, rule string) []string {
	t.Helper()
	lines := configLines(t)
	start, indent := -1, 0
	for i, line := range lines {
		if strings.TrimSpace(line) == rule+":" {
			start, indent = i+1, leadingSpaces(line)
			break
		}
	}
	if start < 0 {
		t.Fatalf(".golangci.yml declares no depguard rule %q", rule)
	}
	for i := start; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) != "" && leadingSpaces(lines[i]) <= indent {
			return lines[start:i]
		}
	}
	return lines[start:]
}

// configLines reads .golangci.yml.
func configLines(t *testing.T) []string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(repoRoot(t), ".golangci.yml"))
	if err != nil {
		t.Fatal(err)
	}
	return strings.Split(string(b), "\n")
}

// negatedInternal reads the package name out of a `!**/internal/<name>/**` glob.
func negatedInternal(line string) (string, bool) {
	const glob = `!**/internal/`
	_, rest, found := strings.Cut(line, glob)
	if !found {
		return "", false
	}
	name, _, ok := strings.Cut(rest, "/")
	return name, ok && name != ""
}

// leadingSpaces counts a line's indentation.
func leadingSpaces(line string) int {
	return len(line) - len(strings.TrimLeft(line, " "))
}
