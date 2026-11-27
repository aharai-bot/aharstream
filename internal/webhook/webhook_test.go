package webhook

import (
	"net"
	"testing"
)

func TestValidateURL(t *testing.T) {
	ok := []string{"https://example.com/hook", "https://hooks.example.com:8443/a?b=c"}
	bad := []string{"http://example.com", "https://localhost/x", "https://127.0.0.1/x", "https://10.0.0.5/x", "https://169.254.169.254/latest", "https://user:pw@example.com", "ftp://example.com", "", "https://[::1]/x", "https://100.64.1.1/x"}
	for _, u := range ok {
		if err := ValidateURL(u); err != nil {
			t.Errorf("%s rejected: %v", u, err)
		}
	}
	for _, u := range bad {
		if err := ValidateURL(u); err == nil {
			t.Errorf("%q accepted", u)
		}
	}
	if !blockedIP(net.ParseIP("192.168.1.1")) || blockedIP(net.ParseIP("8.8.8.8")) {
		t.Error("blockedIP")
	}
}
