// The catalog: every agent omatty can run in this process, built once in
// cmd/omatty and injected wherever a session's agent is resolved (#521). A
// value, not a package-level registry, so a test builds the catalog it needs
// and nothing is shared between them.

package agent

import (
	"errors"
	"fmt"
	"strings"
)

// Catalog is the set of profiles one omatty knows, with the binary the
// config chose for each. The first profile is the one an empty name means.
//
//	c, err := agent.NewCatalog(claude, codex)
//	p, err := c.Lookup(sess.Agent)
//	argv := p.Command(c.Bin(p), id, dir, false, hooksFile)
type Catalog struct {
	profiles []Profile
	bins     map[string]string
	hooks    map[string]string
}

// NewCatalog checks every profile can serve what it declares and returns
// them as a catalog. The first is the default: an empty agent name in
// state.json is every row written before #46, all of them claude's, so cmd
// passes claude first (invariant 9).
//
//	c, err := agent.NewCatalog(claudeProfile())
func NewCatalog(profiles ...Profile) (Catalog, error) {
	if len(profiles) == 0 {
		return Catalog{}, errors.New("agent catalog: no profiles, want at least claude's")
	}
	seen := map[string]bool{}
	for _, p := range profiles {
		if seen[p.Name] {
			return Catalog{}, fmt.Errorf("agent catalog: two profiles named %q, want one per name", p.Name)
		}
		seen[p.Name] = true
		if err := p.servesItsCaps(); err != nil {
			return Catalog{}, err
		}
	}
	return Catalog{profiles: profiles}, nil
}

// servesItsCaps reports a capability declared without the function that
// serves it, which would otherwise fail at session start rather than at
// build (#520).
func (p Profile) servesItsCaps() error {
	switch {
	case p.Command == nil:
		return fmt.Errorf("agent %q: no command template, want one to start it", p.Name)
	case p.Caps.Status >= StatusTranscript && (p.TranscriptPath == nil || p.Status == nil):
		return fmt.Errorf("agent %q: declares a transcript, want a transcript path and a status adapter", p.Name)
	case p.Caps.Status == StatusHooks && (p.HookEvents == nil || p.RenderSettings == nil || p.ParseHook == nil):
		return fmt.Errorf("agent %q: declares hooks, want hook events, a settings renderer and a payload parser", p.Name)
	}
	return nil
}

// Lookup returns the profile a session names. An unknown name is an error
// naming it and every known agent, never a silent fallback to claude: that
// would run the wrong binary on someone else's conversation (#521).
//
//	p, err := c.Lookup("") // the first profile
func (c Catalog) Lookup(name string) (Profile, error) {
	if name == "" {
		return c.profiles[0], nil
	}
	for _, p := range c.profiles {
		if p.Name == name {
			return p, nil
		}
	}
	return Profile{}, fmt.Errorf("agent %q is not one omatty knows, want one of %s", name, strings.Join(c.Names(), ", "))
}

// Names is every agent in the catalog, the default first.
//
//	c.Names() // ["claude"]
func (c Catalog) Names() []string {
	names := make([]string, len(c.profiles))
	for i, p := range c.profiles {
		names[i] = p.Name
	}
	return names
}

// WithBins returns the catalog with the binaries the config names, keyed by
// agent name. A name the config leaves out runs the profile's DefaultBin.
//
//	c = c.WithBins(map[string]string{"claude": cfg.ClaudeBin})
func (c Catalog) WithBins(bins map[string]string) Catalog {
	c.bins = bins
	return c
}

// Bin is the binary to run for p: the configured one, else its default.
//
//	argv := p.Command(c.Bin(p), id, dir, resume, hooksFile)
func (c Catalog) Bin(p Profile) string {
	if bin := c.bins[p.Name]; bin != "" {
		return bin
	}
	return p.DefaultBin
}

// WithHooksFiles returns the catalog with the settings file omatty wrote for
// each agent that takes hooks, keyed by agent name (#522).
//
//	c = c.WithHooksFiles(files) // from hooks.InstallAll
func (c Catalog) WithHooksFiles(files map[string]string) Catalog {
	c.hooks = files
	return c
}

// HooksFile is the settings file to hand p, or "" for an agent without one.
//
//	argv := p.Command(c.Bin(p), id, dir, resume, c.HooksFile(p))
func (c Catalog) HooksFile(p Profile) string { return c.hooks[p.Name] }
