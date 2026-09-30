// Package pubsub is how omatty's services tell their subscribers - the TUI,
// the notify adapter - that something changed, without either side knowing the
// other (ADR 0001, "The event model"). It imports nothing but the standard
// library.
//
// Delivery is chosen by the publisher, and keeps what today's channels do:
// Publish waits for room and never drops, for the transcript tailer and the
// gate runner, whose events are the truth a screen shows (invariant 2); Offer
// never waits and drops what does not fit, for the hook server, because a hook
// must never wait on omatty (invariant 11).
//
//	b := pubsub.NewBroker[gate.Report](64)
//	reports := b.Subscribe(ctx)
//	_ = b.Publish(ctx, pubsub.Event[gate.Report]{Kind: pubsub.Updated, Payload: rep})
package pubsub

import (
	"context"
	"sync"
	"sync/atomic"
)

// Kind says what happened to the payload.
type Kind int

// The three things that happen to a record.
const (
	Created Kind = iota
	Updated
	Deleted
)

// Event is one change, as a subscriber receives it.
type Event[T any] struct {
	Kind    Kind
	Payload T
}

// Broker fans events out to every current subscriber, each with its own
// bounded buffer.
type Broker[T any] struct {
	buffer  int
	mu      sync.Mutex
	subs    map[*subscriber[T]]struct{}
	dropped atomic.Uint64
}

// subscriber is one reader: its buffer, and done, closed when it leaves, so a
// publisher selecting on it never waits for a reader that is gone. The data
// channel is never closed - a send racing the departure would panic - and a
// reader stops on its own context instead.
type subscriber[T any] struct {
	ch   chan Event[T]
	done chan struct{}
}

// NewBroker returns a Broker giving each subscriber buffer events of room.
// Today's channels hold 64 (watcher.eventBuffer, gate.reportBuffer).
//
//	b := pubsub.NewBroker[status.Event](64)
func NewBroker[T any](buffer int) *Broker[T] {
	return &Broker[T]{buffer: buffer, subs: map[*subscriber[T]]struct{}{}}
}

// Subscribe returns a channel of every event published from now on, until ctx
// ends; the TUI drains it with a re-armed Cmd.
//
//	events := b.Subscribe(ctx)
func (b *Broker[T]) Subscribe(ctx context.Context) <-chan Event[T] {
	s := &subscriber[T]{ch: make(chan Event[T], b.buffer), done: make(chan struct{})}
	b.mu.Lock()
	b.subs[s] = struct{}{}
	b.mu.Unlock()
	go b.leaveWhenDone(ctx, s)
	return s.ch
}

func (b *Broker[T]) leaveWhenDone(ctx context.Context, s *subscriber[T]) {
	<-ctx.Done()
	b.mu.Lock()
	delete(b.subs, s)
	b.mu.Unlock()
	close(s.done)
}

// Publish delivers e to every subscriber, waiting for room in each buffer; it
// returns ctx's error if ctx ends first, and never drops.
//
//	err := b.Publish(ctx, pubsub.Event[gate.Report]{Kind: pubsub.Updated, Payload: rep})
func (b *Broker[T]) Publish(ctx context.Context, e Event[T]) error {
	for _, s := range b.current() {
		select {
		case s.ch <- e:
		case <-s.done:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return nil
}

// Offer delivers e to every subscriber with room, never waiting, and reports
// whether all of them took it. What does not fit is dropped and counted.
//
//	_ = b.Offer(pubsub.Event[status.HookPayload]{Kind: pubsub.Created, Payload: p})
func (b *Broker[T]) Offer(e Event[T]) bool {
	delivered := true
	for _, s := range b.current() {
		select {
		case s.ch <- e:
		case <-s.done:
		default:
			b.dropped.Add(1)
			delivered = false
		}
	}
	return delivered
}

// Dropped is how many deliveries Offer has dropped, for the log.
func (b *Broker[T]) Dropped() uint64 { return b.dropped.Load() }

// current is a snapshot of the subscribers, so no send happens under the lock.
func (b *Broker[T]) current() []*subscriber[T] {
	b.mu.Lock()
	defer b.mu.Unlock()
	out := make([]*subscriber[T], 0, len(b.subs))
	for s := range b.subs {
		out = append(out, s)
	}
	return out
}
