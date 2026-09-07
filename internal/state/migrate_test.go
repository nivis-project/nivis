// Copyright 2026 TechNative B.V. and the nivis authors
// SPDX-License-Identifier: Apache-2.0

package state_test

// Migration behaviour: the round trip both ways through real backends, the
// ordering and lock discipline, and the failure modes that must leave the source
// intact.

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nivis-project/nivis/internal/fakes3"
	"github.com/nivis-project/nivis/internal/state"
)

// --- a recording store, for asserting order and lock discipline --------------

// recorder is a Store + Locker + Remover that appends every call to a shared
// ordered log, so the migration's sequence is observable. Failures can be injected
// per operation.
type recorder struct {
	name string
	log  *[]string
	doc  []byte
	held bool
	// injected failures
	failLock    bool
	failRestore bool
	failRemove  bool
	// corruptRestore makes the read-back differ from what was written (a
	// destination that silently did not store what we sent).
	corruptRestore bool
	removed        bool
}

func newRecorder(name string, log *[]string, doc []byte) *recorder {
	return &recorder{name: name, log: log, doc: doc}
}

func (r *recorder) note(op string) { *r.log = append(*r.log, r.name+":"+op) }

func (r *recorder) Get(string) (state.ResourceState, bool, error) {
	r.note("get")
	return state.ResourceState{}, false, nil
}
func (r *recorder) Set(state.ResourceState) error { r.note("set"); return nil }
func (r *recorder) Delete(string) error           { r.note("delete"); return nil }
func (r *recorder) List() ([]state.ResourceState, error) {
	r.note("list")
	return nil, nil
}
func (r *recorder) Snapshot() ([]byte, error) {
	r.note("snapshot")
	return r.doc, nil
}
func (r *recorder) Restore(data []byte) error {
	r.note("restore")
	if r.failRestore {
		return errors.New("injected restore failure")
	}
	if r.corruptRestore {
		r.doc = []byte(`{"resources":{}}`)
		return nil
	}
	r.doc = data
	return nil
}
func (r *recorder) Lock(info state.LockInfo) (string, error) {
	r.note("lock")
	if r.failLock {
		return "", errors.New("state is locked by someone-else@host")
	}
	r.held = true
	return info.ID, nil
}
func (r *recorder) Unlock(string) error {
	r.note("unlock")
	r.held = false
	return nil
}
func (r *recorder) ForceUnlock() error { r.note("force-unlock"); r.held = false; return nil }
func (r *recorder) Remove() error {
	r.note("remove")
	if r.failRemove {
		return errors.New("injected remove failure")
	}
	r.removed = true
	r.doc = []byte(`{"resources":{}}`)
	return nil
}

// docBytes renders a canonical-looking document with the given ids.
func docBytes(ids ...string) []byte {
	var b strings.Builder
	b.WriteString(`{"resources":{`)
	for i, id := range ids {
		if i > 0 {
			b.WriteString(",")
		}
		fmt.Fprintf(&b, `%q:{"id":%q,"type":"alpha_token","attrs":{"id":%q}}`, id, id, id)
	}
	b.WriteString("}}")
	return []byte(b.String())
}

// --- ordering and lock discipline (4.2) --------------------------------------

// The migration locks the destination then the source, snapshots the source,
// reads the destination, writes, verifies by re-reading, removes the source, and
// releases both locks in reverse order.
func TestMigrateCallOrder(t *testing.T) {
	var log []string
	src := newRecorder("src", &log, docBytes("a"))
	dst := newRecorder("dst", &log, docBytes())

	res, err := state.Migrate(src, dst, state.MigrateOptions{})
	if err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	if res.Resources != 1 || res.Resumed {
		t.Errorf("result = %+v, want 1 resource and Resumed=false", res)
	}

	want := []string{
		"dst:lock", "src:lock", // destination first, then source
		"src:snapshot",             // read the source
		"dst:snapshot",             // guard reads the destination
		"dst:restore",              // write
		"dst:snapshot",             // verify by re-reading
		"src:remove",               // only now is the source redundant
		"src:unlock", "dst:unlock", // released in reverse order
	}
	if strings.Join(log, ",") != strings.Join(want, ",") {
		t.Errorf("call order:\n got %v\nwant %v", log, want)
	}
	if src.held || dst.held {
		t.Error("a lock was left held after a successful migration")
	}
}

// Every failure path still releases both locks.
func TestMigrateReleasesLocksOnFailure(t *testing.T) {
	var log []string
	src := newRecorder("src", &log, docBytes("a"))
	dst := newRecorder("dst", &log, docBytes())
	dst.failRestore = true

	if _, err := state.Migrate(src, dst, state.MigrateOptions{}); err == nil {
		t.Fatal("expected the injected restore failure to surface")
	}
	if src.held || dst.held {
		t.Errorf("locks left held after a failed migration (src=%v dst=%v)", src.held, dst.held)
	}
	if src.removed {
		t.Error("the source was removed despite the write failing")
	}
	joined := strings.Join(log, ",")
	if !strings.Contains(joined, "src:unlock") || !strings.Contains(joined, "dst:unlock") {
		t.Errorf("both unlocks should appear in %v", log)
	}
}

