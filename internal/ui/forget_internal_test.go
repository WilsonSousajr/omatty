package ui

import (
	"reflect"
	"sort"
	"testing"
	"time"

	"github.com/WilsonSousajr/omatty/internal/coverage"
	"github.com/WilsonSousajr/omatty/internal/gate"
	"github.com/WilsonSousajr/omatty/internal/registry"
	"github.com/WilsonSousajr/omatty/internal/review"
	"github.com/WilsonSousajr/omatty/internal/termwrap"
	"github.com/WilsonSousajr/omatty/internal/watcher"
)

// The archived session whose id must not survive anywhere on the Model.
const forgottenID = "11111111-2222-3333-4444-555555555555"

// sessionMaps is every Model field that is a map keyed by a session id, found
// by reflection rather than by a written list.
//
// The list is derived for the reason depguard's allowlists are asserted
// against `go list` rather than merely written down: a hand-maintained list
// stops describing the struct and nothing notices. Archive cleared five of
// fifteen of these maps for eleven milestones, and `covers` - a map[int]bool
// per source line, per file - stayed resident for the life of the process
// after the session was gone.
//
// A map[string]T added later that is NOT keyed by session id is the one false
// positive this can produce. That is deliberate: it costs whoever adds it one
// line in skipSessionMaps, and it buys the guarantee for every other map.
func sessionMaps(m *Model) map[string]reflect.Value {
	v := reflect.ValueOf(m).Elem()
	t := v.Type()
	found := map[string]reflect.Value{}
	for i := range t.NumField() {
		f := v.Field(i)
		if f.Kind() == reflect.Map && f.Type().Key().Kind() == reflect.String && !skipSessionMaps[t.Field(i).Name] {
			found[t.Field(i).Name] = f
		}
	}
	return found
}

// skipSessionMaps names the string-keyed Model maps that are NOT keyed by a
// session id, and so are none of archive's business. Empty today.
var skipSessionMaps = map[string]bool{}

// mapsHolding is the names of the session maps that still hold id, sorted so
// a failure reads the same way twice.
func mapsHolding(m *Model, id string) []string {
	var held []string
	for name, f := range sessionMaps(m) {
		if f.MapIndex(reflect.ValueOf(id)).IsValid() {
			held = append(held, name)
		}
	}
	sort.Strings(held)
	return held
}

// mapsMissing is the inverse: the session maps the fixture failed to fill.
func mapsMissing(m *Model, id string) []string {
	var missing []string
	for name, f := range sessionMaps(m) {
		if !f.MapIndex(reflect.ValueOf(id)).IsValid() {
			missing = append(missing, name)
		}
	}
	sort.Strings(missing)
	return missing
}

// filledModel is a Model holding one session with an entry for it in every
// per-session map. Each map is written explicitly, not by reflection: setting
// an unexported field through reflect needs unsafe, and an explicit fixture is
// what makes the guard below fail loudly when a new map appears.
func filledModel() *Model {
	sess := registry.Session{ID: forgottenID, Project: "p", Dir: "/tmp/p"}
	m := NewModel(Deps{
		State: registry.State{
			Projects: []registry.Project{{Name: "p", Root: "/tmp/p"}},
			Sessions: []registry.Session{sess},
		},
		Terms:      map[string]termwrap.Terminal{},
		Reattached: map[string]bool{},
	})
	m.status[forgottenID] = watcher.SessionState{}
	m.notified[forgottenID] = time.Unix(0, 0)
	m.comments[forgottenID] = &review.Comments{}
	m.namePending[forgottenID] = true
	m.lane[forgottenID] = activityLane{}
	m.gates[forgottenID] = gate.Report{}
	m.gateRunning[forgottenID] = true
	m.covers[forgottenID] = coverage.Profile{}
	m.coverFailed[forgottenID] = true
	m.repoStat[forgottenID] = review.Stat{}
	m.statPending[forgottenID] = true
	m.statFailed[forgottenID] = true
	m.filesPending[forgottenID] = true
	m.reattached[forgottenID] = true
	m.terms[forgottenID] = nil
	return m
}

// TestForgetSession_ClearsEveryPerSessionMap is the negative control for the
// leak: archiving a session must leave its id in no map at all.
func TestForgetSession_ClearsEveryPerSessionMap(t *testing.T) {
	m := filledModel()
	if missing := mapsMissing(m, forgottenID); len(missing) > 0 {
		t.Fatalf("fixture does not fill %v: a per-session map was added to Model without being filled here, "+
			"so this test cannot prove archive clears it - fill it in filledModel", missing)
	}

	m.forgetSession(forgottenID)

	if held := mapsHolding(m, forgottenID); len(held) > 0 {
		t.Errorf("forgetSession left %v holding %q: an archived session's state stays resident "+
			"for the life of the process", held, forgottenID)
	}
}
