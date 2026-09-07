// Copyright 2026 TechNative B.V. and the nivis authors
// SPDX-License-Identifier: Apache-2.0

package main

// CLI tests for the --backend override and `state migrate`. The configuration's
// phase-0 graph is supplied through the graphFn seam, so these run without a Nix
// evaluator; the remote backend is the hermetic in-repo fake S3.

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nivis-project/nivis/internal/fakes3"
	"github.com/nivis-project/nivis/internal/ir"
	"github.com/nivis-project/nivis/internal/state"
)

// withGraph makes the configuration appear to declare the given backend for the
// duration of the test.
func withGraph(t *testing.T, backend map[string]interface{}) {
	t.Helper()
	old := graphFn
	graphFn = func(context.Context) (*ir.Graph, error) {
		return &ir.Graph{Backend: backend}, nil
	}
	t.Cleanup(func() { graphFn = old })
}

// withState points the package-global --state path at a temp file, seeded with the
// given resource ids, and returns the path.
func withState(t *testing.T, ids ...string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "nivis.state.json")
	st, err := state.Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	for _, id := range ids {
		if err := st.Set(state.ResourceState{ID: id, Type: "alpha_token", Attrs: map[string]interface{}{"id": id}}); err != nil {
			t.Fatalf("Set: %v", err)
		}
	}
	old := statePath
	statePath = path
	t.Cleanup(func() { statePath = old })
	return path
}

// withOverride sets --backend for the duration of the test.
func withOverride(t *testing.T, value string) {
	t.Helper()
	old := backendOverride
	backendOverride = value
	t.Cleanup(func() { backendOverride = old })
}

// s3At builds an s3 backend block pointing at a fake S3 server.
func s3At(srv *fakes3.Server, bucket, key string) map[string]interface{} {
	return map[string]interface{}{
		"type": "s3", "bucket": bucket, "key": key, "region": "us-east-1", "endpoint": srv.URL(),
	}
}

// fakeCreds sets the dummy credentials the SDK insists on; the fake ignores them.
func fakeCreds(t *testing.T) {
	t.Helper()
	t.Setenv("AWS_ACCESS_KEY_ID", "test")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "test")
	t.Setenv("AWS_REGION", "us-east-1")
}

// --- the --backend override (group 5) ----------------------------------------

// --backend=local uses the local state file even though the configuration
// declares an s3 backend, makes no request to that backend (the fake knows no
// buckets, so any request would fail), and says so in its output.
func TestBackendOverrideUsesLocalState(t *testing.T) {
	fakeCreds(t)
	srv := fakes3.NewWithBuckets() // no bucket exists: any S3 call fails
	defer srv.Close()
	withGraph(t, s3At(srv, "would-fail", "prod/app.json"))
	withState(t, "alpha.alpha_token.app")
	withOverride(t, "local")

	cmd := stateCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"list"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("state list with --backend=local: %v", err)
	}
	got := out.String()
	if !strings.Contains(got, "alpha.alpha_token.app") {
		t.Errorf("the local state was not read:\n%s", got)
	}
	if !strings.Contains(got, "--backend=local overrides the s3 backend") {
		t.Errorf("the override should be announced when it contradicts a declared backend:\n%s", got)
	}
}

// With no declared backend there is nothing to contradict, so the override is
// silent (it is the default store anyway).
func TestBackendOverrideSilentWithoutDeclaredBackend(t *testing.T) {
	withGraph(t, nil)
	withState(t, "alpha.alpha_token.app")
	withOverride(t, "local")

	cmd := stateCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"list"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("state list: %v", err)
	}
	if strings.Contains(out.String(), "--backend=local overrides") {
		t.Errorf("no override notice is warranted without a declared backend:\n%s", out.String())
	}
}

// An unknown --backend value is refused, naming what is accepted.
func TestBackendOverrideUnknownValueRejected(t *testing.T) {
	withGraph(t, nil)
	withState(t)
	withOverride(t, "postgres")

	cmd := stateCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"list"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected an unknown --backend value to be refused")
	}
	if !strings.Contains(err.Error(), `"local"`) || !strings.Contains(err.Error(), "postgres") {
		t.Errorf("error should name the bad value and the accepted one: %v", err)
	}
}

// --- state migrate (group 6) -------------------------------------------------

// runMigrate drives `state migrate` with the given args.
func runMigrate(t *testing.T, args ...string) (string, error) {
	t.Helper()
	cmd := migrateCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return out.String(), err
}

