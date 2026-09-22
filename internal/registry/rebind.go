package registry

import "fmt"

// RebindSession records that the session's claude now runs conversation, as
// it does after /clear. The row keeps its ID: that is the dtach socket's
// name and the UI's key, and only the conversation changed (#316).
//
//	err := registry.RebindSession(store, sess.ID, payload.SessionID)
//
// Rebinding to the row's own id stores the empty value, so a conversation
// has one spelling in state.json. A conversation another row already holds
// is refused: two panes resuming one transcript would interleave two claudes
// in one file.
func RebindSession(s *Store, id, conversation string) error {
	st, err := s.Load()
	if err != nil {
		return err
	}
	i, err := indexOfSession(&st, id)
	if err != nil {
		return err
	}
	if err := refuseHeldConversation(&st, id, conversation); err != nil {
		return err
	}
	st.Sessions[i].Conversation = conversation
	if conversation == id {
		st.Sessions[i].Conversation = ""
	}
	return s.Save(st)
}

// refuseHeldConversation errors when a row other than id already answers to
// conversation, by its ID or by a rebind of its own.
func refuseHeldConversation(st *State, id, conversation string) error {
	for _, sess := range st.Sessions {
		if sess.ID != id && (sess.ID == conversation || sess.Conversation == conversation) {
			return fmt.Errorf(
				"registry: session %q: conversation %q is already held by session %q, want one no row holds",
				id, conversation, sess.ID)
		}
	}
	return nil
}
