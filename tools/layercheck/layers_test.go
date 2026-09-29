package main

import (
	"strings"
	"testing"

	"github.com/WilsonSousajr/omatty/internal/golist"
)

const mod = "github.com/WilsonSousajr/omatty"

func pkg(path string, imports ...string) golist.Package {
	return golist.Package{ImportPath: mod + "/" + path, GoFiles: []string{"x.go"}, Imports: imports}
}

// ADR 0001's target paths place themselves; no table entry is needed once a
// package has moved (#620).
func TestLayerOf_placesTargetPathsByPrefix_issue620(t *testing.T) {
	cases := map[string]Layer{
		"internal/domain/session": Domain, "internal/service/sessions": Service,
		"internal/infra/vcs": Infra, "internal/pubsub": Pubsub, "internal/tui/app": TUI,
		"internal/cli": CLI, "cmd/omatty": Cmd, "tools/depcheck": Tools,
	}
	for path, want := range cases {
		if got := layerOf(mod+"/"+path, mod); got != want {
			t.Errorf("layerOf(%s) = %s, want %s", path, got, want)
		}
	}
}

// A package nobody placed must be reported, never waved through as clean.
func TestLayerOf_unknownPackageIsUnlayeredNotClean_issue620(t *testing.T) {
	if got := layerOf(mod+"/internal/somethingnew", mod); got != Unlayered {
		t.Errorf("layerOf(internal/somethingnew) = %s, want %s", got, Unlayered)
	}
	findings := check(mod, []golist.Package{pkg("internal/somethingnew")})
	if len(findings) != 1 || findings[0].Rule != "unlayered" {
		t.Errorf("an unplaced package gave findings %+v, want one unlayered finding", findings)
	}
}

func TestCheck_serviceImportingInfraIsAFinding_issue620(t *testing.T) {
	findings := check(mod, []golist.Package{
		pkg("internal/service/sessions", mod+"/internal/infra/vcs"),
		pkg("internal/infra/vcs"),
	})
	assertOneFinding(t, findings, "service -> infra")
}

func TestCheck_domainImportingOsIsAFinding_issue620(t *testing.T) {
	findings := check(mod, []golist.Package{pkg("internal/domain/session", "os", "strings")})
	assertOneFinding(t, findings, "domain may not import os")
}

func TestCheck_domainImportingAThirdPartyModuleIsAFinding_issue620(t *testing.T) {
	findings := check(mod, []golist.Package{pkg("internal/domain/review", "github.com/bluekeyes/go-gitdiff/gitdiff")})
	assertOneFinding(t, findings, "domain may not import github.com/bluekeyes/go-gitdiff/gitdiff")
}

func TestCheck_serviceImportingBubbleteaIsAFinding_issue620(t *testing.T) {
	findings := check(mod, []golist.Package{pkg("internal/service/review", "charm.land/bubbletea/v2")})
	assertOneFinding(t, findings, "service may not import charm.land/bubbletea/v2")
}

func TestCheck_tuiImportingServiceAndDomainIsClean_issue620(t *testing.T) {
	findings := check(mod, []golist.Package{
		pkg("internal/tui/app", mod+"/internal/service/sessions", mod+"/internal/domain/session", "charm.land/bubbletea/v2"),
		pkg("internal/service/sessions", mod+"/internal/domain/session"),
		pkg("internal/domain/session", "strings"),
	})
	if len(findings) != 0 {
		t.Errorf("a clean graph gave findings %+v", findings)
	}
}

// Every package in the repository today has a place, so the report's count is
// the migration's backlog and not partly "we never classified it".
func TestCheck_everyTodayPackageIsPlaced_issue620(t *testing.T) {
	pkgs, err := golist.List("../..", "./...")
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range pkgs {
		if layerOf(p.ImportPath, mod) == Unlayered {
			t.Errorf("%s has no layer: add it to the transitional table", p.ImportPath)
		}
	}
}

func assertOneFinding(t *testing.T, findings []Finding, want string) {
	t.Helper()
	if len(findings) != 1 || !strings.Contains(findings[0].String(), want) {
		t.Errorf("findings %v, want exactly one containing %q", findings, want)
	}
}
