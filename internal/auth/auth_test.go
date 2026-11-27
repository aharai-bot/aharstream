package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/url"
	"strconv"
	"testing"
	"time"
)

func TestTokens(t *testing.T) {
	Init("123:ABC", "")
	tok := UnlockToken(42, "abcdef")
	if !CheckUnlock(tok, 42, "abcdef") || CheckUnlock(tok, 43, "abcdef") || CheckUnlock(tok, 42, "x") || CheckUnlock("", 42, "abcdef") {
		t.Fatal("unlock token")
	}
	if id, ok := ParseSession(SessionToken(777)); !ok || id != 777 {
		t.Fatal("session")
	}
	if _, ok := ParseSession(tok); ok {
		t.Fatal("kind confusion")
	}
	lt := NewLoginToken(5)
	if id, ok := ConsumeLoginToken(lt); !ok || id != 5 {
		t.Fatal("login")
	}
	if _, ok := ConsumeLoginToken(lt); ok {
		t.Fatal("login reuse")
	}
}

func TestPassword(t *testing.T) {
	h, err := HashPassword("hunter22")
	if err != nil || !CheckPassword(h, "hunter22") || CheckPassword(h, "hunter23") {
		t.Fatal("password")
	}
}

func TestWebApp(t *testing.T) {
	token := "123:ABC"
	vals := url.Values{}
	vals.Set("auth_date", strconv.FormatInt(time.Now().Unix(), 10))
	vals.Set("query_id", "AAH")
	vals.Set("user", `{"id":99,"first_name":"A"}`)
	key := hmacSHA256([]byte("WebAppData"), token)
	vals.Set("hash", hex.EncodeToString(hmacSHA256(key, dataCheckString(vals))))
	u, err := ValidateWebApp(vals.Encode(), token, time.Hour)
	if err != nil || u.ID != 99 {
		t.Fatal(err)
	}
	vals.Set("user", `{"id":100}`)
	if _, err := ValidateWebApp(vals.Encode(), token, time.Hour); err == nil {
		t.Fatal("tamper accepted")
	}
	// login widget
	w := url.Values{}
	w.Set("id", "55")
	w.Set("first_name", "B")
	w.Set("auth_date", strconv.FormatInt(time.Now().Unix(), 10))
	sum := sha256.Sum256([]byte(token))
	m := hmac.New(sha256.New, sum[:])
	m.Write([]byte(dataCheckString(w)))
	w.Set("hash", hex.EncodeToString(m.Sum(nil)))
	if u, err := ValidateLoginWidget(w, token, time.Hour); err != nil || u.ID != 55 {
		t.Fatal(err)
	}
}

func TestLimiterAndUA(t *testing.T) {
	l := NewLimiter()
	for i := 0; i < 3; i++ {
		if !l.Allow("k", 3, time.Minute) {
			t.Fatal("early block")
		}
	}
	if l.Allow("k", 3, time.Minute) {
		t.Fatal("no block")
	}
	if ClassifyUA("TelegramBot (like TwitterBot)") != "Bot" || ClassifyUA("VLC/3.0") != "Player" || ClassifyUA("Mozilla/5.0 (iPhone) Mobile") != "Mobile" {
		t.Fatal("ua")
	}
}
