// Package setup is the first-run wizard. When the required settings are not
// configured, it starts a small web page where the owner fills them in. The
// page is protected by a one-time token that is printed in the server logs, so
// only the person who can read the logs can use it.
package setup

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"html/template"
	"net"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"
)

// EnvFile is where the wizard stores the settings.
const EnvFile = "fsb.env"

var required = []string{"API_ID", "API_HASH", "BOT_TOKEN", "LOG_CHANNEL"}

// LoadEnvFile exports KEY=VALUE lines of fsb.env that are not already set.
func LoadEnvFile(path string) {
	b, err := os.ReadFile(path)
	if err != nil {
		return
	}
	for _, line := range strings.Split(string(b), "\n") {
		k, v, ok := parseLine(line)
		if !ok {
			continue
		}
		if _, set := os.LookupEnv(k); !set {
			_ = os.Setenv(k, v)
		}
	}
}

func parseLine(line string) (key, val string, ok bool) {
	line = strings.TrimSpace(strings.TrimSuffix(line, "\r"))
	if line == "" || strings.HasPrefix(line, "#") {
		return "", "", false
	}
	line = strings.TrimPrefix(line, "export ")
	i := strings.Index(line, "=")
	if i <= 0 {
		return "", "", false
	}
	key = strings.TrimSpace(line[:i])
	val = strings.TrimSpace(line[i+1:])
	if len(val) >= 2 && (val[0] == '"' && val[len(val)-1] == '"' || val[0] == '\'' && val[len(val)-1] == '\'') {
		val = val[1 : len(val)-1]
	}
	return key, val, key != ""
}

// Missing returns the required settings that are not set in the environment.
func Missing() []string {
	var out []string
	for _, k := range required {
		if strings.TrimSpace(os.Getenv(k)) == "" {
			out = append(out, k)
		}
	}
	return out
}

var (
	reDigits  = regexp.MustCompile(`^\d{1,12}$`)
	reHash    = regexp.MustCompile(`^[0-9a-fA-F]{32}$`)
	reToken   = regexp.MustCompile(`^\d{5,}:[A-Za-z0-9_-]{30,}$`)
	reChannel = regexp.MustCompile(`^-?\d{5,20}$`)
	reAdmins  = regexp.MustCompile(`^\d{1,15}(\s*,\s*\d{1,15})*$`)
	reName    = regexp.MustCompile(`^[^"'\\$#\r\n` + "`" + `]{1,60}$`)
)

// Validate checks the submitted values and returns the cleaned settings.
func Validate(in map[string]string) (map[string]string, error) {
	out := map[string]string{}
	get := func(k string) string { return strings.TrimSpace(in[k]) }
	if v := get("API_ID"); !reDigits.MatchString(v) {
		return nil, errors.New("API_ID must be a number (from my.telegram.org)")
	} else {
		out["API_ID"] = v
	}
	if v := get("API_HASH"); !reHash.MatchString(v) {
		return nil, errors.New("API_HASH must be 32 letters/digits (from my.telegram.org)")
	} else {
		out["API_HASH"] = v
	}
	if v := get("BOT_TOKEN"); !reToken.MatchString(v) {
		return nil, errors.New("BOT_TOKEN looks wrong. It should look like 123456:ABC-DEF... (from @BotFather)")
	} else {
		out["BOT_TOKEN"] = v
	}
	if v := get("LOG_CHANNEL"); !reChannel.MatchString(v) {
		return nil, errors.New("LOG_CHANNEL must be the channel ID, for example -1001234567890")
	} else {
		out["LOG_CHANNEL"] = v
	}
	if v := strings.TrimRight(get("HOST"), "/"); v != "" {
		if !strings.HasPrefix(v, "http://") && !strings.HasPrefix(v, "https://") {
			return nil, errors.New("HOST must start with http:// or https://")
		}
		if strings.ContainsAny(v, " \"'\\$#\r\n`") {
			return nil, errors.New("HOST contains invalid characters")
		}
		out["HOST"] = v
	}
	if v := get("ADMINS"); v != "" {
		if !reAdmins.MatchString(v) {
			return nil, errors.New("ADMINS must be Telegram user IDs separated by commas")
		}
		out["ADMINS"] = strings.ReplaceAll(v, " ", "")
	}
	if v := get("BOT_NAME"); v != "" {
		if !reName.MatchString(v) {
			return nil, errors.New("BOT_NAME can have up to 60 characters and no quotes or special symbols")
		}
		out["BOT_NAME"] = v
	}
	return out, nil
}

