// `omatty carry`: the gitignored files copied into each new worktree of a
// project. Thin over internal/registry (invariant 10).

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/WilsonSousajr/omatty/internal/registry"
)

// carryCommand shows, sets or clears a project's carry list.
//
//	omatty carry <project>                  show it
//	omatty carry <project> .env certs       set it
//	omatty carry <project> --clear          forget it
//
// Setting replaces rather than appends, which is what `gate` does and the same
// reason: a list you can only add to needs a remove command to be usable, and
// re-typing two paths is cheaper than that.
func carryCommand(store *registry.Store, args []string) error {
	project, err := carryProject(store, args)
	if err != nil {
		return err
	}
	if hasFlag(args, "--clear") {
		return clearCarry(store, project.Name)
	}
	paths := carryPaths(args)
	if len(paths) == 0 {
		reportCarry(project)
		return nil
	}
	warnMissing(project.Root, paths)
	if err := registry.SetCarry(store, project.Name, paths); err != nil {
		return err
	}
	report(fmt.Sprintf("carry list set for %s: %s", project.Name, strings.Join(paths, " ")))
	return nil
}

// clearCarry forgets the list and says so. Silence would be
// indistinguishable from a command that did nothing, which is the argument
// gateCommand's own --clear arm makes.
func clearCarry(store *registry.Store, project string) error {
	if err := registry.ClearCarry(store, project); err != nil {
		return err
	}
	report("carry list cleared for " + project)
	return nil
}

// carryPaths is every argument after the project that is not a flag.
func carryPaths(args []string) []string {
	if len(args) < 2 {
		return nil
	}
	paths := make([]string, 0, len(args)-1)
	for _, a := range args[1:] {
		if !strings.HasPrefix(a, "-") {
			paths = append(paths, a)
		}
	}
	return paths
}

// warnMissing names a path the checkout does not have. It is a warning, not a
// refusal: the operator may be about to create the file, and a list that
// cannot be set before the file exists would be the more annoying rule. But
// silence would let a typo sit there until a worktree quietly came up short.
func warnMissing(root string, paths []string) {
	for _, rel := range paths {
		if _, err := os.Lstat(filepath.Join(root, rel)); err != nil {
			report(fmt.Sprintf("  note: %q is not in %s yet; it will be carried once it is", rel, root))
		}
	}
}

// reportCarry prints the list, or says there is none. "Not set" and "set to
// nothing" are the same state here, and saying which files would be copied is
// the question the plain form answers.
func reportCarry(p registry.Project) {
	if len(p.Carry) == 0 {
		report("no carry list for " + p.Name + "; set one with `omatty carry " + p.Name + " <path>...`")
		return
	}
	report("every new worktree of " + p.Name + " carries:")
	for _, rel := range p.Carry {
		report("  " + rel)
	}
}

// carryProject resolves the project argument, naming this command in the error
// the way gateProject does.
func carryProject(store *registry.Store, args []string) (registry.Project, error) {
	if len(args) == 0 || strings.HasPrefix(args[0], "-") {
		return registry.Project{}, fmt.Errorf("carry: want <project> [<path>...|--clear], got no project")
	}
	p, err := registry.NamedProject(store, args[0])
	if err != nil {
		return registry.Project{}, fmt.Errorf("carry: %w", err)
	}
	return p, nil
}
