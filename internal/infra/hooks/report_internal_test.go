package hooks

import (
	"net"
	"testing"
	"time"
)

// Regression, issue #57: the hook set a dial timeout but no write deadline, so
// a peer that accepted the connection and never read from it held the hook
// open indefinitely. Invariant 11 says a hook must never block claude, and a
// blocked hook stalls every claude session on the machine, not just omatty's.
//
// net.Pipe is the connection this needs: it is synchronous and unbuffered, so
// the write parks until someone reads. A real unix socket cannot reproduce the
// bug at all — a payload this small disappears into the kernel send buffer and
// returns whether or not the peer ever reads.
func TestSendLine_ReturnsWhenThePeerNeverReads_issue57(t *testing.T) {
	client, server := net.Pipe()
	defer func() { _ = client.Close() }()
	defer func() { _ = server.Close() }()
	// server is deliberately never read from.

	done := make(chan struct{})
	go func() {
		defer close(done)
		sendLine(client, Payload{SessionID: "abc", HookEventName: "Stop"}, 50*time.Millisecond)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("sendLine blocked on a peer that never reads: the write deadline is missing (issue #57)")
	}
}

// The deadline must not cost anything on the ordinary path: a peer that reads
// gets the line, and sendLine returns as soon as it is written.
func TestSendLine_WritesTheLineWhenThePeerReads_issue57(t *testing.T) {
	client, server := net.Pipe()
	defer func() { _ = client.Close() }()
	defer func() { _ = server.Close() }()

	got := make(chan string, 1)
	go func() {
		buf := make([]byte, 256)
		n, err := server.Read(buf)
		if err != nil {
			return
		}
		got <- string(buf[:n])
	}()

	sendLine(client, Payload{SessionID: "abc", HookEventName: "Stop"}, time.Second)

	select {
	case line := <-got:
		want := `{"session_id":"abc","hook_event_name":"Stop"}` + "\n"
		if line != want {
			t.Errorf("sendLine wrote %q, want %q", line, want)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("the reading peer never received the line")
	}
}
