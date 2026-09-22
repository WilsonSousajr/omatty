package config_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/WilsonSousajr/omatty/internal/config"
)

// tomlKeys walks t's toml tags the way the decoder does: a struct field is a
// table, and its fields are "table.key". Written here, independently of the
// package, so the list the package offers is checked against the struct
// rather than against itself (#321).
func tomlKeys(t reflect.Type, prefix string) []string {
	var keys []string
	for i := range t.NumField() {
		f := t.Field(i)
		name := prefix + f.Tag.Get("toml")
		if f.Type.Kind() == reflect.Struct {
			keys = append(keys, tomlKeys(f.Type, name+".")...)
			continue
		}
		keys = append(keys, name)
	}
	return keys
}

// unknownKeyError is what Load says for a key it does not know.
func unknownKeyError(t *testing.T, body string) string {
	t.Helper()
	home := t.TempDir()
	_, err := config.Load(writeConfig(t, home, body), home)
	if err == nil {
		t.Fatalf("Load(%q) = nil error, want an unknown-key error", body)
	}
	return err.Error()
}

// Regression, issue #321: the offered list was written by hand and M9's
// [gate] table never made it in. Every key the struct can decode must be
// offered, whichever table it lives in.
func TestKnownKeys_ListsEveryTomlTagReachableFromConfig_issue321(t *testing.T) {
	msg := unknownKeyError(t, "bogus = 1\n")

	for _, key := range tomlKeys(reflect.TypeOf(config.Config{}), "") {
		if !strings.Contains(msg, key) {
			t.Errorf("unknown-key error does not offer %q:\n%s", key, msg)
		}
	}
}

// The issue's reproduction: a typo under [gate] must be answered with the
// spelling that would have worked.
func TestLoad_UnknownGateKeyNamesTheGateKeys_issue321(t *testing.T) {
	msg := unknownKeyError(t, "[gate]\nmax_paralel = 2\n")

	if !strings.Contains(msg, "gate.max_parallel") {
		t.Errorf("a [gate] typo was answered without gate.max_parallel:\n%s", msg)
	}
}
