// Package auth holds the small security helpers shared by the web pages, the
// app API and the Telegram sign-in flows: signed tokens, one-time login links,
// Telegram login validation, password hashing and tiny in-memory limiters.
package auth

import (
	"crypto/hmac"
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

var secret []byte

// Init derives the signing key from the bot token (or SESSION_SECRET if set).
func Init(botToken, override string) {
	src := override
	if src == "" {
		src = botToken
	}
	h := sha256.Sum256([]byte("ahar-file-streamer:" + src))
	secret = h[:]
}

func sign(msg string) string {
	m := hmac.New(sha256.New, secret)
	m.Write([]byte(msg))
	return base64.RawURLEncoding.EncodeToString(m.Sum(nil))
}

// MakeToken returns "payload.expiry.signature" valid for ttl.
func MakeToken(kind, payload string, ttl time.Duration) string {
	exp := strconv.FormatInt(time.Now().Add(ttl).Unix(), 10)
	p := base64.RawURLEncoding.EncodeToString([]byte(payload))
	return p + "." + exp + "." + sign(kind+"|"+p+"|"+exp)
}

// VerifyToken checks a token made by MakeToken and returns its payload.
func VerifyToken(kind, tok string) (string, bool) {
	parts := strings.Split(tok, ".")
	if len(parts) != 3 {
		return "", false
	}
	exp, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil || time.Now().Unix() > exp {
		return "", false
	}
	want := sign(kind + "|" + parts[0] + "|" + parts[1])
	if subtle.ConstantTimeCompare([]byte(want), []byte(parts[2])) != 1 {
		return "", false
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return "", false
	}
	return string(raw), true
}

// ---------- sessions & link access ----------

const SessionTTL = 30 * 24 * time.Hour

func SessionToken(uid int64) string {
	return MakeToken("s", strconv.FormatInt(uid, 10), SessionTTL)
}

func ParseSession(tok string) (int64, bool) {
	p, ok := VerifyToken("s", tok)
	if !ok {
		return 0, false
	}
	id, err := strconv.ParseInt(p, 10, 64)
	return id, err == nil
}

const UnlockTTL = 6 * time.Hour

// UnlockToken proves a visitor passed the password / view check for one link.
func UnlockToken(messageID int, hash string) string {
	return MakeToken("u", fmt.Sprintf("%d:%s", messageID, hash), UnlockTTL)
}

func CheckUnlock(tok string, messageID int, hash string) bool {
	if tok == "" {
		return false
	}
	p, ok := VerifyToken("u", tok)
	return ok && p == fmt.Sprintf("%d:%s", messageID, hash)
}

// ---------- one-time login links (/login in the bot) ----------

type loginEntry struct {
	uid int64
	exp time.Time
}

var (
	loginMu sync.Mutex
	logins  = map[string]loginEntry{}
)

func NewLoginToken(uid int64) string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	t := hex.EncodeToString(b)
	loginMu.Lock()
	defer loginMu.Unlock()
	now := time.Now()
	for k, v := range logins {
		if now.After(v.exp) {
			delete(logins, k)
		}
	}
	logins[t] = loginEntry{uid: uid, exp: now.Add(5 * time.Minute)}
	return t
}

// ConsumeLoginToken returns the user id once; the token is then burned.
func ConsumeLoginToken(t string) (int64, bool) {
	loginMu.Lock()
	defer loginMu.Unlock()
	e, ok := logins[t]
	if !ok {
		return 0, false
	}
	delete(logins, t)
	if time.Now().After(e.exp) {
		return 0, false
	}
	return e.uid, true
}

// ---------- Telegram sign-in validation ----------

type TGUser struct {
	ID        int64  `json:"id"`
	FirstName string `json:"first_name"`
	Username  string `json:"username"`
}

func hmacSHA256(key []byte, msg string) []byte {
	m := hmac.New(sha256.New, key)
	m.Write([]byte(msg))
	return m.Sum(nil)
}

