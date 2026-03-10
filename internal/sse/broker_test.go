package sse

import (
	"testing"
	"time"
)

func TestBroadcastReachesSubscriber(t *testing.T) {
	b := NewBroker()
	ch := b.Subscribe()
	defer b.Unsubscribe(ch)

	go b.Broadcast("<div>test</div>")

	select {
	case msg := <-ch:
		if msg != "<div>test</div>" {
			t.Errorf("unexpected msg: %s", msg)
		}
	case <-time.After(1 * time.Second):
		t.Error("broadcast timeout")
	}
}

func TestSlowClientDoesNotBlock(t *testing.T) {
	b := NewBroker()

	// Subscribe but never read
	slow := b.Subscribe()
	defer b.Unsubscribe(slow)

	// Fill the buffer
	for i := 0; i < 20; i++ {
		b.Broadcast("msg")
	}

	// Fast subscriber should still work
	fast := b.Subscribe()
	defer b.Unsubscribe(fast)

	go b.Broadcast("final")

	select {
	case msg := <-fast:
		if msg != "final" {
			t.Errorf("unexpected: %s", msg)
		}
	case <-time.After(1 * time.Second):
		t.Error("fast client blocked by slow client")
	}
}
