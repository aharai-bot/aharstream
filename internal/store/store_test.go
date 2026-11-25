package store

import (
	"path/filepath"
	"testing"
	"time"
)

func TestStoreRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "db.json")
	if err := Init(path); err != nil {
		t.Fatal(err)
	}
	s := Default

	if _, isNew := s.Touch(42, "alice", "Alice"); !isNew {
		t.Fatal("expected new user")
	}
	if _, isNew := s.Touch(42, "alice", "Alice"); isNew {
		t.Fatal("user should not be new twice")
	}

	s.SetBan(42, true, "spam")
	if !s.IsBanned(42) {
		t.Fatal("expected banned")
	}
	s.SetBan(42, false, "")
	if s.IsBanned(42) {
		t.Fatal("expected unbanned")
	}

	s.AddFile(&FileRecord{MessageID: 7, OwnerID: 42, Name: "a.mp4", CreatedAt: time.Now().Unix()})
	s.RecordDownload(7, 1000, true)
	s.RecordDownload(7, 500, false) // a seek: bandwidth only
	f, ok := s.GetFile(7)
	if !ok || f.Downloads != 1 || f.Bytes != 1500 {
		t.Fatalf("bad counters: %+v", f)
	}
	if n := s.CountUserLinksSince(42, time.Now().Add(-time.Hour).Unix()); n != 1 {
		t.Fatalf("expected 1 recent link, got %d", n)
	}

	s.RevokeFile(7, false)
	if f, _ := s.GetFile(7); !f.Revoked {
		t.Fatal("expected revoked")
	}
	s.RevokeFile(7, true)
	if f, _ := s.GetFile(7); f.Revoked {
		t.Fatal("expected restored")
	}

	s.AddAdmin(99)
	s.AddAdmin(99)
	if len(s.Admins()) != 1 || !s.IsStoredAdmin(99) {
		t.Fatal("admin add should be idempotent")
	}
	if !s.RemoveAdmin(99) || s.IsStoredAdmin(99) {
		t.Fatal("admin remove failed")
	}

	s.AddFile(&FileRecord{MessageID: 8, OwnerID: 42, ExpiresAt: time.Now().Add(-time.Hour).Unix()})
	if n := s.PurgeExpired(0); n != 1 {
		t.Fatalf("expected 1 purged, got %d", n)
	}

	// persistence
	s.SetMaintenance(true)
	if err := s.Flush(); err != nil {
		t.Fatal(err)
	}
	if err := Init(path); err != nil {
		t.Fatal(err)
	}
	if !Default.Maintenance() {
		t.Fatal("maintenance flag not persisted")
	}
	if _, ok := Default.GetFile(7); !ok {
		t.Fatal("file record not persisted")
	}
	st, _ := Default.GetStats()
	if st.TotalDownloads != 1 || st.TotalBytes != 1500 {
		t.Fatalf("stats not persisted: %+v", st)
	}
}
