package store

import (
	"path/filepath"
	"testing"
)

func newTest(t *testing.T) *Store {
	t.Helper()
	if err := Init(filepath.Join(t.TempDir(), "db.json")); err != nil {
		t.Fatal(err)
	}
	return Default
}

func TestViewsAndLimits(t *testing.T) {
	s := newTest(t)
	s.AddFile(&FileRecord{MessageID: 1, OwnerID: 7, Name: "a.mp4", Size: 10, Hash: "h", MaxViews: 2})
	if !s.AddView(1, "IN", "Mobile") || !s.AddView(1, "IN", "Desktop") {
		t.Fatal("views under the limit must count")
	}
	if s.AddView(1, "US", "Desktop") {
		t.Fatal("third view must be refused")
	}
	f, _ := s.GetFile(1)
	if f.Views != 2 || f.Countries["IN"] != 2 || f.Devices["Mobile"] != 1 {
		t.Fatalf("%+v", f)
	}
	// copies must not alias the stored maps
	f.Countries["IN"] = 99
	g, _ := s.GetFile(1)
	if g.Countries["IN"] != 2 {
		t.Fatal("clone shares maps")
	}
	s.UpdateFile(1, func(r *FileRecord) { r.MaxViews = 0 })
	if !s.AddView(1, "", "") {
		t.Fatal("unlimited link refused")
	}
}

func TestQuerySummaryCollections(t *testing.T) {
	s := newTest(t)
	s.AddFile(&FileRecord{MessageID: 1, OwnerID: 7, Name: "Holiday.mp4", Size: 100, Folder: "trips"})
	s.AddFile(&FileRecord{MessageID: 2, OwnerID: 7, Name: "notes.pdf", Size: 50, PassHash: "x"})
	s.AddFile(&FileRecord{MessageID: 3, OwnerID: 8, Name: "other.zip", Size: 5})
	if n := len(s.QueryFiles(7, "holi", "", 0)); n != 1 {
		t.Fatalf("search: %d", n)
	}
	if n := len(s.QueryFiles(7, "", "-", 0)); n != 1 {
		t.Fatalf("no-folder filter: %d", n)
	}
	sum := s.SummaryOf(7)
	if sum.Files != 2 || sum.Bytes != 150 || sum.Protected != 1 || len(sum.Folders) != 1 {
		t.Fatalf("%+v", sum)
	}
	c := s.CreateCollection(7, "Mine", []int{1, 2, 3}) // 3 belongs to someone else
	if len(c.Files) != 2 {
		t.Fatalf("collection kept foreign file: %v", c.Files)
	}
	if s.DeleteCollection(8, c.ID) {
		t.Fatal("another user deleted the collection")
	}
	s.DeleteFile(1)
	got, _ := s.GetCollection(c.ID)
	if len(got.Files) != 1 {
		t.Fatalf("deleted file still in collection: %v", got.Files)
	}
}

func TestAPIKeysAndChannels(t *testing.T) {
	s := newTest(t)
	plain, k := s.CreateAPIKey(7, "site")
	if _, ok := s.LookupAPIKey(plain); !ok {
		t.Fatal("key not found")
	}
	if _, ok := s.LookupAPIKey(plain + "x"); ok {
		t.Fatal("wrong key accepted")
	}
	if s.DeleteAPIKey(8, k.ID) {
		t.Fatal("foreign delete")
	}
	if !s.DeleteAPIKey(7, k.ID) {
		t.Fatal("own delete failed")
	}
	if _, ok := s.LookupAPIKey(plain); ok {
		t.Fatal("deleted key still works")
	}
	s.SetChannel(55, 7, true)
	if c, ok := s.GetChannel(55); !ok || c.OwnerID != 7 {
		t.Fatal("channel")
	}
	s.SetChannel(55, 0, false)
	if _, ok := s.GetChannel(55); ok {
		t.Fatal("channel not removed")
	}
}
