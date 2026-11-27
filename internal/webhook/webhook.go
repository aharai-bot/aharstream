// Package webhook delivers signed JSON events to a user's HTTPS endpoint.
package webhook

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

var sem = make(chan struct{}, 8) // at most 8 deliveries at once (low-power servers)

var client = &http.Client{
	Timeout: 6 * time.Second,
	Transport: &http.Transport{
		DialContext:         safeDial,
		DisableKeepAlives:   true,
		TLSHandshakeTimeout: 5 * time.Second,
	},
	CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
}

var cgnat = mustCIDR("100.64.0.0/10")

func mustCIDR(s string) *net.IPNet {
	_, n, err := net.ParseCIDR(s)
	if err != nil {
		panic(err)
	}
	return n
}

func blockedIP(ip net.IP) bool {
	return ip.IsLoopback() || ip.IsPrivate() || ip.IsUnspecified() || ip.IsMulticast() ||
		ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || cgnat.Contains(ip)
}

// safeDial refuses private / internal addresses so webhooks cannot be used to
// probe the server's own network (SSRF).
func safeDial(ctx context.Context, network, addr string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, err
	}
	ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil {
		return nil, err
	}
	d := &net.Dialer{Timeout: 5 * time.Second}
	last := errors.New("no usable address")
	for _, ia := range ips {
		if blockedIP(ia.IP) {
			last = errors.New("address not allowed")
			continue
		}
		c, err := d.DialContext(ctx, network, net.JoinHostPort(ia.IP.String(), port))
		if err == nil {
			return c, nil
		}
		last = err
	}
	return nil, last
}

// ValidateURL checks a webhook URL: https only, no credentials, no raw private IPs.
func ValidateURL(raw string) error {
	if len(raw) > 500 {
		return errors.New("URL is too long")
	}
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Host == "" {
		return errors.New("invalid URL")
	}
	if u.Scheme != "https" {
		return errors.New("the URL must start with https://")
	}
	if u.User != nil {
		return errors.New("credentials in the URL are not allowed")
	}
	if ip := net.ParseIP(u.Hostname()); ip != nil && blockedIP(ip) {
		return errors.New("private addresses are not allowed")
	}
	if strings.EqualFold(u.Hostname(), "localhost") {
		return errors.New("localhost is not allowed")
	}
	return nil
}

type envelope struct {
	Event string `json:"event"`
	Time  int64  `json:"time"`
	Data  any    `json:"data"`
}

// Fire sends an event in the background. It never blocks the caller.
func Fire(targetURL, secret, event string, data any) {
	if targetURL == "" {
		return
	}
	select {
	case sem <- struct{}{}:
	default:
		return // too many deliveries in flight; drop instead of piling up
	}
	go func() {
		defer func() { <-sem }()
		_ = deliver(targetURL, secret, event, data)
	}()
}

// Send delivers synchronously (used by /webhook test) and returns the error.
func Send(targetURL, secret, event string, data any) error {
	return deliver(targetURL, secret, event, data)
}

func deliver(targetURL, secret, event string, data any) error {
	body, err := json.Marshal(envelope{Event: event, Time: time.Now().Unix(), Data: data})
	if err != nil {
		return err
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	sig := "sha256=" + hex.EncodeToString(mac.Sum(nil))

	var lastErr error
	for attempt := 0; attempt < 2; attempt++ {
		if attempt > 0 {
			time.Sleep(2 * time.Second)
		}
		req, err := http.NewRequest(http.MethodPost, targetURL, bytes.NewReader(body))
		if err != nil {
			return err
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("User-Agent", "Ahar-File-Streamer-Webhook/1.0")
		req.Header.Set("X-Fsb-Event", event)
		req.Header.Set("X-Fsb-Signature", sig)
		resp, err := client.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		resp.Body.Close()
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return nil
		}
		lastErr = errors.New("endpoint answered HTTP " + resp.Status)
	}
	return lastErr
}
