package session_test

import (
	"testing"

	"github.com/WilsonSousajr/omatty/internal/domain/session"
)

// claude names the conversation it was in when it exits, so `omatty --resume
// <id>` finds the row by conversation first (#815). After a /clear that is the
// row's Conversation, not its ID (#316).
func TestState_SessionByConversation_findsTheRowAClearMoved_issue815(t *testing.T) {
	st := session.State{Sessions: []session.Session{
		{ID: "row-1"},
		{ID: "row-2", Conversation: "after-clear"},
	}}

	got, ok := st.SessionByConversation("after-clear")

	if !ok || got.ID != "row-2" {
		t.Errorf("SessionByConversation(after-clear) = %q, %v; want row-2, true", got.ID, ok)
	}
}

// A row nothing has cleared resumes by its own ID, which is its conversation.
func TestState_SessionByConversation_findsAnUnclearedRowByID_issue815(t *testing.T) {
	st := session.State{Sessions: []session.Session{{ID: "row-1"}}}

	got, ok := st.SessionByConversation("row-1")

	if !ok || got.ID != "row-1" {
		t.Errorf("SessionByConversation(row-1) = %q, %v; want row-1, true", got.ID, ok)
	}
}

// A row a /clear moved no longer owns its old conversation: resuming that one
// would open the row on a conversation claude has left.
func TestState_SessionByConversation_refusesAConversationTheRowLeft_issue815(t *testing.T) {
	st := session.State{Sessions: []session.Session{{ID: "row-2", Conversation: "after-clear"}}}

	if got, ok := st.SessionByConversation("row-2"); ok {
		t.Errorf("SessionByConversation(row-2) = %q, true; want no row, since it moved to after-clear", got.ID)
	}
}
