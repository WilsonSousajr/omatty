package pubsub_test

import (
	"context"
	"errors"
	"testing"

	"github.com/WilsonSousajr/omatty/internal/pubsub"
)

func created(n int) pubsub.Event[int] { return pubsub.Event[int]{Kind: pubsub.Created, Payload: n} }

// ADR 0001's event model (#653): every subscriber gets every event.
func TestBroker_everySubscriberGetsEveryEvent_issue653(t *testing.T) {
	b := pubsub.NewBroker[int](4)
	a, c := b.Subscribe(t.Context()), b.Subscribe(t.Context())

	if err := b.Publish(t.Context(), created(7)); err != nil {
		t.Fatal(err)
	}

	for name, ch := range map[string]<-chan pubsub.Event[int]{"a": a, "c": c} {
		if got := <-ch; got != created(7) {
			t.Errorf("subscriber %s got %+v, want Created 7", name, got)
		}
	}
}

// Publish never drops: with the subscriber's buffer full it waits, and a
// cancelled context is the only way out. The tailer and the gate runner block
// today (watcher/tailer.go, gate/runner.go), and a lost verdict is a wrong
// screen.
func TestBroker_publishWaitsRatherThanDrop_issue653(t *testing.T) {
	b := pubsub.NewBroker[int](1)
	sub := b.Subscribe(t.Context())
	if err := b.Publish(t.Context(), created(1)); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	err := b.Publish(ctx, created(2))

	if !errors.Is(err, context.Canceled) {
		t.Errorf("Publish into a full buffer = %v, want it to wait until its context ended", err)
	}
	if got := <-sub; got != created(1) {
		t.Errorf("the buffered event is %+v, want Created 1", got)
	}
}

// Offer never waits: into a full buffer it drops, says so, and counts it. The
// hook server offers, so a hook never waits on omatty (invariant 11).
func TestBroker_offerDropsAndCountsRatherThanWait_issue653(t *testing.T) {
	b := pubsub.NewBroker[int](1)
	b.Subscribe(t.Context())

	first, second := b.Offer(created(1)), b.Offer(created(2))

	if !first || second {
		t.Errorf("Offer into an empty then a full buffer = %v, %v; want true, false", first, second)
	}
	if b.Dropped() != 1 {
		t.Errorf("Dropped() = %d, want 1", b.Dropped())
	}
}

// A subscriber whose context ended is gone: a publisher never waits on a
// reader that has left, even with its buffer full.
func TestBroker_aCancelledSubscriberNeverBlocksAPublisher_issue653(t *testing.T) {
	b := pubsub.NewBroker[int](1)
	ctx, cancel := context.WithCancel(t.Context())
	b.Subscribe(ctx)
	if err := b.Publish(t.Context(), created(1)); err != nil {
		t.Fatal(err)
	}
	cancel()

	if err := b.Publish(t.Context(), created(2)); err != nil {
		t.Errorf("Publish after the only subscriber left = %v, want nil", err)
	}
}

// With no subscribers there is nobody to wait for or drop on.
func TestBroker_noSubscribersIsNotAnError_issue653(t *testing.T) {
	b := pubsub.NewBroker[int](1)
	if err := b.Publish(t.Context(), created(1)); err != nil || !b.Offer(created(2)) {
		t.Errorf("Publish/Offer with no subscribers = %v, want nil and delivered", err)
	}
}