func dataCheckString(v url.Values) string {
	keys := make([]string, 0, len(v))
	for k := range v {
		if k != "hash" {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	lines := make([]string, 0, len(keys))
	for _, k := range keys {
		lines = append(lines, k+"="+v.Get(k))
	}
	return strings.Join(lines, "\n")
}

func freshAuthDate(v url.Values, maxAge time.Duration) error {
	ts, err := strconv.ParseInt(v.Get("auth_date"), 10, 64)
	if err != nil {
		return errors.New("bad auth_date")
	}
	if time.Since(time.Unix(ts, 0)) > maxAge {
		return errors.New("login data is too old")
	}
	return nil
}

// ValidateWebApp verifies Mini App initData and returns the Telegram user.
func ValidateWebApp(initData, botToken string, maxAge time.Duration) (TGUser, error) {
	v, err := url.ParseQuery(initData)
	if err != nil || v.Get("hash") == "" {
		return TGUser{}, errors.New("invalid init data")
	}
	key := hmacSHA256([]byte("WebAppData"), botToken)
	want := hex.EncodeToString(hmacSHA256(key, dataCheckString(v)))
	if !hmac.Equal([]byte(want), []byte(strings.ToLower(v.Get("hash")))) {
		return TGUser{}, errors.New("signature mismatch")
	}
	if err := freshAuthDate(v, maxAge); err != nil {
		return TGUser{}, err
	}
	var u TGUser
	if err := json.Unmarshal([]byte(v.Get("user")), &u); err != nil || u.ID == 0 {
		return TGUser{}, errors.New("no user in init data")
	}
	return u, nil
}

// ValidateLoginWidget verifies the browser "Log in with Telegram" callback.
func ValidateLoginWidget(v url.Values, botToken string, maxAge time.Duration) (TGUser, error) {
	if v.Get("hash") == "" {
		return TGUser{}, errors.New("missing hash")
	}
	sum := sha256.Sum256([]byte(botToken))
	want := hex.EncodeToString(hmacSHA256(sum[:], dataCheckString(v)))
	if !hmac.Equal([]byte(want), []byte(strings.ToLower(v.Get("hash")))) {
		return TGUser{}, errors.New("signature mismatch")
	}
	if err := freshAuthDate(v, maxAge); err != nil {
		return TGUser{}, err
	}
	id, err := strconv.ParseInt(v.Get("id"), 10, 64)
	if err != nil || id == 0 {
		return TGUser{}, errors.New("bad id")
	}
	return TGUser{ID: id, FirstName: v.Get("first_name"), Username: v.Get("username")}, nil
}

// ---------- passwords ----------

func HashPassword(pw string) (string, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	k, err := pbkdf2.Key(sha256.New, pw, salt, 20000, 32)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(salt) + "$" + hex.EncodeToString(k), nil
}

func CheckPassword(stored, pw string) bool {
	parts := strings.SplitN(stored, "$", 2)
	if len(parts) != 2 {
		return false
	}
	salt, err := hex.DecodeString(parts[0])
	if err != nil {
		return false
	}
	k, err := pbkdf2.Key(sha256.New, pw, salt, 20000, 32)
	if err != nil {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(hex.EncodeToString(k)), []byte(parts[1])) == 1
}

// ---------- tiny in-memory helpers ----------

// Limiter allows at most max events per key inside a sliding window.
type Limiter struct {
	mu sync.Mutex
	m  map[string][]time.Time
}

func NewLimiter() *Limiter { return &Limiter{m: map[string][]time.Time{}} }

func (l *Limiter) Allow(key string, max int, window time.Duration) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	cut := now.Add(-window)
	keep := l.m[key][:0]
	for _, t := range l.m[key] {
		if t.After(cut) {
			keep = append(keep, t)
		}
	}
	if len(keep) >= max {
		l.m[key] = keep
		return false
	}
	l.m[key] = append(keep, now)
	if len(l.m) > 5000 { // keep memory bounded
		for k, v := range l.m {
			if len(v) == 0 || v[len(v)-1].Before(cut) {
				delete(l.m, k)
			}
		}
	}
	return true
}

// TTLSet remembers keys for a limited time (used to count unique visitors).
type TTLSet struct {
	mu sync.Mutex
	m  map[string]time.Time
}

func NewTTLSet() *TTLSet { return &TTLSet{m: map[string]time.Time{}} }

func (s *TTLSet) Has(key string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	t, ok := s.m[key]
	return ok && time.Now().Before(t)
}

func (s *TTLSet) Add(key string, ttl time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	if len(s.m) > 10000 {
		for k, t := range s.m {
			if now.After(t) {
				delete(s.m, k)
			}
		}
	}
	s.m[key] = now.Add(ttl)
}

// ---------- request classification ----------

// ClassifyUA buckets a User-Agent: Bot, Player (VLC/MX/etc.), Tablet, Mobile, Desktop.
func ClassifyUA(ua string) string {
	l := strings.ToLower(ua)
	switch {
	case l == "":
		return "Player"
	case strings.Contains(l, "bot"), strings.Contains(l, "crawler"), strings.Contains(l, "spider"),
		strings.Contains(l, "preview"), strings.Contains(l, "facebookexternalhit"), strings.Contains(l, "whatsapp"),
		strings.Contains(l, "slurp"), strings.Contains(l, "headlesschrome"):
		return "Bot"
	case strings.Contains(l, "vlc"), strings.Contains(l, "mxplayer"), strings.Contains(l, "lavf"),
		strings.Contains(l, "exoplayer"), strings.Contains(l, "stagefright"), strings.Contains(l, "kodi"),
		strings.Contains(l, "mpv"), strings.Contains(l, "curl"), strings.Contains(l, "wget"), strings.Contains(l, "okhttp"):
		return "Player"
	case strings.Contains(l, "ipad"), strings.Contains(l, "tablet"):
		return "Tablet"
	case strings.Contains(l, "mobi"), strings.Contains(l, "android"), strings.Contains(l, "iphone"):
		return "Mobile"
	}
	return "Desktop"
}

// Country reads the country code that CDNs / hosts add to requests. It returns
// "" when the host does not provide one (no GeoIP database is bundled).
func Country(h http.Header) string {
	for _, k := range []string{"CF-IPCountry", "CloudFront-Viewer-Country", "X-Vercel-IP-Country", "X-AppEngine-Country", "Fly-Region-Country", "X-Country-Code"} {
		if v := strings.ToUpper(strings.TrimSpace(h.Get(k))); len(v) == 2 && v != "XX" && v != "T1" {
			return v
		}
	}
	return ""
}