// Local state moves into the declared backend: the command reports both sides and
// the count, the remote holds the document, and the local file is gone.
func TestMigrateToRemoteMovesAndReports(t *testing.T) {
	fakeCreds(t)
	srv := fakes3.NewWithBuckets("nivis-state")
	defer srv.Close()
	withGraph(t, s3At(srv, "nivis-state", "prod/app.json"))
	path := withState(t, "alpha.alpha_token.app", "beta.beta_record.app")

	out, err := runMigrate(t, "--to-remote")
	if err != nil {
		t.Fatalf("state migrate --to-remote: %v\n%s", err, out)
	}
	for _, want := range []string{
		"from the local state file", path, "to   s3://nivis-state/prod/app.json",
		"Moved 2 resource(s)", "removed the source document",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output lacks %q:\n%s", want, out)
		}
	}
	if !srv.Has("nivis-state", "prod/app.json") {
		t.Error("the state object was not written to the remote backend")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("the local state file survived the migration (err = %v)", err)
	}
}

// The reverse direction brings the document back and deletes the remote object.
func TestMigrateFromRemote(t *testing.T) {
	fakeCreds(t)
	srv := fakes3.NewWithBuckets("nivis-state")
	defer srv.Close()
	backend := s3At(srv, "nivis-state", "prod/app.json")
	withGraph(t, backend)
	path := withState(t) // local starts empty

	remote, err := state.OpenBackend(backend, path)
	if err != nil {
		t.Fatalf("OpenBackend: %v", err)
	}
	if err := remote.Set(state.ResourceState{ID: "alpha.alpha_token.app", Type: "alpha_token", Attrs: map[string]interface{}{"id": "t1"}}); err != nil {
		t.Fatalf("seed remote: %v", err)
	}

	out, err := runMigrate(t, "--from-remote")
	if err != nil {
		t.Fatalf("state migrate --from-remote: %v\n%s", err, out)
	}
	if !strings.Contains(out, "Moved 1 resource(s)") {
		t.Errorf("output lacks the moved count:\n%s", out)
	}
	local, err := state.Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if _, ok, _ := local.Get("alpha.alpha_token.app"); !ok {
		t.Error("the document did not arrive in the local state file")
	}
	if srv.Has("nivis-state", "prod/app.json") {
		t.Error("the remote state object was not removed")
	}
}

// No direction, and both directions, are refused with a message naming the flags.
func TestMigrateDirectionIsRequiredAndExclusive(t *testing.T) {
	fakeCreds(t)
	srv := fakes3.NewWithBuckets("nivis-state")
	defer srv.Close()
	withGraph(t, s3At(srv, "nivis-state", "prod/app.json"))
	path := withState(t, "alpha.alpha_token.app")

	_, err := runMigrate(t)
	if err == nil {
		t.Fatal("expected a refusal with no direction")
	}
	if !strings.Contains(err.Error(), "--to-remote") || !strings.Contains(err.Error(), "--from-remote") {
		t.Errorf("error should name both direction flags: %v", err)
	}

	_, err = runMigrate(t, "--to-remote", "--from-remote")
	if err == nil {
		t.Fatal("expected a refusal with both directions")
	}
	if !strings.Contains(err.Error(), "exactly one direction") {
		t.Errorf("unexpected error: %v", err)
	}

	// Nothing was touched by either refusal.
	if _, err := os.Stat(path); err != nil {
		t.Errorf("the state file was disturbed by a refused migration: %v", err)
	}
	if srv.Has("nivis-state", "prod/app.json") {
		t.Error("a refused migration wrote to the remote backend")
	}
}

