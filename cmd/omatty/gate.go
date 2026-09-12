// `omatty gate`: what a project is verified by, and how it comes to be set.
// Thin over internal/gate and internal/registry (invariant 10).

package main

import (
	"fmt"
	"io"
	"strings"

	"github.com/WilsonSousajr/omatty/internal/gate"
	"github.com/WilsonSousajr/omatty/internal/registry"
)

// gateCommand shows, proposes, sets or clears a project's gate.
//
//	omatty gate <project>            show it, or propose one and ask
//	omatty gate <project> --detect   print the proposal, write nothing
//	omatty gate <project> --set      write the proposal without asking
//	omatty gate <project> --clear    forget it
//
// The plain form is the confirm-once flow, and it is the same shape `discover`
// and `adopt` already use: print what was found, read one answer, write only
// on yes. Detection proposes and confirming is a separate act, which is what
// stops a cloned repository getting a command run because omatty looked at it
// (#226).
func gateCommand(store *registry.Store, args []string, in io.Reader) error {
	project, err := gateProject(store, args)
	if err != nil {
		return err
	}
	if hasFlag(args, "--clear") {
		if err := registry.ClearGate(store, project.Name); err != nil {
			return err
		}
		// Silence would be indistinguishable from a command that did nothing,
		// and every other path here reports what it did.
		report("gate cleared for " + project.Name)
		return nil
	}
	if len(project.Gate) > 0 && !hasFlag(args, "--detect") && !hasFlag(args, "--set") {
		reportGate("the gate for "+project.Name+" is:", project.Gate)
		return nil
	}
	return proposeGate(store, project, args, in)
}

// proposeGate detects, prints, and writes if it is allowed to.
func proposeGate(store *registry.Store, project registry.Project, args []string, in io.Reader) error {
	steps := gate.Detect(project.Root)
	if len(steps) == 0 {
		report("nothing recognised in " + project.Root + "; set a gate by hand in ~/.omatty/state.json")
		return nil
	}
	reportGate("omatty proposes this gate for "+project.Name+":", steps)
	if hasFlag(args, "--detect") {
		return nil
	}
	if !hasFlag(args, "--set") && !confirmed(in) {
		report("nothing written")
		return nil
	}
	if err := registry.SetGate(store, project.Name, steps); err != nil {
		return err
	}
	report("gate set for " + project.Name)
	return nil
}

// confirmed reads the one answer. Anything but an explicit yes is a no: the
// default has to be "write nothing", because this is the step that makes a
// command runnable.
func confirmed(in io.Reader) bool {
	report("")
	report("use it? (y to confirm, anything else to decline)")
	answer := strings.ToLower(readLine(in))
	return answer == "y" || answer == "yes"
}

// reportGate prints a gate the way a person reads it, commands verbatim, so
// what is about to become runnable is visible before it is confirmed.
func reportGate(heading string, steps []gate.Step) {
	report(heading)
	for _, step := range steps {
		line := fmt.Sprintf("  %-5s $ %s", step.Name, step.Run)
		if step.Kind != "" {
			line += "   [" + step.Kind + "]"
		}
		report(line)
	}
}

// gateProject resolves the project argument, naming this command in the error
// the way namedProject names adopt.
func gateProject(store *registry.Store, args []string) (registry.Project, error) {
	if len(args) == 0 || strings.HasPrefix(args[0], "-") {
		return registry.Project{}, fmt.Errorf("gate: want <project> [--detect|--set|--clear], got no project")
	}
	p, err := registry.NamedProject(store, args[0])
	if err != nil {
		return registry.Project{}, fmt.Errorf("gate: %w", err)
	}
	return p, nil
}

// hasFlag reports whether args carries flag.
func hasFlag(args []string, flag string) bool {
	for _, a := range args {
		if a == flag {
			return true
		}
	}
	return false
}
