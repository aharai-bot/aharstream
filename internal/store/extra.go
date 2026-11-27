package store

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strings"
	"time"
)

// ---------- extra types ----------

// Collection groups several links into one shareable page.
type Collection struct {
	ID        string `json:"id"`
	OwnerID   int64  `json:"owner_id"`
	Title     string `json:"title"`
	Token     string `json:"token"` // secret part of the public URL
	Files     []int  `json:"files"` // FileRecord message ids, in display order
	CreatedAt int64  `json:"created_at"`
	Revoked   bool   `json:"revoked"`
}

// APIKey authenticates calls to /api/v1. Only a SHA-256 of the key is stored.
type APIKey struct {
	ID        string `json:"id"` // first 10 hex chars of Hash, safe to show
	Hash      string `json:"hash"`
	Prefix    string `json:"prefix"` // first characters of the plain key, for display
	OwnerID   int64  `json:"owner_id"`
	Name      string `json:"name"`
	CreatedAt int64  `json:"created_at"`
	LastUsed  int64  `json:"last_used"`
}

// Channel is a channel where the bot adds Watch/Download buttons to new posts.
type Channel struct {
	ID      int64 `json:"id"`
	OwnerID int64 `json:"owner_id"`
	AddedAt int64 `json:"added_at"`
}

func (d *data) initMaps() {
	if d.Collections == nil {
		d.Collections = map[string]*Collection{}
	}
	if d.APIKeys == nil {
		d.APIKeys = map[string]*APIKey{}
	}
	if d.Channels == nil {
		d.Channels = map[int64]*Channel{}
	}
}

// RandomToken returns n random bytes as lowercase hex (2n characters).
func RandomToken(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}

func hashKey(plain string) string {
	h := sha256.Sum256([]byte(plain))
	return hex.EncodeToString(h[:])
}

// ---------- generic file update ----------

// UpdateFile mutates a record under the store lock. Returns false if missing.
func (s *Store) UpdateFile(messageID int, fn func(*FileRecord)) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	f, ok := s.d.Files[messageID]
	if !ok {
		return false
	}
	fn(f)
	s.dirty = true
	return true
}

// DeleteFile removes a record entirely (and drops it from collections).
func (s *Store) DeleteFile(messageID int) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.d.Files[messageID]; !ok {
		return false
	}
	delete(s.d.Files, messageID)
	for _, c := range s.d.Collections {
		c.Files = removeInt(c.Files, messageID)
	}
	s.dirty = true
	return true
}

func removeInt(in []int, v int) []int {
	out := in[:0]
	for _, x := range in {
		if x != v {
			out = append(out, x)
		}
	}
	return out
}

// AddView registers a unique visit. It returns false (and counts nothing) when
// the link has a view limit that is already used up.
func (s *Store) AddView(messageID int, country, device string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	f, ok := s.d.Files[messageID]
	if !ok {
		return true
	}
	if f.MaxViews > 0 && f.Views >= f.MaxViews {
		return false
	}
	f.Views++
	f.LastViewed = time.Now().Unix()
	if country != "" {
		if f.Countries == nil {
			f.Countries = map[string]int64{}
		}
		f.Countries[country]++
	}
	if device != "" {
		if f.Devices == nil {
			f.Devices = map[string]int64{}
		}
		f.Devices[device]++
	}
	s.dirty = true
	return true
}

// ---------- listing / search ----------

// QueryFiles lists an owner's links, newest first. q matches the name
// (case-insensitive); folder filters when not empty ("-" = no folder).
func (s *Store) QueryFiles(owner int64, q, folder string, limit int) []FileRecord {
	q = strings.ToLower(strings.TrimSpace(q))
	s.mu.RLock()
	var out []FileRecord
	for _, f := range s.d.Files {
		if f.OwnerID != owner {
			continue
		}
		if q != "" && !strings.Contains(strings.ToLower(f.Name), q) {
			continue
		}
		if folder == "-" && f.Folder != "" || folder != "" && folder != "-" && f.Folder != folder {
			continue
		}
		out = append(out, f.clone())
	}
	s.mu.RUnlock()
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt > out[j].CreatedAt })
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out
}

// Summary is the "storage" overview of one user.
type Summary struct {
	Files       int      `json:"files"`
	Active      int      `json:"active"`
	Expired     int      `json:"expired"`
	Revoked     int      `json:"revoked"`
	Protected   int      `json:"protected"`
	Bytes       int64    `json:"bytes"`        // total size of all links
	ActiveBytes int64    `json:"active_bytes"` // size of active links
	Downloads   int64    `json:"downloads"`
	BytesServed int64    `json:"bytes_served"`
	Views       int64    `json:"views"`
	Folders     []string `json:"folders"`
}

func (s *Store) SummaryOf(owner int64) Summary {
	now := time.Now().Unix()
	folders := map[string]bool{}
	var sum Summary
	s.mu.RLock()
	for _, f := range s.d.Files {
		if f.OwnerID != owner {
			continue
		}
		sum.Files++
		sum.Bytes += f.Size
		sum.Downloads += f.Downloads
		sum.BytesServed += f.Bytes
		sum.Views += int64(f.Views)
		if f.PassHash != "" {
			sum.Protected++
		}
		if f.Folder != "" {
			folders[f.Folder] = true
		}
		switch {
		case f.Revoked:
			sum.Revoked++
		case f.ExpiresAt != 0 && now > f.ExpiresAt:
			sum.Expired++
		default:
			sum.Active++
			sum.ActiveBytes += f.Size
		}
	}
	s.mu.RUnlock()
	for k := range folders {
		sum.Folders = append(sum.Folders, k)
	}
	sort.Strings(sum.Folders)
	return sum
}

