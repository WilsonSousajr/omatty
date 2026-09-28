package ui

import (
	"errors"
	"log/slog"
	"strconv"
	"strings"

	"github.com/WilsonSousajr/omatty/internal/forge"
)

// LabelFunc names a project's forge and its unit of change, so no copy in the
// window hard-codes GitHub's words (#449). The argument is the project's root.
// It is called while drawing, so it must answer from what the forge layer
// already knows and never run a process.
//
//	d.Label = func(string) forge.Label { return forge.GitHub }
type LabelFunc func(projectRoot string) forge.Label

// unwiredLabel is the Deps.Label default: the unwired forge funcs answer as a
// machine whose gh is missing, so the words are gh's forge's too.
func unwiredLabel(string) forge.Label { return forge.GitHub }

// label is the project's forge words, looked up by its name.
func (m *Model) label(project string) forge.Label { return m.labelOf(m.projectRoot(project)) }

// stopsForge sorts a failed forge call. A missing tool, a token the forge
// refused, or a checkout on no forge omatty reads stops that project for the
// rest of the run and reports true: none of them changes while omatty runs,
// the environment included. Anything else is an outage the caller keeps as a
// stale list.
//
// Per project, not per machine: with several forges a missing glab says
// nothing about a GitHub project, which is why #449 retired the global flag.
func (m *Model) stopsForge(project string, err error) bool {
	if !stopping(err) {
		return false
	}
	m.loseForge(project, err)
	return true
}

// stopping is whether err is one of the answers that stop a project.
func stopping(err error) bool {
	var missing *forge.MissingToolError
	var refused *forge.AuthError
	return errors.As(err, &missing) || errors.As(err, &refused) || errors.Is(err, forge.ErrNoForge)
}

// loseForge stops both lists for one project and drops what either had read,
// said once in the log: a verdict nothing will refresh must not stand as
// current for the rest of the run (#310). Shared by both lists, because it is a
// fact about the checkout and the machine rather than about the list that
// happened to find it (#394).
func (m *Model) loseForge(project string, err error) {
	if m.forgeStopped[project] != nil {
		return
	}
	m.forgeStopped[project] = err
	delete(m.prs, project)
	delete(m.prFailed, project)
	delete(m.issues, project)
	delete(m.issueFailed, project)
	slog.Info("the project's forge cannot be read; it will show no changes or issues", "project", project, "err", err)
}

// stoppedNote is the tracker's note for a stopped project: the tool that is
// missing with its fix, the token the forge refused, or the forge the checkout
// is not on.
func (m *Model) stoppedNote(project string, err error) string {
	why, cannotRead := stoppedPhrase(err)
	if !cannotRead {
		return offForge(m.label(project))
	}
	return why + ", so omatty cannot read this project's issues or " + m.label(project).Change + "s."
}

// stoppedPhrase says why a stopped project cannot be read, and whether it is
// a reason the operator can fix; a checkout on no forge is not.
func stoppedPhrase(err error) (string, bool) {
	var missing *forge.MissingToolError
	var refused *forge.AuthError
	switch {
	case errors.As(err, &missing):
		return missingPhrase(missing), true
	case errors.As(err, &refused):
		return refused.Host + " refused " + refused.TokenEnv + " (" + strconv.Itoa(refused.Status) + ")", true
	}
	return "", false
}

// missingPhrase is "install X or set Y" said as a fact: what is missing, both
// halves when the forge has a fallback.
func missingPhrase(e *forge.MissingToolError) string {
	switch {
	case e.Tool == "":
		return e.TokenEnv + " is unset"
	case e.TokenEnv == "":
		return e.Tool + " is not installed"
	}
	return e.Tool + " is not installed and " + e.TokenEnv + " is unset"
}

// offForge names the forge a checkout is not on, or says none is known.
func offForge(l forge.Label) string {
	if l.Forge == "" {
		return "this project is not on a forge omatty reads; name its host in [forge.hosts]."
	}
	return "this project is not on " + l.Forge + "."
}

// countSuffix is the header's one-letter change count unit: "p" for PR, "m"
// for GitLab's MR, beside the issues' "i" (#395).
func countSuffix(l forge.Label) string {
	if l.Short == "" {
		return ""
	}
	return strings.ToLower(l.Short)[:1]
}

// changeRef writes a project's change the way its forge does, "!12" on GitLab.
func (m *Model) changeRef(project string, number int) string {
	return m.label(project).Ref(number)
}

// itemRef writes an item's number: a change the forge's way, an issue "#N"
// everywhere.
func (m *Model) itemRef(project string, pr bool, number int) string {
	if pr {
		return m.changeRef(project, number)
	}
	return "#" + strconv.Itoa(number)
}