// A held lock on either side stops the migration before it copies or removes
// anything, and reports the holder.
func TestMigrateBlockedByHeldLock(t *testing.T) {
	t.Run("destination held", func(t *testing.T) {
		var log []string
		src := newRecorder("src", &log, docBytes("a"))
		dst := newRecorder("dst", &log, docBytes())
		dst.failLock = true

		_, err := state.Migrate(src, dst, state.MigrateOptions{})
		if err == nil {
			t.Fatal("expected a lock failure")
		}
		if !strings.Contains(err.Error(), "someone-else@host") {
			t.Errorf("error should name the holder: %v", err)
		}
		if !strings.Contains(err.Error(), "unchanged") {
			t.Errorf("error should say the source is unchanged: %v", err)
		}
		for _, forbidden := range []string{"dst:restore", "src:remove", "src:snapshot"} {
			if strings.Contains(strings.Join(log, ","), forbidden) {
				t.Errorf("%s happened despite the destination being locked: %v", forbidden, log)
			}
		}
	})

	t.Run("source held", func(t *testing.T) {
		var log []string
		src := newRecorder("src", &log, docBytes("a"))
		dst := newRecorder("dst", &log, docBytes())
		src.failLock = true

		_, err := state.Migrate(src, dst, state.MigrateOptions{})
		if err == nil {
			t.Fatal("expected a lock failure")
		}
		if src.removed {
			t.Error("the source was removed despite failing to lock it")
		}
		// The destination lock taken first must have been released again.
		if dst.held {
			t.Error("the destination lock was left held after the source lock failed")
		}
		if !strings.Contains(strings.Join(log, ","), "dst:unlock") {
			t.Errorf("the destination lock should have been released: %v", log)
		}
	})
}

// The verification step is what earns the right to delete: a destination that does
// not read back as what we wrote aborts the migration with the source intact.
func TestMigrateVerificationFailureKeepsSource(t *testing.T) {
	var log []string
	src := newRecorder("src", &log, docBytes("a", "b"))
	dst := newRecorder("dst", &log, docBytes())
	dst.corruptRestore = true

	_, err := state.Migrate(src, dst, state.MigrateOptions{})
	if err == nil {
		t.Fatal("expected the verification to fail")
	}
	if !strings.Contains(err.Error(), "does not match the source") {
		t.Errorf("error should name the verification failure: %v", err)
	}
	if !strings.Contains(err.Error(), "unchanged") {
		t.Errorf("error should say the source is unchanged: %v", err)
	}
	if src.removed {
		t.Fatal("the source was removed even though verification failed")
	}
}

// A source that cannot remove its document is refused BEFORE anything is written:
// discovering it later would leave two live copies of the state of record.
func TestMigrateRefusesUnremovableSource(t *testing.T) {
	var log []string
	dst := newRecorder("dst", &log, docBytes())
	src := noRemove{newRecorder("src", &log, docBytes("a"))}

	_, err := state.Migrate(src, dst, state.MigrateOptions{})
	if err == nil {
		t.Fatal("expected a refusal for a source that cannot be removed")
	}
	if !strings.Contains(err.Error(), "does not support removing") {
		t.Errorf("unexpected error: %v", err)
	}
	if strings.Contains(strings.Join(log, ","), "dst:restore") {
		t.Errorf("the destination was written before the source was known removable: %v", log)
	}
}

// noRemove is a Store that deliberately does NOT implement Remover: it delegates
// the Store methods explicitly rather than embedding the recorder, so no Remove
// method comes along with it.
type noRemove struct{ r *recorder }

func (n noRemove) Get(id string) (state.ResourceState, bool, error) { return n.r.Get(id) }
func (n noRemove) Set(rs state.ResourceState) error                 { return n.r.Set(rs) }
func (n noRemove) Delete(id string) error                           { return n.r.Delete(id) }
func (n noRemove) List() ([]state.ResourceState, error)             { return n.r.List() }
func (n noRemove) Snapshot() ([]byte, error)                        { return n.r.Snapshot() }
func (n noRemove) Restore(data []byte) error                        { return n.r.Restore(data) }

