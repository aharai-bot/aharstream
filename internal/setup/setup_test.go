package setup

import (
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"
)

func good() map[string]string {
	return map[string]string{
		"API_ID": "123456", "API_HASH": "0123456789abcdef0123456789abcdef",
		"BOT_TOKEN": "123456789:AAEvrINCsduhjkjhahadvHRdk7oF46KZnc", "LOG_CHANNEL": "-1001234567890",
		"HOST": "https://files.example.com/", "ADMINS": "1, 2", "BOT_NAME": "My Bot",
	}
}

func TestValidate(t *testing.T) {
	out, err := Validate(good())
	if err != nil || out["HOST"] != "https://files.example.com" || out["ADMINS"] != "1,2" {
		t.Fatalf("%v %v", out, err)
	}
	for k, v := range map[string]string{"API_ID": "abc", "API_HASH": "zz", "BOT_TOKEN": "nope", "LOG_CHANNEL": "12", "HOST": "ftp://x", "ADMINS": "a", "BOT_NAME": "a\"b"} {
		in := good()
		in[k] = v
		if _, err := Validate(in); err == nil {
			t.Errorf("%s=%q accepted", k, v)
		}
	}
}

func TestMerge(t *testing.T) {
	old := "# keep me\nAPI_ID=\nPORT=8080\nBOT_NAME=Old\n"
	out := Merge(old, map[string]string{"API_ID": "5", "BOT_NAME": "New Name", "HOST": "https://h"})
	for _, want := range []string{"# keep me", "API_ID=5", "PORT=8080", `BOT_NAME="New Name"`, "HOST=https://h"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in\n%s", want, out)
		}
	}
	if strings.Contains(out, "BOT_NAME=Old") {
		t.Error("old value kept")
	}
	k, v, ok := parseLine(`BOT_NAME="New Name"`)
	if !ok || k != "BOT_NAME" || v != "New Name" {
		t.Error("roundtrip", k, v)
	}
}

func TestWizardFlow(t *testing.T) {
	dir := t.TempDir()
	wd, _ := os.Getwd()
	_ = os.Chdir(dir)
	defer os.Chdir(wd)
	for _, k := range required {
		os.Unsetenv(k)
	}
	l, _ := net.Listen("tcp", "127.0.0.1:0")
	port := strings.Split(l.Addr().String(), ":")[1]
	l.Close()
	os.Setenv("PORT", port)

	logs := make(chan string, 10)
	res := make(chan error, 1)
	go func() {
		res <- RunIfNeeded(func(f string, a ...any) {
			logs <- strings.TrimSpace(strings.ReplaceAll(f, "%s", "%s")) + "|" + strings.Join(strFmt(a), ",")
		})
	}()
	var token string
	for i := 0; i < 20 && token == ""; i++ {
		select {
		case l := <-logs:
			if strings.HasPrefix(l, "Setup wizard") {
				parts := strings.Split(l, "|")
				a := strings.Split(parts[1], ",")
				token = a[1]
			}
		case <-time.After(300 * time.Millisecond):
		}
	}
	if token == "" {
		t.Fatal("no token logged")
	}
	base := "http://127.0.0.1:" + port
	// without token: asks for it
	r, err := http.Get(base + "/")
	if err != nil {
		t.Fatal(err)
	}
	b, _ := io.ReadAll(r.Body)
	if !strings.Contains(string(b), "Setup token") {
		t.Fatal("token prompt missing")
	}
	// wrong token on save
	form := url.Values{}
	for k, v := range good() {
		form.Set(k, v)
	}
	form.Set("token", "wrong")
	r, _ = http.PostForm(base+"/save", form)
	if r.StatusCode != 401 {
		t.Fatalf("status %d", r.StatusCode)
	}
	form.Set("token", token)
	r, _ = http.PostForm(base+"/save", form)
	b, _ = io.ReadAll(r.Body)
	if r.StatusCode != 200 || !strings.Contains(string(b), "Saved") {
		t.Fatalf("save failed %d %s", r.StatusCode, b)
	}
	select {
	case err := <-res:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("wizard did not finish")
	}
	data, _ := os.ReadFile(EnvFile)
	if !strings.Contains(string(data), "API_ID=123456") || os.Getenv("BOT_TOKEN") == "" {
		t.Fatalf("not saved: %s", data)
	}
	if st, _ := os.Stat(EnvFile); st.Mode().Perm() != 0o600 {
		t.Errorf("perm %v", st.Mode().Perm())
	}
}

func strFmt(a []any) []string {
	var out []string
	for _, x := range a {
		out = append(out, toS(x))
	}
	return out
}

func toS(x any) string {
	if s, ok := x.(string); ok {
		return s
	}
	return ""
}
