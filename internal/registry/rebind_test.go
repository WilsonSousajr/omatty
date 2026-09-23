package registry_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/WilsonSousajr/omatty/internal/registry"
)

// seedReboundPair saves a project with two sessions, the second already
// re-bound once, so a test can aim a conversation at the other row.
func seedReboundPair(t *testing.T) *registry.Store {
	t.Helper()
	store, _ := newStoreAt(t)
	st := registry.State{Version: registry.Version,
		Projects: []registry.Project{{Name: "omatty", Root: "/p/omatty"}},
		Sessions: []registry.Session{
			{ID: "row-1", Project: "omatty", Title: "one", Dir: "/p/omatty"},
			{ID: "row-2", Project: "omatty", Title: "two", Dir: "/p/omatty", Conversation: "conv-2"},
		}}
	if err := store.Save(st); err != nil {
		t.Fatal(err)
	}
	return store
}

func TestSession_ConversationIDIsIDUntilRebound_issue316(t *testing.T) {
	fresh := registry.Session{ID: "row-1"}
	cleared := registry.Session{ID: "row-1", Conversation: "after-clear"}

	if fresh.ConversationID() != "row-1" || cleared.ConversationID() != "after-clear" {
		t.Errorf("ConversationID = %q and %q, want row-1 and after-clear",
			fresh.ConversationID(), cleared.ConversationID())
	}
}

// Regression, issue #316: /clear moved claude to a new uuid and state.json
// never learned, so the next start resumed the conversation from before the
// clear. The row keeps its ID - the dtach socket is named after it - and
// records the conversation it is on now.
func TestRebindSession_PersistsTheNewConversation_issue316(t *testing.T) {
	store := seedReboundPair(t)

	if err := registry.RebindSession(store, "row-1", "after-clear"); err != nil {
		t.Fatalf("RebindSession() error = %v, want nil", err)
	}

	st, _ := store.Load()
	if got := st.Sessions[0]; got.ID != "row-1" || got.Conversation != "after-clear" {
		t.Errorf("row after rebind = %+v, want ID row-1 on conversation after-clear", got)
	}
}

// Rebinding back to the row's own id writes the empty value, so one
// conversation never has two spellings in state.json.
func TestRebindSession_BackToItsOwnIDStoresTheEmptyValue_issue316(t *testing.T) {
	store := seedReboundPair(t)

	if err := registry.RebindSession(store, "row-2", "row-2"); err != nil {
		t.Fatal(err)
	}

	st, _ := store.Load()
	if got := st.Sessions[1].Conversation; got != "" {
		t.Errorf("Conversation = %q, want empty for a row on its own id", got)
	}
}

// A conversation another row already holds is refused: two panes resuming
// one transcript would interleave two claudes in one file (#316).
func TestRebindSession_RefusesAConversationAnotherRowHolds_issue316(t *testing.T) {
	for _, taken := range []string{"row-2", "conv-2"} {
		store := seedReboundPair(t)

		err := registry.RebindSession(store, "row-1", taken)

		if err == nil || !strings.Contains(err.Error(), taken) || !strings.Contains(err.Error(), "row-2") {
			t.Errorf("RebindSession onto %q = %v, want an error naming it and row-2", taken, err)
		}
		if st, _ := store.Load(); st.Sessions[0].Conversation != "" {
			t.Errorf("row-1 was rebound to %q despite the refusal", st.Sessions[0].Conversation)
		}
	}
}

func TestRebindSession_UnknownSessionIsAnError_issue316(t *testing.T) {
	store := seedReboundPair(t)

	if err := registry.RebindSession(store, "nope", "after-clear"); err == nil {
		t.Error("RebindSession of an unknown id = nil, want an error")
	}
}

// Adoption leaves out what omatty holds. A post-clear transcript is held, so
// it must not be offered as a session to adopt a second time (#316).
func TestKnownSessionIDs_IncludesReboundConversations_issue316(t *testing.T) {
	store := seedReboundPair(t)

	ids, err := registry.KnownSessionIDs(store)

	if err != nil || !slices.Contains(ids, "conv-2") || !slices.Contains(ids, "row-2") {
		t.Errorf("KnownSessionIDs = %v (err %v), want row-2 and conv-2 both", ids, err)
	}
}
