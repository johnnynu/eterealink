package realtime

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"
)

func TestFolderBrokerTargetsAndCoalescesSubscribers(t *testing.T) {
	broker := NewFolderBroker()
	first, cancelFirst := broker.Subscribe([]string{"folder-1", "ancestor-1"})
	defer cancelFirst()
	second, cancelSecond := broker.Subscribe([]string{"folder-2"})
	defer cancelSecond()

	broker.Publish("ancestor-1")
	broker.Publish("folder-1")

	select {
	case event := <-first:
		if event.FolderID != "ancestor-1" {
			t.Fatalf("event = %#v", event)
		}
	case <-time.After(time.Second):
		t.Fatal("matching subscriber did not receive an event")
	}
	select {
	case event := <-first:
		t.Fatalf("duplicate invalidation was not coalesced: %#v", event)
	default:
	}
	select {
	case event := <-second:
		t.Fatalf("unrelated subscriber received %#v", event)
	default:
	}
}

func TestFolderBrokerStopsDeliveryAfterCancel(t *testing.T) {
	broker := NewFolderBroker()
	events, cancel := broker.Subscribe([]string{"folder-1"})
	cancel()
	cancel()
	broker.Publish("folder-1")
	if _, ok := <-events; ok {
		t.Fatal("canceled subscription remained open")
	}
}

func TestFolderBrokerReconnectsListener(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	source := &failingFolderEventSource{called: make(chan struct{})}
	broker := NewFolderBroker()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	go broker.Run(ctx, source, logger)

	select {
	case <-source.called:
	case <-time.After(2 * time.Second):
		t.Fatal("listener was not retried")
	}
}

type failingFolderEventSource struct {
	mu     sync.Mutex
	calls  int
	called chan struct{}
}

func (s *failingFolderEventSource) ListenFolderEvents(context.Context, func(string)) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls++
	if s.calls == 2 {
		close(s.called)
	}
	return errors.New("listener unavailable")
}