// Merge writes values into the env file text, replacing existing keys and
// keeping every other line (comments included) untouched.
func Merge(existing string, vals map[string]string) string {
	done := map[string]bool{}
	var lines []string
	if existing != "" {
		for _, line := range strings.Split(strings.TrimRight(existing, "\n"), "\n") {
			k, _, ok := parseLine(line)
			if v, has := vals[k]; ok && has {
				lines = append(lines, formatLine(k, v))
				done[k] = true
				continue
			}
			lines = append(lines, strings.TrimSuffix(line, "\r"))
		}
	}
	for _, k := range []string{"API_ID", "API_HASH", "BOT_TOKEN", "LOG_CHANNEL", "HOST", "ADMINS", "BOT_NAME"} {
		if v, has := vals[k]; has && !done[k] {
			lines = append(lines, formatLine(k, v))
		}
	}
	return strings.Join(lines, "\n") + "\n"
}

func formatLine(k, v string) string {
	if strings.ContainsAny(v, " ") {
		return k + `="` + v + `"`
	}
	return k + "=" + v
}

func save(vals map[string]string) error {
	old, _ := os.ReadFile(EnvFile)
	if err := os.WriteFile(EnvFile, []byte(Merge(string(old), vals)), 0o600); err != nil {
		return err
	}
	for k, v := range vals {
		_ = os.Setenv(k, v)
	}
	return nil
}

// RunIfNeeded loads fsb.env and, when required settings are still missing,
// serves the wizard until the settings are saved. logf receives progress lines.
func RunIfNeeded(logf func(format string, args ...any)) error {
	LoadEnvFile(EnvFile)
	missing := Missing()
	if len(missing) == 0 {
		return nil
	}
	port := strings.TrimSpace(os.Getenv("PORT"))
	if port == "" {
		port = "8080"
	}
	tokenBytes := make([]byte, 6)
	if _, err := rand.Read(tokenBytes); err != nil {
		return err
	}
	token := hex.EncodeToString(tokenBytes)
	done := make(chan struct{})

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		render(w, view{Token: token, Missing: missing, NeedToken: !tokenOK(r, token)}, http.StatusOK)
	})
	mux.HandleFunc("/save", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, 16<<10)
		if err := r.ParseForm(); err != nil {
			http.Error(w, "bad form", http.StatusBadRequest)
			return
		}
		if subtle.ConstantTimeCompare([]byte(r.PostForm.Get("token")), []byte(token)) != 1 {
			render(w, view{Token: token, Missing: missing, NeedToken: true, Error: "Wrong setup token. Copy it from the server logs."}, http.StatusUnauthorized)
			return
		}
		in := map[string]string{}
		for k := range r.PostForm {
			in[k] = r.PostForm.Get(k)
		}
		vals, err := Validate(in)
		if err != nil {
			render(w, view{Token: token, Missing: missing, Error: err.Error(), Form: in}, http.StatusBadRequest)
			return
		}
		if err := save(vals); err != nil {
			render(w, view{Token: token, Missing: missing, Error: "Could not save " + EnvFile + ": " + err.Error(), Form: in}, http.StatusInternalServerError)
			return
		}
		render(w, view{Done: true}, http.StatusOK)
		select {
		case <-done:
		default:
			close(done)
		}
	})
	srv := &http.Server{Addr: ":" + port, Handler: mux, ReadHeaderTimeout: 10 * time.Second}

	ln, err := net.Listen("tcp", srv.Addr)
	if err != nil {
		return fmt.Errorf("cannot start the setup wizard on port %s: %w", port, err)
	}
	errc := make(chan error, 1)
	go func() { errc <- srv.Serve(ln) }()

	logf("First run: required settings are missing (%s).", strings.Join(missing, ", "))
	logf("Setup wizard: open http://<your-server>:%s/?token=%s in a browser.", port, token)
	logf("The wizard stops as soon as the settings are saved. Alternatively set them as environment variables and restart.")

	select {
	case <-done:
		time.Sleep(1500 * time.Millisecond) // let the success page reach the browser
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(ctx)
		if left := Missing(); len(left) > 0 {
			return fmt.Errorf("still missing: %s", strings.Join(left, ", "))
		}
		logf("Settings saved to %s. Starting the bot...", EnvFile)
		return nil
	case err := <-errc:
		return err
	}
}

func tokenOK(r *http.Request, token string) bool {
	return subtle.ConstantTimeCompare([]byte(r.URL.Query().Get("token")), []byte(token)) == 1
}

type view struct {
	Token     string
	Missing   []string
	NeedToken bool
	Error     string
	Done      bool
	Form      map[string]string
}

func render(w http.ResponseWriter, v view, status int) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.WriteHeader(status)
	_ = page.Execute(w, v)
}