// Removing the source after a verified copy is the only failure that leaves two
// copies, so its message says exactly that.
func TestMigrateSourceRemovalFailureIsExplicit(t *testing.T) {
	var log []string
	src := newRecorder("src", &log, docBytes("a"))
	dst := newRecorder("dst", &log, docBytes())
	src.failRemove = true

	_, err := state.Migrate(src, dst, state.MigrateOptions{})
	if err == nil {
		t.Fatal("expected the injected removal failure to surface")
	}
	if !strings.Contains(err.Error(), "copied and verified") {
		t.Errorf("error should say the copy succeeded: %v", err)
	}
	if !strings.Contains(err.Error(), "resume") {
		t.Errorf("error should say a re-run resumes: %v", err)
	}
}

// --- the real backends, both directions (4.3) --------------------------------

// setupLocal returns a local store seeded with the given resource ids.
func setupLocal(t *testing.T, path string, ids ...string) state.Store {
	t.Helper()
	st, err := state.Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	for _, id := range ids {
		if err := st.Set(state.ResourceState{ID: id, Type: "alpha_token", Attrs: map[string]interface{}{"id": id}}); err != nil {
			t.Fatalf("Set %s: %v", id, err)
		}
	}
	return st
}

// ids lists the resource ids a store holds.
func ids(t *testing.T, st state.Store) []string {
	t.Helper()
	list, err := st.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	out := make([]string, 0, len(list))
	for _, rs := range list {
		out = append(out, rs.ID)
	}
	return out
}

// local -> s3 -> local: the document survives both hops, and each source document
// is gone afterwards.
func TestMigrateRoundTripLocalToS3AndBack(t *testing.T) {
	srv := fakes3.NewWithBuckets("nivis-state")
	defer srv.Close()

	dir := t.TempDir()
	path := filepath.Join(dir, "nivis.state.json")
	local := setupLocal(t, path, "alpha.alpha_token.app", "beta.beta_record.app")
	remote := openS3(t, srv, "nivis-state", "prod/app.json")

	// Up.
	res, err := state.Migrate(local, remote, state.MigrateOptions{})
	if err != nil {
		t.Fatalf("migrate local -> s3: %v", err)
	}
	if res.Resources != 2 {
		t.Errorf("moved %d resource(s), want 2", res.Resources)
	}
	if got := ids(t, remote); strings.Join(got, ",") != "alpha.alpha_token.app,beta.beta_record.app" {
		t.Errorf("remote holds %v", got)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("the local state file survived the migration (err = %v)", err)
	}
	if !srv.Has("nivis-state", "prod/app.json") {
		t.Error("the s3 state object was not written")
	}

	// Back down.
	local2, err := state.Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	res, err = state.Migrate(remote, local2, state.MigrateOptions{})
	if err != nil {
		t.Fatalf("migrate s3 -> local: %v", err)
	}
	if res.Resources != 2 {
		t.Errorf("moved %d resource(s) back, want 2", res.Resources)
	}
	if got := ids(t, local2); strings.Join(got, ",") != "alpha.alpha_token.app,beta.beta_record.app" {
		t.Errorf("local holds %v after the return trip", got)
	}
	if srv.Has("nivis-state", "prod/app.json") {
		t.Error("the s3 state object survived the migration back to local")
	}
}

// --- guard behaviour through the real backends (4.4, 4.5) --------------------

// A destination holding different resources is refused, with both counts, and
// nothing is touched.
func TestMigrateRefusesConflictingDestination(t *testing.T) {
	srv := fakes3.NewWithBuckets("nivis-state")
	defer srv.Close()

	path := filepath.Join(t.TempDir(), "nivis.state.json")
	local := setupLocal(t, path, "a", "b")
	remote := openS3(t, srv, "nivis-state", "prod/app.json")
	if err := remote.Set(state.ResourceState{ID: "z", Type: "alpha_token", Attrs: map[string]interface{}{"id": "z"}}); err != nil {
		t.Fatalf("seed remote: %v", err)
	}

	_, err := state.Migrate(local, remote, state.MigrateOptions{})
	var ce *state.ConflictError
	if !errors.As(err, &ce) {
		t.Fatalf("error = %v (%T), want a *ConflictError", err, err)
	}
	if ce.SourceResources != 2 || ce.DestinationResources != 1 {
		t.Errorf("ConflictError = %+v, want source 2 / destination 1", ce)
	}
	if got := ids(t, remote); strings.Join(got, ",") != "z" {
		t.Errorf("the destination was modified despite the refusal: %v", got)
	}
	if got := ids(t, local); len(got) != 2 {
		t.Errorf("the source was modified despite the refusal: %v", got)
	}

	// Forced, the same migration overwrites the destination and removes the source.
	if _, err := state.Migrate(local, remote, state.MigrateOptions{Force: true}); err != nil {
		t.Fatalf("forced migrate: %v", err)
	}
	if got := ids(t, remote); strings.Join(got, ",") != "a,b" {
		t.Errorf("forced destination holds %v, want a,b", got)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("the source survived a forced migration")
	}
}

