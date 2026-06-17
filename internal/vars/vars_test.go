// Copyright 2026 TechNative B.V. and the nivis authors
// SPDX-License-Identifier: Apache-2.0

package vars

import (
	"os"
	"path/filepath"
	"testing"
)

func writeJSON(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "vars.json")
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

// An explicit --var flag beats a var-file beats the environment (Terraform
// precedence: defaults < env < file < flag).
func TestPrecedenceFlagBeatsFileBeatsEnv(t *testing.T) {
	file := writeJSON(t, `{"region":"eu-central-1","only_file":"f"}`)
	env := []string{"NIVIS_VAR_region=eu-west-1", "NIVIS_VAR_only_env=e", "UNRELATED=x"}
	got, err := Resolve(env, []string{file}, []string{"region=us-east-1"})
	if err != nil {
		t.Fatal(err)
	}
	if got["region"] != "us-east-1" {
		t.Errorf("region: flag should win, got %v", got["region"])
	}
	if got["only_file"] != "f" {
		t.Errorf("only_file: %v", got["only_file"])
	}
	if got["only_env"] != "e" {
		t.Errorf("only_env: %v", got["only_env"])
	}
}

// With no flag for a name, the var-file value beats the environment.
func TestFileBeatsEnv(t *testing.T) {
	file := writeJSON(t, `{"region":"eu-central-1"}`)
	got, err := Resolve([]string{"NIVIS_VAR_region=eu-west-1"}, []string{file}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got["region"] != "eu-central-1" {
		t.Errorf("file should beat env, got %v", got["region"])
	}
}

// Later flags and later files override earlier ones.
func TestLaterWins(t *testing.T) {
	f1 := writeJSON(t, `{"y":"first"}`)
	f2 := writeJSON(t, `{"y":"second"}`)
	got, err := Resolve(nil, []string{f1, f2}, []string{"x=1", "x=2"})
	if err != nil {
		t.Fatal(err)
	}
	if got["x"] != "2" {
		t.Errorf("later --var should win, got %v", got["x"])
	}
	if got["y"] != "second" {
		t.Errorf("later --var-file should win, got %v", got["y"])
	}
}

// Nothing set -> nil map (so the ledger omits `vars`).
func TestEmptyIsNil(t *testing.T) {
	got, err := Resolve([]string{"UNRELATED=x"}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got != nil {
		t.Errorf("expected nil for no variables, got %#v", got)
	}
}

// An env var named exactly the prefix (empty name) is ignored.
func TestEmptyEnvNameIgnored(t *testing.T) {
	got, err := Resolve([]string{"NIVIS_VAR_=x"}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got != nil {
		t.Errorf("empty-name env var should be ignored, got %#v", got)
	}
}

func TestMalformedVarFlag(t *testing.T) {
	if _, err := Resolve(nil, nil, []string{"notanassignment"}); err == nil {
		t.Error("a --var without = must error")
	}
	if _, err := Resolve(nil, nil, []string{"=value"}); err == nil {
		t.Error("a --var with an empty name must error")
	}
}

func TestUnreadableVarFile(t *testing.T) {
	if _, err := Resolve(nil, []string{"/no/such/file.json"}, nil); err == nil {
		t.Error("a missing --var-file must error")
	}
}

func TestNonObjectVarFile(t *testing.T) {
	bad := writeJSON(t, `["a","b"]`) // valid JSON, not an object
	if _, err := Resolve(nil, []string{bad}, nil); err == nil {
		t.Error("a --var-file that is not a JSON object must error")
	}
}

// A var-file may carry non-string JSON values (numbers, bools) for permissively
// typed variables; they pass through.
func TestVarFileTypedValues(t *testing.T) {
	file := writeJSON(t, `{"count":3,"enabled":true}`)
	got, err := Resolve(nil, []string{file}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got["count"] != float64(3) { // JSON numbers decode to float64
		t.Errorf("count: %#v", got["count"])
	}
	if got["enabled"] != true {
		t.Errorf("enabled: %#v", got["enabled"])
	}
}
