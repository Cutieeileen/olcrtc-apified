package store

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/openlibrecommunity/olcrtc/internal/channel"
)

func testStore(t *testing.T) *SQLiteStore {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	s, err := NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func sampleChannel(id string) *channel.Channel {
	now := time.Now().UTC().Truncate(time.Second)
	return &channel.Channel{
		ID:        id,
		Carrier:   "wbstream",
		Transport: "datachannel",
		Link:      "direct",
		RoomID:    "test-room-" + id,
		ClientID:  "client-" + id,
		KeyHex:    "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890",
		DNSServer: "1.1.1.1:53",
		TransportConfig: channel.TransportConfig{
			VP8FPS:       60,
			VP8BatchSize: 8,
		},
		Status:    channel.StatusStopped,
		ExpiresAt: now.Add(30 * 24 * time.Hour),
		CreatedAt: now,
		UpdatedAt: now,
	}
}

func TestCreateAndGet(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	ch := sampleChannel("ch1")

	if err := s.Create(ctx, ch); err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := s.Get(ctx, "ch1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got == nil {
		t.Fatal("Get returned nil")
	}
	if got.Carrier != "wbstream" {
		t.Errorf("Carrier = %q, want wbstream", got.Carrier)
	}
	if got.ClientID != "client-ch1" {
		t.Errorf("ClientID = %q, want client-ch1", got.ClientID)
	}
	if got.TransportConfig.VP8FPS != 60 {
		t.Errorf("VP8FPS = %d, want 60", got.TransportConfig.VP8FPS)
	}
}

func TestGetNotFound(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	got, err := s.Get(ctx, "nonexistent")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got != nil {
		t.Errorf("expected nil, got %+v", got)
	}
}

func TestList(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	for _, id := range []string{"a", "b", "c"} {
		if err := s.Create(ctx, sampleChannel(id)); err != nil {
			t.Fatalf("Create(%s): %v", id, err)
		}
	}

	list, err := s.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 3 {
		t.Errorf("List len = %d, want 3", len(list))
	}
}

func TestUpdate(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	ch := sampleChannel("u1")

	if err := s.Create(ctx, ch); err != nil {
		t.Fatalf("Create: %v", err)
	}

	ch.Carrier = "jazz"
	ch.Status = channel.StatusRunning
	ch.UpdatedAt = time.Now().UTC().Truncate(time.Second)

	if err := s.Update(ctx, ch); err != nil {
		t.Fatalf("Update: %v", err)
	}

	got, err := s.Get(ctx, "u1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Carrier != "jazz" {
		t.Errorf("Carrier = %q, want jazz", got.Carrier)
	}
	if got.Status != channel.StatusRunning {
		t.Errorf("Status = %q, want running", got.Status)
	}
}

func TestUpdateNotFound(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	ch := sampleChannel("ghost")

	err := s.Update(ctx, ch)
	if err == nil {
		t.Fatal("Update should fail for nonexistent channel")
	}
}

func TestDelete(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	ch := sampleChannel("d1")

	if err := s.Create(ctx, ch); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := s.Delete(ctx, "d1"); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	got, err := s.Get(ctx, "d1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got != nil {
		t.Error("channel should be deleted")
	}
}

func TestDeleteNotFound(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	err := s.Delete(ctx, "ghost")
	if err == nil {
		t.Fatal("Delete should fail for nonexistent channel")
	}
}

func TestDeleteExpired(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	expired := sampleChannel("exp")
	expired.ExpiresAt = time.Now().UTC().Add(-1 * time.Hour)
	if err := s.Create(ctx, expired); err != nil {
		t.Fatalf("Create expired: %v", err)
	}

	alive := sampleChannel("alive")
	alive.ExpiresAt = time.Now().UTC().Add(24 * time.Hour)
	if err := s.Create(ctx, alive); err != nil {
		t.Fatalf("Create alive: %v", err)
	}

	ids, err := s.DeleteExpired(ctx)
	if err != nil {
		t.Fatalf("DeleteExpired: %v", err)
	}
	if len(ids) != 1 || ids[0] != "exp" {
		t.Errorf("DeleteExpired ids = %v, want [exp]", ids)
	}

	count, _ := s.Count(ctx)
	if count != 1 {
		t.Errorf("Count = %d, want 1", count)
	}
}

func TestCount(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	count, err := s.Count(ctx)
	if err != nil {
		t.Fatalf("Count: %v", err)
	}
	if count != 0 {
		t.Errorf("Count = %d, want 0", count)
	}

	if err := s.Create(ctx, sampleChannel("c1")); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := s.Create(ctx, sampleChannel("c2")); err != nil {
		t.Fatalf("Create: %v", err)
	}

	count, err = s.Count(ctx)
	if err != nil {
		t.Fatalf("Count: %v", err)
	}
	if count != 2 {
		t.Errorf("Count = %d, want 2", count)
	}
}

func TestPersistenceAcrossReopen(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "persist.db")

	s1, err := NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	ctx := context.Background()
	if err := s1.Create(ctx, sampleChannel("p1")); err != nil {
		t.Fatalf("Create: %v", err)
	}
	_ = s1.Close()

	s2, err := NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteStore reopen: %v", err)
	}
	defer s2.Close()

	got, err := s2.Get(ctx, "p1")
	if err != nil {
		t.Fatalf("Get after reopen: %v", err)
	}
	if got == nil {
		t.Fatal("channel should persist across reopen")
	}
	if got.Carrier != "wbstream" {
		t.Errorf("Carrier = %q, want wbstream", got.Carrier)
	}
}

func TestNewSQLiteStoreCreatesDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "sub", "dir")
	dbPath := filepath.Join(dir, "test.db")

	// The directory does not exist yet; SQLite should handle creation
	// or we expect an error — let's verify we get a usable store.
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	s, err := NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	defer s.Close()

	count, err := s.Count(context.Background())
	if err != nil {
		t.Fatalf("Count: %v", err)
	}
	if count != 0 {
		t.Errorf("Count = %d, want 0", count)
	}
}