// ---------- user settings ----------

func (s *Store) SetLang(id int64, lang string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if u, ok := s.d.Users[id]; ok {
		u.Lang = lang
		s.dirty = true
	}
}

func (s *Store) SetWebhook(id int64, url, secret string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if u, ok := s.d.Users[id]; ok {
		u.WebhookURL, u.WebhookSecret = url, secret
		s.dirty = true
	}
}

// ---------- collections ----------

func (s *Store) CreateCollection(owner int64, title string, files []int) *Collection {
	s.mu.Lock()
	defer s.mu.Unlock()
	c := &Collection{
		ID:        RandomToken(4),
		OwnerID:   owner,
		Title:     title,
		Token:     RandomToken(8),
		CreatedAt: time.Now().Unix(),
	}
	for _, id := range files {
		if f, ok := s.d.Files[id]; ok && f.OwnerID == owner {
			c.Files = append(c.Files, id)
		}
	}
	s.d.Collections[c.ID] = c
	s.dirty = true
	cp := *c
	cp.Files = append([]int(nil), c.Files...)
	return &cp
}

func (s *Store) GetCollection(id string) (Collection, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	c, ok := s.d.Collections[id]
	if !ok {
		return Collection{}, false
	}
	cp := *c
	cp.Files = append([]int(nil), c.Files...)
	return cp, true
}

func (s *Store) CollectionsOf(owner int64) []Collection {
	s.mu.RLock()
	var out []Collection
	for _, c := range s.d.Collections {
		if c.OwnerID == owner {
			cp := *c
			cp.Files = append([]int(nil), c.Files...)
			out = append(out, cp)
		}
	}
	s.mu.RUnlock()
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt > out[j].CreatedAt })
	return out
}

// UpdateCollection changes title / files / revoked. Only the owner may do so.
func (s *Store) UpdateCollection(owner int64, id string, fn func(*Collection, map[int]*FileRecord)) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	c, ok := s.d.Collections[id]
	if !ok || c.OwnerID != owner {
		return false
	}
	fn(c, s.d.Files)
	s.dirty = true
	return true
}

func (s *Store) DeleteCollection(owner int64, id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	c, ok := s.d.Collections[id]
	if !ok || c.OwnerID != owner {
		return false
	}
	delete(s.d.Collections, id)
	s.dirty = true
	return true
}

// ---------- API keys ----------

// CreateAPIKey makes a new key and returns the plain text (shown only once).
func (s *Store) CreateAPIKey(owner int64, name string) (string, APIKey) {
	plain := "fsb_" + RandomToken(20)
	k := &APIKey{
		Hash:      hashKey(plain),
		Prefix:    plain[:8],
		OwnerID:   owner,
		Name:      name,
		CreatedAt: time.Now().Unix(),
	}
	k.ID = k.Hash[:10]
	s.mu.Lock()
	s.d.APIKeys[k.Hash] = k
	s.dirty = true
	s.mu.Unlock()
	return plain, *k
}

// LookupAPIKey resolves a plain key to its record.
func (s *Store) LookupAPIKey(plain string) (APIKey, bool) {
	h := hashKey(plain)
	s.mu.Lock()
	defer s.mu.Unlock()
	k, ok := s.d.APIKeys[h]
	if !ok {
		return APIKey{}, false
	}
	now := time.Now().Unix()
	if now-k.LastUsed > 60 { // avoid marking the db dirty on every request
		k.LastUsed = now
		s.dirty = true
	}
	return *k, true
}

func (s *Store) APIKeysOf(owner int64) []APIKey {
	s.mu.RLock()
	var out []APIKey
	for _, k := range s.d.APIKeys {
		if k.OwnerID == owner {
			out = append(out, *k)
		}
	}
	s.mu.RUnlock()
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt > out[j].CreatedAt })
	return out
}

func (s *Store) DeleteAPIKey(owner int64, id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	for h, k := range s.d.APIKeys {
		if k.OwnerID == owner && k.ID == id {
			delete(s.d.APIKeys, h)
			s.dirty = true
			return true
		}
	}
	return false
}

// ---------- channels ----------

func (s *Store) SetChannel(id, owner int64, enabled bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if enabled {
		s.d.Channels[id] = &Channel{ID: id, OwnerID: owner, AddedAt: time.Now().Unix()}
	} else {
		delete(s.d.Channels, id)
	}
	s.dirty = true
}

func (s *Store) GetChannel(id int64) (Channel, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	c, ok := s.d.Channels[id]
	if !ok {
		return Channel{}, false
	}
	return *c, true
}

func (s *Store) Channels() []Channel {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Channel, 0, len(s.d.Channels))
	for _, c := range s.d.Channels {
		out = append(out, *c)
	}
	return out
}