// An empty destination document is not a conflict: it is the ordinary fresh-backend
// case and must not require --force.
func TestMigrateEmptyDestinationIsNotAConflict(t *testing.T) {
	srv := fakes3.NewWithBuckets("nivis-state")
	defer srv.Close()

	path := filepath.Join(t.TempDir(), "nivis.state.json")
	local := setupLocal(t, path, "a")
	remote := openS3(t, srv, "nivis-state", "prod/app.json")
	// An empty document already present at the destination (a prior half-attempt).
	if err := remote.Restore([]byte(`{"resources":{}}`)); err != nil {
		t.Fatalf("seed empty remote: %v", err)
	}

	if _, err := state.Migrate(local, remote, state.MigrateOptions{}); err != nil {
		t.Fatalf("migrate into an empty destination should not need --force: %v", err)
	}
	if got := ids(t, remote); strings.Join(got, ",") != "a" {
		t.Errorf("destination holds %v, want a", got)
	}
}

// An interrupted migration (copy landed, source removal did not) resumes: it does
// not report a conflict, it removes the source, and the destination is unchanged.
func TestMigrateResumesInterruptedMigration(t *testing.T) {
	srv := fakes3.NewWithBuckets("nivis-state")
	defer srv.Close()

	path := filepath.Join(t.TempDir(), "nivis.state.json")
	local := setupLocal(t, path, "a", "b")
	remote := openS3(t, srv, "nivis-state", "prod/app.json")

	// Model the interruption: the destination already holds exactly the source
	// document, but the source is still there.
	snap, err := local.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	if err := remote.Restore(snap); err != nil {
		t.Fatalf("seed destination: %v", err)
	}

	res, err := state.Migrate(local, remote, state.MigrateOptions{})
	if err != nil {
		t.Fatalf("resuming an interrupted migration should succeed: %v", err)
	}
	if !res.Resumed {
		t.Error("result should report a resumed migration")
	}
	if res.Resources != 2 {
		t.Errorf("resumed migration reports %d resource(s), want 2", res.Resources)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("the source was not removed by the resumed migration")
	}
	if got := ids(t, remote); strings.Join(got, ",") != "a,b" {
		t.Errorf("destination holds %v after the resume, want a,b", got)
	}
}

// Content at the destination that is not a state document is refused unless forced.
func TestMigrateRefusesUnparseableDestination(t *testing.T) {
	dir := t.TempDir()
	srcPath := filepath.Join(dir, "src.json")
	dstPath := filepath.Join(dir, "dst.json")
	local := setupLocal(t, srcPath, "a")
	if err := os.WriteFile(dstPath, []byte("this is not a state document"), 0o600); err != nil {
		t.Fatalf("write destination: %v", err)
	}
	dst, err := state.Open(dstPath)
	if err != nil {
		t.Fatalf("Open destination: %v", err)
	}

	_, err = state.Migrate(local, dst, state.MigrateOptions{})
	var ce *state.ConflictError
	if !errors.As(err, &ce) {
		t.Fatalf("error = %v (%T), want a *ConflictError", err, err)
	}
	if !ce.Unparseable {
		t.Error("ConflictError should be flagged unparseable")
	}
	data, _ := os.ReadFile(dstPath)
	if string(data) != "this is not a state document" {
		t.Errorf("the unrecognized destination content was overwritten: %q", data)
	}

	if _, err := state.Migrate(local, dst, state.MigrateOptions{Force: true}); err != nil {
		t.Fatalf("forced migrate over unparseable content: %v", err)
	}
	if got := ids(t, dst); strings.Join(got, ",") != "a" {
		t.Errorf("forced destination holds %v, want a", got)
	}
}

// A migration into a backend whose bucket does not exist fails with the
// missing-bucket error and leaves the source alone (the bootstrap ordering: the
// bucket must exist before state can move into it).
func TestMigrateIntoMissingBucketKeepsSource(t *testing.T) {
	srv := fakes3.NewWithBuckets() // no buckets
	defer srv.Close()

	path := filepath.Join(t.TempDir(), "nivis.state.json")
	local := setupLocal(t, path, "a")
	remote := openS3(t, srv, "not-yet", "prod/app.json")

	_, err := state.Migrate(local, remote, state.MigrateOptions{})
	if err == nil {
		t.Fatal("expected the migration to fail against a missing bucket")
	}
	var mb *state.MissingBucketError
	if !errors.As(err, &mb) {
		t.Errorf("error = %v (%T), want a *MissingBucketError", err, err)
	}
	if _, statErr := os.Stat(path); statErr != nil {
		t.Errorf("the source state file was lost: %v", statErr)
	}
	if got := ids(t, local); strings.Join(got, ",") != "a" {
		t.Errorf("the source document changed: %v", got)
	}
}