var page = template.Must(template.New("setup").Parse(`<!doctype html>
<html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<meta name="robots" content="noindex"><title>Setup</title>
<style>
:root{--bg:#0b0b0f;--card:#15151c;--card2:#1b1b25;--text:#f3f3f7;--muted:#9a9ab0;--line:rgba(255,255,255,.1);--grad:linear-gradient(135deg,#ff0033,#a100ff)}
*{box-sizing:border-box}body{margin:0;background:var(--bg);color:var(--text);font-family:Roboto,"Segoe UI",system-ui,sans-serif;line-height:1.5;display:grid;place-items:center;min-height:100vh;padding:20px}
body::before{content:"";position:fixed;inset:0;z-index:-1;background:radial-gradient(700px 400px at 10% -5%,rgba(255,0,51,.18),transparent 60%),radial-gradient(700px 400px at 95% 0,rgba(161,0,255,.18),transparent 60%)}
.card{width:100%;max-width:520px;background:var(--card);border:1px solid var(--line);border-radius:20px;padding:26px}
h1{margin:0 0 4px;font-size:1.5rem}.muted{color:var(--muted);font-size:.92rem}
label{display:block;margin:14px 0 6px;font-size:.78rem;color:var(--muted);text-transform:uppercase;letter-spacing:.8px}
input{width:100%;height:44px;padding:0 14px;border-radius:12px;border:1px solid var(--line);background:var(--card2);color:var(--text);font:inherit;outline:none}
input:focus{border-color:rgba(255,0,51,.6)}
small{display:block;color:var(--muted);margin-top:5px;font-size:.8rem}
button{margin-top:20px;width:100%;height:46px;border:0;border-radius:12px;background:var(--grad);color:#fff;font:inherit;font-weight:700;cursor:pointer}
.err{background:rgba(255,90,110,.12);border:1px solid rgba(255,90,110,.35);color:#ff7a8a;border-radius:12px;padding:10px 12px;margin:14px 0 0;font-size:.9rem}
.ok{text-align:center}.ok h1{font-size:1.6rem}code{background:var(--card2);padding:2px 7px;border-radius:6px}
a{color:#8ab4ff}
</style></head><body><div class="card">
{{if .Done}}
<div class="ok"><h1>✅ Saved</h1><p class="muted">Your settings were saved and the bot is starting now.<br>Reload this address in a few seconds to see the home page.</p>
<p class="muted">If this host loses its files on restart (many free hosts do), also set these values as environment variables.</p></div>
{{else}}
<h1>Welcome 👋</h1>
<div class="muted">Let's set up your bot. This takes one minute.</div>
{{if .Error}}<div class="err">{{.Error}}</div>{{end}}
{{if .NeedToken}}
<form method="get" action="/"><label for="t">Setup token</label><input id="t" name="token" autocomplete="off" required>
<small>Find it in the server logs: the line that starts with "Setup wizard".</small><button type="submit">Continue</button></form>
{{else}}
<form method="post" action="/save"><input type="hidden" name="token" value="{{.Token}}">
<label for="API_ID">API ID</label><input id="API_ID" name="API_ID" inputmode="numeric" required value="{{index .Form "API_ID"}}"><small>From <a href="https://my.telegram.org" target="_blank" rel="noopener">my.telegram.org</a> → API development tools</small>
<label for="API_HASH">API Hash</label><input id="API_HASH" name="API_HASH" autocomplete="off" required value="{{index .Form "API_HASH"}}">
<label for="BOT_TOKEN">Bot token</label><input id="BOT_TOKEN" name="BOT_TOKEN" autocomplete="off" required value="{{index .Form "BOT_TOKEN"}}"><small>From <b>@BotFather</b></small>
<label for="LOG_CHANNEL">Log channel ID</label><input id="LOG_CHANNEL" name="LOG_CHANNEL" required placeholder="-1001234567890" value="{{index .Form "LOG_CHANNEL"}}"><small>A private channel where the bot is an admin. Files are stored there.</small>
<label for="ADMINS">Owner Telegram ID <span style="text-transform:none">(optional)</span></label><input id="ADMINS" name="ADMINS" inputmode="numeric" value="{{index .Form "ADMINS"}}"><small>Enables admin commands. Several IDs: separate with commas.</small>
<label for="HOST">Public URL <span style="text-transform:none">(optional)</span></label><input id="HOST" name="HOST" placeholder="https://files.example.com" value="{{index .Form "HOST"}}"><small>Use https to enable the in-Telegram app.</small>
<label for="BOT_NAME">Bot name <span style="text-transform:none">(optional)</span></label><input id="BOT_NAME" name="BOT_NAME" placeholder="Ahar File Streamer" value="{{index .Form "BOT_NAME"}}">
<button type="submit">Save and start</button></form>
{{end}}
{{end}}
</div></body></html>`))
