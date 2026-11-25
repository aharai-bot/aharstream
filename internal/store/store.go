// Package store is a tiny dependency-free JSON persistence layer used by
// Ahar File Streamer for users, bans, admins, link records and statistics.
package store

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

type User struct {
	ID        int64  `json:"id"`
	Username  string `json:"username"`
	FirstName string `json:"first_name"`
	JoinedAt  int64  `json:"joined_at"`
	LastSeen  int64  `json:"last_seen"`
	Files     int    `json:"files"`
	Banned    bool   `json:"banned"`
	BanReason string `json:"ban_reason,omitempty"`
	// ExpiryHours is the user's default link lifetime (0 = never expires).
	ExpiryHours int `json:"expiry_hours"`
}

// FileRecord is one generated link.
type FileRecord struct {
	MessageID int    `json:"message_id"` // message id inside LOG_CHANNEL
	OwnerID   int64  `json:"owner_id"`
	Name      string `json:"name"`
	Size      int64  `json:"size"`
	Mime      string `json:"mime"`
	Hash      string `json:"hash"`
	CreatedAt int64  `json:"created_at"`
	ExpiresAt int64  `json:"expires_at"` // 0 = never
	Revoked   bool   `json:"revoked"`
	Downloads int64  `json:"downloads"`
	Bytes     int64  `json:"bytes"`
}

type Stats struct {
	TotalLinks     int64 `json:"total_links"`
	TotalDownloads int64 `json:"total_downloads"`
	TotalBytes     int64 `json:"total_bytes"`
}

type data struct {
	Users       map[int64]*User     `json:"users"`
	Files       map[int]*FileRecord `json:"files"`
	Admins      []int64             `json:"admins"`
	Stats       Stats               `json:"stats"`
	Maintenance bool                `json:"maintenance"`
	Settings    map[string]string   `json:"settings"`
}

type Store struct {
	mu    sync.RWMutex
	path  string
	d     data
	dirty bool
	stop  chan struct{}
}

var Default *Store

// Init loads (or creates) the database file and starts the background flusher.
func Init(path string) error {
	s := &Store{path: path, stop: make(chan struct{})}
	s.d = data{
		Users:    map[int64]*User{},
		Files:    map[int]*FileRecord{},
		Settings: map[string]string{},
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if b, err := os.ReadFile(path); err == nil {
		if err := json.Unmarshal(b, &s.d); err != nil {
			return err
		}
		if s.d.Users == nil {
			s.d.Users = map[int64]*User{}
		}
		if s.d.Files == nil {
			s.d.Files = map[int]*FileRecord{}
		}
		if s.d.Settings == nil {
			s.d.Settings = map[string]string{}
		}
	}
	Default = s
	go s.flusher()
	return nil
}

func (s *Store) flusher() {
	t := time.NewTicker(15 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-t.C:
			_ = s.Flush()
		case <-s.stop:
			return
		}
	}
}

// Flush writes the database to disk if anything changed.
func (s *Store) Flush() error {
	s.mu.Lock()
	if !s.dirty {
		s.mu.Unlock()
		return nil
	}
	b, err := json.MarshalIndent(&s.d, "", " ")
	s.dirty = false
	s.mu.Unlock()
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

// ---------- users ----------

// Touch registers the user if new and updates last-seen. Returns the user and
// whether it was newly created.
func (s *Store) Touch(id int64, username, first string) (*User, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	u, ok := s.d.Users[id]
	now := time.Now().Unix()
	if !ok {
		u = &User{ID: id, JoinedAt: now}
		s.d.Users[id] = u
	}
	u.Username, u.FirstName, u.LastSeen = username, first, now
	s.dirty = true
	cp := *u
	return &cp, !ok
}

func (s *Store) GetUser(id int64) (User, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	u, ok := s.d.Users[id]
	if !ok {
		return User{}, false
	}
	return *u, true
}

func (s *Store) SetBan(id int64, banned bool, reason string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	u, ok := s.d.Users[id]
	if !ok {
		u = &User{ID: id, JoinedAt: time.Now().Unix()}
		s.d.Users[id] = u
	}
	u.Banned = banned
	if banned {
		u.BanReason = reason
	} else {
		u.BanReason = ""
	}
	s.dirty = true
}

func (s *Store) IsBanned(id int64) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	u, ok := s.d.Users[id]
	return ok && u.Banned
}

func (s *Store) SetExpiry(id int64, hours int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if u, ok := s.d.Users[id]; ok {
		u.ExpiryHours = hours
		s.dirty = true
	}
}

func (s *Store) AllUserIDs() []int64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	ids := make([]int64, 0, len(s.d.Users))
	for id, u := range s.d.Users {
		if !u.Banned {
			ids = append(ids, id)
		}
	}
	return ids
}

func (s *Store) BannedUsers() []User {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []User
	for _, u := range s.d.Users {
		if u.Banned {
			out = append(out, *u)
		}
	}
	return out
}

