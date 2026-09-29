package main

import (
	"strings"
	"testing"
)

// #134 promoted develop to main and tagged it. A tag names a version the
// binary itself could not report: `omatty --version` was an unknown command,
// so a bug report could not say which build it came from.

func TestVersionLine_NamesTheLinkedVersion_issue134(t *testing.T) {
	if got, want := versionLine("v0.1.0"), "omatty v0.1.0"; got != want {
		t.Errorf("versionLine(%q) = %q, want %q", "v0.1.0", got, want)
	}
}

// A plain `go build ./cmd/omatty` passes no -X flag, so version is "". Left
// unhandled that prints a bare "omatty ", which reads as a release whose tag
// went missing rather than as a build from source.
func TestVersionLine_SaysDevWhenTheLinkerSetNothing_issue134(t *testing.T) {
	got := versionLine("")
	if !strings.Contains(got, "dev") {
		t.Errorf("versionLine(%q) = %q, want it to say dev", "", got)
	}
}

// The flag is read in main before the config file is parsed, so a config
// omatty refuses to start on can still be reported against a version. Both
// spellings answer: `omatty version` matches the other subcommands, and
// `omatty --version` is what a stranger tries first.
func TestIsVersionFlag_AcceptsEverySpelling_issue134(t *testing.T) {
	for _, arg := range []string{"version", "--version", "-version"} {
		if !isVersionFlag(arg) {
			t.Errorf("isVersionFlag(%q) = false, want true", arg)
		}
	}
}

func TestIsVersionFlag_RejectsASubcommand_issue134(t *testing.T) {
	for _, arg := range []string{"add", "new", "hook", "", "-v"} {
		if isVersionFlag(arg) {
			t.Errorf("isVersionFlag(%q) = true, want false", arg)
		}
	}
}

// README's install line is `go install`, which passes no -X flag. Without a
// fallback every user who installs the documented way reports "dev", and a
// bug report cannot name the release it came from (#134).

func TestResolveVersion_PrefersTheLinkedVersion_issue134(t *testing.T) {
	if got, want := resolveVersion("v0.2.0", "v0.1.0"), "v0.2.0"; got != want {
		t.Errorf("resolveVersion = %q, want %q: -X beats the module version", got, want)
	}
}

func TestResolveVersion_FallsBackToTheModuleVersion_issue134(t *testing.T) {
	if got, want := resolveVersion("", "v0.1.0"), "v0.1.0"; got != want {
		t.Errorf("resolveVersion = %q, want %q", got, want)
	}
}

// A build from a working tree rather than a tag reports "(devel)", which is
// not a release and must not be printed as one.
func TestResolveVersion_TreatsDevelAsNoVersion_issue134(t *testing.T) {
	if got := resolveVersion("", "(devel)"); got != "" {
		t.Errorf("resolveVersion(\"\", \"(devel)\") = %q, want \"\"", got)
	}
}

func TestResolveVersion_IsEmptyWhenNeitherIsSet_issue134(t *testing.T) {
	if got := resolveVersion("", ""); got != "" {
		t.Errorf("resolveVersion(\"\", \"\") = %q, want \"\"", got)
	}
}
