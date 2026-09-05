package database

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/eterealink/eterealink/backend/internal/domain"
)

func TestPostgresFolderEventsCommitAndRollback(t *testing.T) {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL is not set")
	}
	ctx := context.Background()
	database, err := Open(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	now := time.Date(2026, time.September, 5, 12, 0, 0, 0, time.UTC)
	owner := domain.User{ID: "95000000-0000-4000-8000-000000000001", FirebaseUID: "phase6-owner", Email: "phase6-owner@example.com", DisplayName: "Realtime Owner", CreatedAt: now}
	folder := domain.Folder{ID: "96000000-0000-4000-8000-000000000001", OwnerID: owner.ID, Name: "Realtime", CreatedAt: now}
	child := domain.Folder{ID: "96000000-0000-4000-8000-000000000002", OwnerID: owner.ID, ParentFolderID: &folder.ID, Name: "Drafts", CreatedAt: now}
	cleanup := func() {
		_, _ = database.pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, owner.ID)
	}
	cleanup()
	defer cleanup()
	if _, err := database.UpsertUser(ctx, owner); err != nil {
		t.Fatal(err)
	}
	if err := database.CreateFolder(ctx, folder); err != nil {
		t.Fatal(err)
	}
	if err := database.CreateFolder(ctx, child); err != nil {
		t.Fatal(err)
	}

	listenerContext, stopListener := context.WithCancel(ctx)
	defer stopListener()
	ready := make(chan struct{})
	events := make(chan string, 4)
	listenerDone := make(chan error, 1)
	go func() {
		listenerDone <- database.listenFolderEvents(listenerContext, func() { close(ready) }, func(folderID string) { events <- folderID })
	}()
	select {
	case <-ready:
	case <-time.After(time.Second):
		t.Fatal("database listener did not become ready")
	}

	tx, err := database.pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, `SELECT eterealink_notify_folder($1)`, folder.ID); err != nil {
		_ = tx.Rollback(ctx)
		t.Fatal(err)
	}
	if err := tx.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	select {
	case event := <-events:
		t.Fatalf("rolled-back event was delivered for %q", event)
	case <-time.After(100 * time.Millisecond):
	}

	if _, err := database.UpdateFolder(ctx, owner.ID, folder.ID, "Realtime plans", nil); err != nil {
		t.Fatal(err)
	}
	select {
	case event := <-events:
		if event != folder.ID {
			t.Fatalf("event folder = %q", event)
		}
	case <-time.After(time.Second):
		t.Fatal("committed folder change did not produce an event")
	}
	if _, err := database.UpdateFolder(ctx, owner.ID, child.ID, "Final drafts", &folder.ID); err != nil {
		t.Fatal(err)
	}
	renamedFolderEvents := map[string]bool{}
	for len(renamedFolderEvents) < 2 {
		select {
		case event := <-events:
			renamedFolderEvents[event] = true
		case <-time.After(time.Second):
			t.Fatalf("child rename events = %#v", renamedFolderEvents)
		}
	}
	if !renamedFolderEvents[child.ID] || !renamedFolderEvents[folder.ID] {
		t.Fatalf("child rename events = %#v", renamedFolderEvents)
	}

	file := domain.File{
		ID: "97000000-0000-4000-8000-000000000001", OwnerID: &owner.ID, FolderID: &folder.ID,
		StorageKey: "phase6-test/file-1", OriginalName: "realtime.txt", MIMEType: "text/plain", SizeBytes: 8,
		Status: domain.FileStatusPending, CreatedAt: now,
	}
	if err := database.CreateOwnedFile(ctx, file); err != nil {
		t.Fatal(err)
	}
	select {
	case event := <-events:
		t.Fatalf("pending file produced a visible-folder event for %q", event)
	case <-time.After(100 * time.Millisecond):
	}
	if _, err := database.CompleteOwnedFile(ctx, owner.ID, file.ID, now); err != nil {
		t.Fatal(err)
	}
	select {
	case event := <-events:
		if event != folder.ID {
			t.Fatalf("completed file event folder = %q", event)
		}
	case <-time.After(time.Second):
		t.Fatal("completed file did not produce an event")
	}

	stopListener()
	select {
	case <-listenerDone:
	case <-time.After(time.Second):
		t.Fatal("database listener did not stop")
	}
}