// RecentUsers returns the n most recently active users.
func (s *Store) RecentUsers(n int) []User {
	s.mu.RLock()
	out := make([]User, 0, len(s.d.Users))
	for _, u := range s.d.Users {
		out = append(out, *u)
	}
	s.mu.RUnlock()
	sort.Slice(out, func(i, j int) bool { return out[i].LastSeen > out[j].LastSeen })
	if len(out) > n {
		out = out[:n]
	}
	return out
}

func (s *Store) UserCount() (total, banned int) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, u := range s.d.Users {
		total++
		if u.Banned {
			banned++
		}
	}
	return
}

// ActiveSince counts users seen after the given unix time.
func (s *Store) ActiveSince(ts int64) int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	n := 0
	for _, u := range s.d.Users {
		if u.LastSeen >= ts {
			n++
		}
	}
	return n
}

// ---------- admins ----------

func (s *Store) Admins() []int64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]int64(nil), s.d.Admins...)
}

func (s *Store) IsStoredAdmin(id int64) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, a := range s.d.Admins {
		if a == id {
			return true
		}
	}
	return false
}

func (s *Store) AddAdmin(id int64) {
	if s.IsStoredAdmin(id) {
		return
	}
	s.mu.Lock()
	s.d.Admins = append(s.d.Admins, id)
	s.dirty = true
	s.mu.Unlock()
}

func (s *Store) RemoveAdmin(id int64) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, a := range s.d.Admins {
		if a == id {
			s.d.Admins = append(s.d.Admins[:i], s.d.Admins[i+1:]...)
			s.dirty = true
			return true
		}
	}
	return false
}

// ---------- maintenance ----------

func (s *Store) Maintenance() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.d.Maintenance
}

func (s *Store) SetMaintenance(v bool) {
	s.mu.Lock()
	s.d.Maintenance = v
	s.dirty = true
	s.mu.Unlock()
}

// ---------- files ----------

func (s *Store) AddFile(f *FileRecord) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.d.Files[f.MessageID] = f
	s.d.Stats.TotalLinks++
	if u, ok := s.d.Users[f.OwnerID]; ok {
		u.Files++
	}
	s.dirty = true
}

func (s *Store) GetFile(messageID int) (FileRecord, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	f, ok := s.d.Files[messageID]
	if !ok {
		return FileRecord{}, false
	}
	return *f, true
}

func (s *Store) RevokeFile(messageID int, restore bool) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	f, ok := s.d.Files[messageID]
	if !ok {
		return false
	}
	f.Revoked = !restore
	s.dirty = true
	return true
}

func (s *Store) FilesOf(owner int64, n int) []FileRecord {
	s.mu.RLock()
	var out []FileRecord
	for _, f := range s.d.Files {
		if f.OwnerID == owner {
			out = append(out, *f)
		}
	}
	s.mu.RUnlock()
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt > out[j].CreatedAt })
	if n > 0 && len(out) > n {
		out = out[:n]
	}
	return out
}

// TopFiles returns the n most downloaded files.
func (s *Store) TopFiles(n int) []FileRecord {
	s.mu.RLock()
	out := make([]FileRecord, 0, len(s.d.Files))
	for _, f := range s.d.Files {
		out = append(out, *f)
	}
	s.mu.RUnlock()
	sort.Slice(out, func(i, j int) bool { return out[i].Downloads > out[j].Downloads })
	if len(out) > n {
		out = out[:n]
	}
	return out
}

// CountUserLinksSince counts links an owner created after ts (for rate limiting).
func (s *Store) CountUserLinksSince(owner int64, ts int64) int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	n := 0
	for _, f := range s.d.Files {
		if f.OwnerID == owner && f.CreatedAt >= ts {
			n++
		}
	}
	return n
}

// RecordDownload bumps counters. newDownload should be true only for the
// first request of a transfer (range start == 0) so seeking is not counted.
func (s *Store) RecordDownload(messageID int, bytes int64, newDownload bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.d.Stats.TotalBytes += bytes
	if newDownload {
		s.d.Stats.TotalDownloads++
	}
	if f, ok := s.d.Files[messageID]; ok {
		f.Bytes += bytes
		if newDownload {
			f.Downloads++
		}
	}
	s.dirty = true
}

func (s *Store) GetStats() (Stats, int) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.d.Stats, len(s.d.Files)
}

// PurgeExpired removes records that expired more than `grace` ago and returns the count.
func (s *Store) PurgeExpired(grace time.Duration) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	cut := time.Now().Add(-grace).Unix()
	n := 0
	for id, f := range s.d.Files {
		if f.ExpiresAt != 0 && f.ExpiresAt < cut {
			delete(s.d.Files, id)
			n++
		}
	}
	if n > 0 {
		s.dirty = true
	}
	return n
}
