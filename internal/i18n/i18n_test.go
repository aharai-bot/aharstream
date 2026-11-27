package i18n

import (
	"strings"
	"testing"
)

// Every language must define every key, with the same placeholders as English.
func TestTablesComplete(t *testing.T) {
	en := table["en"]
	for _, l := range Langs {
		tab, ok := table[l.Code]
		if !ok {
			t.Fatalf("language %s has no table", l.Code)
		}
		for k, v := range en {
			tv, ok := tab[k]
			if !ok || tv == "" {
				t.Errorf("%s: missing key %q", l.Code, k)
				continue
			}
			if strings.Count(v, "%d") != strings.Count(tv, "%d") || strings.Count(v, "%s") != strings.Count(tv, "%s") {
				t.Errorf("%s: placeholder mismatch in %q", l.Code, k)
			}
		}
		for k := range tab {
			if _, ok := en[k]; !ok {
				t.Errorf("%s: unknown key %q", l.Code, k)
			}
		}
	}
}

func TestFallback(t *testing.T) {
	if T("xx", "never") != "never" || T("hi", "nope") != "nope" {
		t.Fatal("fallback")
	}
}
