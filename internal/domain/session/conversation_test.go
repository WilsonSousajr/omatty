package session_test

import (
	"testing"

	"github.com/WilsonSousajr/omatty/internal/domain/session"
)

// ConversationID moved here with Session (migration step 3.1, #635). Its
// callers test it through the store, so it is pinned in its own package: a
// /clear moves claude to a new conversation while the row keeps its ID (#316).
func TestSession_ConversationIDIsTheIDUntilAClearMovesIt_issue635(t *testing.T) {
	s := session.Session{ID: "row"}
	if got := s.ConversationID(); got != "row" {
		t.Errorf("fresh session: ConversationID() = %q, want its ID", got)
	}
	s.Conversation = "after-clear"
	if got := s.ConversationID(); got != "after-clear" {
		t.Errorf("after /clear: ConversationID() = %q, want the new conversation", got)
	}
}
