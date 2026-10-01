package gate

import (
	"context"
	dgate "github.com/WilsonSousajr/omatty/internal/domain/gate"
	"testing"
	"testing/synctest"
	"time"

	"github.com/WilsonSousajr/omatty/internal/pubsub"
)

// passes is a run that finishes at once with one passing step.
func passes(context.Context, string, []dgate.Step) ([]dgate.StepResult, error) {
	return []dgate.StepResult{{Step: dgate.Step{Name: "ok"}, Verdict: dgate.Pass}}, nil
}

// ADR 0001's event model, step 5.3 (#653): a gate's report reaches its readers
// through a pubsub.Broker, so the TUI is one subscriber among any - not the one
// reader of a channel. Two subscribers both hear one report.
func TestRunner_everySubscriberHearsTheReport_issue653(t *testing.T) {
	r := NewRunner(1, passes)
	defer r.Close()
	a, b := r.Subscribe(t.Context()), r.Subscribe(t.Context())

	r.Start("s1", "/p", nil)

	for name, ch := range map[string]<-chan pubsub.Event[dgate.Report]{"a": a, "b": b} {
		select {
		case e := <-ch:
			if e.Payload.ID != "s1" || len(e.Payload.Results) != 1 {
				t.Errorf("subscriber %s got %+v, want s1's one result", name, e.Payload)
			}
		case <-time.After(5 * time.Second):
			t.Fatalf("subscriber %s heard nothing", name)
		}
	}
}

// A gate can finish before the TUI subscribes - an auto-run started as the
// program boots. Published straight away, to a broker with no subscribers
// yet, that report would be lost and the card would never show its verdict.
// The pump therefore starts on the first Subscribe (#653). synctest makes the
// losing order certain rather than lucky: Wait lets an eager pump drain the
// report before the subscription exists.
func TestRunner_aReportFinishedBeforeAnyoneSubscribedIsNotLost_issue653(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		r := NewRunner(1, passes)
		defer r.Close()
		r.Start("s1", "/p", nil)
		synctest.Wait()

		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		select {
		case e := <-r.Subscribe(ctx):
			if e.Payload.ID != "s1" {
				t.Errorf("got %+v, want s1's report", e.Payload)
			}
		case <-time.After(time.Minute):
			t.Fatal("the report finished before anyone subscribed was lost")
		}
	})
}
