package realtime

import (
	"context"
	"log/slog"
	"strings"
	"sync"
	"time"
)

const (
	initialRetryDelay = time.Second
	maximumRetryDelay = 30 * time.Second
)

type FolderEvent struct {
	FolderID string `json:"folderId"`
}

type FolderEventSource interface {
	ListenFolderEvents(ctx context.Context, publish func(folderID string)) error
}

type FolderEventSubscriber interface {
	Subscribe(folderIDs []string) (<-chan FolderEvent, func())
}

type FolderBroker struct {
	mu          sync.RWMutex
	nextID      uint64
	subscribers map[uint64]folderSubscription
}

type folderSubscription struct {
	folderIDs map[string]struct{}
	events    chan FolderEvent
}

func NewFolderBroker() *FolderBroker {
	return &FolderBroker{subscribers: make(map[uint64]folderSubscription)}
}

func (b *FolderBroker) Subscribe(folderIDs []string) (<-chan FolderEvent, func()) {
	folders := make(map[string]struct{}, len(folderIDs))
	for _, folderID := range folderIDs {
		if folderID = strings.TrimSpace(folderID); folderID != "" {
			folders[folderID] = struct{}{}
		}
	}
	events := make(chan FolderEvent, 1)

	b.mu.Lock()
	b.nextID++
	id := b.nextID
	b.subscribers[id] = folderSubscription{folderIDs: folders, events: events}
	b.mu.Unlock()

	var once sync.Once
	cancel := func() {
		once.Do(func() {
			b.mu.Lock()
			delete(b.subscribers, id)
			close(events)
			b.mu.Unlock()
		})
	}
	return events, cancel
}

func (b *FolderBroker) Publish(folderID string) {
	folderID = strings.TrimSpace(folderID)
	if folderID == "" {
		return
	}
	event := FolderEvent{FolderID: folderID}
	b.mu.RLock()
	defer b.mu.RUnlock()
	for _, subscriber := range b.subscribers {
		if _, ok := subscriber.folderIDs[folderID]; !ok {
			continue
		}
		select {
		case subscriber.events <- event:
		default:
			// Folder events are invalidation hints. One queued event is enough to
			// make the browser refetch authoritative state.
		}
	}
}

func (b *FolderBroker) Run(ctx context.Context, source FolderEventSource, logger *slog.Logger) {
	retryDelay := initialRetryDelay
	for ctx.Err() == nil {
		connectedAt := time.Now()
		err := source.ListenFolderEvents(ctx, b.Publish)
		if ctx.Err() != nil {
			return
		}
		if time.Since(connectedAt) >= time.Minute {
			retryDelay = initialRetryDelay
		}
		logger.Warn("folder event listener disconnected", "error", err, "retry_in", retryDelay)
		timer := time.NewTimer(retryDelay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
		if retryDelay < maximumRetryDelay {
			retryDelay *= 2
			if retryDelay > maximumRetryDelay {
				retryDelay = maximumRetryDelay
			}
		}
	}
}
