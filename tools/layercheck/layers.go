package main

import (
	"fmt"
	"strings"
)

// Layer is one ring of ADR 0001's hexagon.
type Layer string

// The layers of ADR 0001's table, plus the two places outside it.
const (
	Domain    Layer = "domain"
	Service   Layer = "service"
	Infra     Layer = "infra"
	Pubsub    Layer = "pubsub"
	TUI       Layer = "tui"
	CLI       Layer = "cli"
	Cmd       Layer = "cmd"
	Tools     Layer = "tools"     // gate tooling and scripts: outside the hexagon
	Unlayered Layer = "unlayered" // placed nowhere: always a finding
)

// prefixes place a package by the path ADR 0001 gives its layer, so a package
// that has moved needs no entry in transitional.
var prefixes = []struct {
	prefix string
	layer  Layer
}{
	{"internal/domain/", Domain}, {"internal/service/", Service}, {"internal/infra/", Infra},
	{"internal/tui/", TUI}, {"cmd/", Cmd}, {"tools/", Tools},
}

// transitional places today's packages where ADR 0001's tree sends them, or
// sends their core when the package is mixed - so what the mixed half imports
// shows up as a finding. The migration empties it; an entry left after its
// package moved is dead and harmless.
var transitional = map[string]Layer{
	"internal/agent": Domain, "internal/coverage": Domain, "internal/gate": Service,
	"internal/review": Domain, "internal/tally": Domain,
	"internal/discover": Service, "internal/registry": Service, "internal/supervisor": Service,
	"internal/watcher": Service, "internal/watcher/e2e": Service,
	"internal/keys": TUI, "internal/termwrap": TUI, "internal/ui": TUI,
	"internal/pubsub": Pubsub, "internal/cli": CLI, "scripts": Tools,
}

// layerOf places importPath, a package of module.
func layerOf(importPath, module string) Layer {
	rel := strings.TrimPrefix(importPath, module+"/")
	if l, ok := transitional[rel]; ok {
		return l
	}
	for _, p := range prefixes {
		if strings.HasPrefix(rel, p.prefix) {
			return p.layer
		}
	}
	return Unlayered
}

// mayNotReach is ADR 0001's "must not import" column for the module's own
// layers. Domain may reach only domain; pubsub reaches nothing.
var mayNotReach = map[Layer][]Layer{
	Domain:  {Service, Infra, Pubsub, TUI, CLI, Cmd, Tools},
	Service: {Infra, TUI, CLI, Cmd, Tools},
	Infra:   {Service, TUI, CLI, Pubsub, Cmd, Tools},
	Pubsub:  {Domain, Service, Infra, TUI, CLI, Cmd, Tools, Pubsub},
	TUI:     {Infra, CLI, Cmd, Tools},
	CLI:     {Infra, TUI, Cmd, Tools},
}

// mayNotImport is the same column for packages outside the module, matched by
// prefix. Domain's is a different shape - everything that is not pure stdlib -
// so it is domainMayNotImport in check.go.
var mayNotImport = map[Layer][]string{
	Service: {"os/exec", "net/http", "charm.land/"},
	Infra:   {"charm.land/"},
	CLI:     {"charm.land/"},
}

// Finding is one import ADR 0001's table forbids, or a package it cannot place.
type Finding struct {
	From, To string
	Rule     string
}

func (f Finding) String() string {
	if f.To == "" {
		return fmt.Sprintf("%s: %s", f.From, f.Rule)
	}
	return fmt.Sprintf("%s -> %s: %s", f.From, f.To, f.Rule)
}

// ioStdlib is the standard library domain may not import: every package that
// reaches the process, the filesystem, the network or outside Go. The os/ and
// net/ trees are banned whole (ioTrees), less the two that only parse.
var ioStdlib = map[string]bool{"C": true, "os": true, "net": true, "syscall": true,
	"unsafe": true, "plugin": true, "io/ioutil": true}

var ioTrees = []string{"os/", "net/"}

var pureNet = map[string]bool{"net/url": true, "net/netip": true}