// A configuration with no remote backend has nothing to migrate to or from.
func TestMigrateWithoutDeclaredBackendIsRefused(t *testing.T) {
	withGraph(t, nil)
	path := withState(t, "alpha.alpha_token.app")

	_, err := runMigrate(t, "--to-remote")
	if err == nil {
		t.Fatal("expected a refusal without a declared backend")
	}
	if !strings.Contains(err.Error(), "declares no remote backend") {
		t.Errorf("unexpected error: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("the state file was disturbed: %v", err)
	}
}

// A destination holding different resources is refused with both counts, and
// --force then overwrites it.
func TestMigrateConflictingDestination(t *testing.T) {
	fakeCreds(t)
	srv := fakes3.NewWithBuckets("nivis-state")
	defer srv.Close()
	backend := s3At(srv, "nivis-state", "prod/app.json")
	withGraph(t, backend)
	path := withState(t, "a", "b")

	remote, err := state.OpenBackend(backend, path)
	if err != nil {
		t.Fatalf("OpenBackend: %v", err)
	}
	if err := remote.Set(state.ResourceState{ID: "z", Type: "alpha_token", Attrs: map[string]interface{}{"id": "z"}}); err != nil {
		t.Fatalf("seed remote: %v", err)
	}

	_, err = runMigrate(t, "--to-remote")
	if err == nil {
		t.Fatal("expected a refusal for a conflicting destination")
	}
	for _, want := range []string{"already holds 1 resource(s)", "source holds 2", "--force"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error lacks %q: %v", want, err)
		}
	}
	if _, statErr := os.Stat(path); statErr != nil {
		t.Errorf("the source was removed despite the refusal: %v", statErr)
	}

	out, err := runMigrate(t, "--to-remote", "--force")
	if err != nil {
		t.Fatalf("forced migrate: %v\n%s", err, out)
	}
	if !strings.Contains(out, "Moved 2 resource(s)") {
		t.Errorf("forced migrate output:\n%s", out)
	}
	if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
		t.Error("the source survived a forced migration")
	}
}

// An interrupted migration is completed rather than reported as a conflict.
func TestMigrateResumeIsReported(t *testing.T) {
	fakeCreds(t)
	srv := fakes3.NewWithBuckets("nivis-state")
	defer srv.Close()
	backend := s3At(srv, "nivis-state", "prod/app.json")
	withGraph(t, backend)
	path := withState(t, "a")

	local, err := state.Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	snap, err := local.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	remote, err := state.OpenBackend(backend, path)
	if err != nil {
		t.Fatalf("OpenBackend: %v", err)
	}
	if err := remote.Restore(snap); err != nil {
		t.Fatalf("seed destination: %v", err)
	}

	out, err := runMigrate(t, "--to-remote")
	if err != nil {
		t.Fatalf("resuming migrate: %v\n%s", err, out)
	}
	if !strings.Contains(out, "interrupted migration") {
		t.Errorf("output should explain the resume:\n%s", out)
	}
	if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
		t.Error("the resumed migration did not remove the source")
	}
}

// A failure before the source is removed says the source is unchanged, and it is.
func TestMigrateFailureSaysSourceIsIntact(t *testing.T) {
	fakeCreds(t)
	srv := fakes3.NewWithBuckets() // the bucket does not exist
	defer srv.Close()
	withGraph(t, s3At(srv, "not-yet", "prod/app.json"))
	path := withState(t, "a", "b")

	_, err := runMigrate(t, "--to-remote")
	if err == nil {
		t.Fatal("expected the migration to fail against a missing bucket")
	}
	if !strings.Contains(err.Error(), "unchanged") {
		t.Errorf("error should state the source is unchanged: %v", err)
	}
	// The missing-bucket message names the location and the bootstrap recipe.
	for _, want := range []string{"not-yet", "us-east-1", "nivis apply --backend=local", "nivis state migrate --to-remote"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error lacks %q: %v", want, err)
		}
	}
	st, err2 := state.Open(path)
	if err2 != nil {
		t.Fatalf("Open: %v", err2)
	}
	list, err2 := st.List()
	if err2 != nil || len(list) != 2 {
		t.Errorf("the source document changed: (%v, %v)", list, err2)
	}
}

// --backend has no meaning for a command that addresses both backends itself.
func TestMigrateRejectsBackendOverride(t *testing.T) {
	fakeCreds(t)
	srv := fakes3.NewWithBuckets("nivis-state")
	defer srv.Close()
	withGraph(t, s3At(srv, "nivis-state", "prod/app.json"))
	withState(t, "a")
	withOverride(t, "local")

	_, err := runMigrate(t, "--to-remote")
	if err == nil {
		t.Fatal("expected --backend to be refused for state migrate")
	}
	if !strings.Contains(err.Error(), "no meaning for `state migrate`") {
		t.Errorf("unexpected error: %v", err)
	}
}

// `state migrate` is listed among the state subcommands.
func TestStateHelpListsMigrate(t *testing.T) {
	cmd := stateCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--help"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("state --help: %v", err)
	}
	if !strings.Contains(out.String(), "migrate") {
		t.Errorf("state --help does not list migrate:\n%s", out.String())
	}
}
