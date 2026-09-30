package status

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"github.com/WilsonSousajr/omatty/internal/domain/session"

	"github.com/WilsonSousajr/omatty/internal/infra/paths"
	"github.com/WilsonSousajr/omatty/internal/pubsub"
)

// ADR 0001's event model, step 5.2a (#653): status reaches its readers through
// a pubsub.Broker, so the TUI is one subscriber among any - not the one reader
// of a channel. Two subscribers both hear one hook event.
func TestWatch_everySubscriberHearsTheSameEvent_issue653(t *testing.T) {
	home := shortHome(t)
	w := Start(claudeDeps(home), twoSessions())
	defer w.Close()
	a, b := w.Subscribe(t.Context()), w.Subscribe(t.Context())

	c, err := net.Dial("unix", paths.HookSocket(home))
	if err != nil {
		t.Fatal(err)
	}
	_, _ = fmt.Fprintf(c, "%s\n", `{"session_id":"s1","hook_event_name":"PermissionRequest"}`)
	_ = c.Close()

	for name, ch := range map[string]<-chan pubsub.Event[Event]{"a": a, "b": b} {
		select {
		case ev := <-ch:
			if ev.Payload.SessionID != "s1" || ev.Payload.Kind != PermissionRequested {
				t.Errorf("subscriber %s got %+v, want s1 PermissionRequested", name, ev.Payload)
			}
		case <-time.After(2 * time.Second):
			t.Fatalf("subscriber %s heard nothing", name)
		}
	}
}

// A tailer reports a session's status the moment Start adds it - before
// anything can subscribe. Published straight away, to a broker with no
// subscribers yet, that status would be lost and the card would open wrong.
// The pump therefore starts on the first Subscribe (#653). synctest makes the
// losing order certain rather than lucky: Wait lets an eager pump drain the
// event before the subscription exists. Tailer-only (the socket path is too
// long to bind), because a listening socket's I/O never settles in a bubble.
func TestWatch_aStatusReportedBeforeAnyoneSubscribedIsNotLost_issue653(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		home := filepath.Join(t.TempDir(), strings.Repeat("x", 120))
		sess := session.Session{ID: "s1", Project: "p", Title: "one", Dir: "/p"}
		w := Start(claudeDeps(home), []session.Session{sess})
		defer w.Close()
		transcript := paths.Transcript(home, sess.Dir, sess.ID)
		if err := os.MkdirAll(filepath.Dir(transcript), 0o700); err != nil {
			t.Fatal(err)
		}
		line := `{"type":"user","timestamp":"2026-09-02T12:00:01Z","message":{"role":"user","content":"hi"}}`
		if err := os.WriteFile(transcript, []byte(line+"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		w.tailers["s1"].Poll()
		synctest.Wait()

		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		select {
		case e := <-w.Subscribe(ctx):
			if e.Payload.Kind != PromptSubmitted {
				t.Errorf("got %+v, want the startup PromptSubmitted", e.Payload)
			}
		case <-time.After(time.Minute):
			t.Fatal("the status reported before anyone subscribed was lost")
		}
	})
}
