// Copyright 2026 TechNative B.V. and the nivis authors
// SPDX-License-Identifier: Apache-2.0

package e2e_test

import (
	"context"
	"testing"

	"github.com/nivis-project/nivis/internal/fakes3"
	"github.com/nivis-project/nivis/internal/ledger"
	"github.com/nivis-project/nivis/internal/phase"
	"github.com/nivis-project/nivis/internal/plan"
	"github.com/nivis-project/nivis/internal/plugin"
	"github.com/nivis-project/nivis/internal/state"
)

// TestS3BackendRoundTrip proves the S3 remote state backend end to end against the
// REAL fake providers AND a hermetic in-repo fake S3 (no network, no AWS): a full
// apply stores state in the S3 object, a re-plan reads it back from S3 and reports
// no changes, and the stored ids are stable (a real no-op, not a re-create). This
// is the headline B1 proof: the executor's whole plan/apply path works through the
// S3 Store with nothing local.
func TestS3BackendRoundTrip(t *testing.T) {
	requireNix(t)
	root := repoRoot(t)
	buildBinaries(t, root)
	t.Setenv("TERRAE_NIVIS_FAKE_COUNTER", "")

	// The SDK needs credentials present; the fake S3 ignores them.
	t.Setenv("AWS_ACCESS_KEY_ID", "test")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "test")

	srv := fakes3.New()
	defer srv.Close()
	bucket, key := "nivis-state", "e2e/app.json"
	backend := map[string]interface{}{
		"type": "s3", "bucket": bucket, "key": key, "region": "us-east-1", "endpoint": srv.URL(),
	}

	openS3 := func() state.Store {
		st, err := state.OpenBackend(backend, "")
		if err != nil {
			t.Fatalf("open s3 backend: %v", err)
		}
		return st
	}

	newDriver := func(st state.Store) (*phase.Driver, func()) {
		mgr := plugin.NewManager()
		return &phase.Driver{
			Eval:      phase.NixEval{FlakeRef: ".", Attr: "nivis.plan", WorkDir: root},
			Manager:   mgr,
			Store:     st,
			Ledger:    ledger.New(),
			MaxPhases: 10,
		}, mgr.Close
	}

	// Apply against the S3 store.
	d, closer := newDriver(openS3())
	res, err := d.Run(context.Background())
	closer()
	if err != nil {
		t.Fatalf("apply against s3: %v", err)
	}
	if len(res.Applied) == 0 {
		t.Fatal("nothing applied")
	}

	// State landed in the S3 object (nothing local).
	if !srv.Has(bucket, key) {
		t.Fatal("state was not written to the S3 object")
	}
	if sse := srv.SSEFor(bucket, key); sse != "AES256" {
		t.Errorf("S3 state object SSE = %q, want AES256", sse)
	}

	// A fresh store over the same bucket/key reads the applied state back.
	st2 := openS3()
	ids, err := st2.List()
	if err != nil {
		t.Fatalf("list from s3: %v", err)
	}
	if len(ids) != len(res.Applied) {
		t.Errorf("s3 state has %d resources, want %d (the applied set)", len(ids), len(res.Applied))
	}

	// Re-plan against the S3 state: every resource is a no-op (read back from S3,
	// nothing changed), so the round trip through S3 is correct.
	dp, closerp := newDriver(st2)
	items, err := dp.PlanReport(context.Background())
	closerp()
	if err != nil {
		t.Fatalf("re-plan against s3: %v", err)
	}
	if len(items) == 0 {
		t.Fatal("re-plan produced no items")
	}
	for _, it := range items {
		if it.Op != plan.OpNoop {
			t.Errorf("re-plan: %s op = %v, want OpNoop (state read back from S3 unchanged)", it.ID, it.Op)
		}
	}
}

// TestS3BackendAgainstEnforcingBucket is the bean's acceptance criterion, end to
// end against the real binaries: a bucket whose policy denies any write carrying
// an encryption header other than its own default (the shape the TechNative state
// buckets carry) refuses a run under the default AES256 and accepts the same run
// under sseAlgorithm = "bucket-default".
//
// The lock object is the first S3 write of a mutating run, so this is also where
// the denial bites first.
func TestS3BackendAgainstEnforcingBucket(t *testing.T) {
	requireNix(t)
	root := repoRoot(t)
	buildBinaries(t, root)
	t.Setenv("TERRAE_NIVIS_FAKE_COUNTER", "")
	t.Setenv("AWS_ACCESS_KEY_ID", "test")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "test")

	srv := fakes3.New()
	defer srv.Close()
	bucket, key := "hardened-state", "e2e/enforced.json"
	srv.DenyMismatchedSSE(bucket, "aws:kms")

	backendWith := func(sse string) map[string]interface{} {
		b := map[string]interface{}{
			"type": "s3", "bucket": bucket, "key": key, "region": "us-east-1", "endpoint": srv.URL(),
		}
		if sse != "" {
			b["sseAlgorithm"] = sse
		}
		return b
	}
	open := func(t *testing.T, sse string) state.Store {
		t.Helper()
		st, err := state.OpenBackend(backendWith(sse), "")
		if err != nil {
			t.Fatalf("open s3 backend (sse=%q): %v", sse, err)
		}
		return st
	}
	run := func(st state.Store) error {
		mgr := plugin.NewManager()
		defer mgr.Close()
		d := &phase.Driver{
			Eval:      phase.NixEval{FlakeRef: ".", Attr: "nivis.plan", WorkDir: root},
			Manager:   mgr,
			Store:     st,
			Ledger:    ledger.New(),
			MaxPhases: 10,
		}
		_, err := d.Run(context.Background())
		return err
	}

	// The default mode is denied, and nothing lands in the bucket.
	if err := run(open(t, "")); err == nil {
		t.Fatal("an apply under the default AES256 should be denied by an enforcing bucket")
	}
	if srv.Has(bucket, key) {
		t.Error("a denied run must not have written the state object")
	}

	// bucket-default is accepted: the run completes and state lands in S3.
	if err := run(open(t, "bucket-default")); err != nil {
		t.Fatalf("an apply under bucket-default should be accepted: %v", err)
	}
	if !srv.Has(bucket, key) {
		t.Fatal("state was not written to the S3 object under bucket-default")
	}
	if sse := srv.SSEFor(bucket, key); sse != "" {
		t.Errorf("state object SSE = %q, want none (the bucket default applies)", sse)
	}

	// The state is readable back, so the run is genuinely complete and not merely
	// un-denied.
	ids, err := open(t, "bucket-default").List()
	if err != nil {
		t.Fatalf("list from s3: %v", err)
	}
	if len(ids) == 0 {
		t.Error("no resource state was persisted")
	}
}

// TestS3BackendDefaultEncryptionUnchanged pins the back-compat claim: a backend
// declaring none of the new keys produces exactly the requests it did before this
// change, on BOTH objects it writes.
func TestS3BackendDefaultEncryptionUnchanged(t *testing.T) {
	t.Setenv("AWS_ACCESS_KEY_ID", "test")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "test")

	srv := fakes3.New()
	defer srv.Close()
	bucket, key := "compat-state", "e2e/compat.json"
	st, err := state.OpenBackend(map[string]interface{}{
		"type": "s3", "bucket": bucket, "key": key, "region": "us-east-1", "endpoint": srv.URL(),
	}, "")
	if err != nil {
		t.Fatalf("open s3 backend: %v", err)
	}

	lk, ok := st.(state.Locker)
	if !ok {
		t.Fatal("the s3 store should implement state.Locker")
	}
	if _, err := lk.Lock(state.NewLockInfo("apply")); err != nil {
		t.Fatalf("lock: %v", err)
	}
	if err := st.Set(state.ResourceState{ID: "alpha.alpha_token.a", Type: "alpha_token", Attrs: map[string]interface{}{"id": "t1"}}); err != nil {
		t.Fatalf("set: %v", err)
	}

	for _, obj := range []string{key, key + ".lock"} {
		if sse := srv.SSEFor(bucket, obj); sse != "AES256" {
			t.Errorf("%s SSE = %q, want AES256 (unchanged default)", obj, sse)
		}
		if k := srv.KMSKeyFor(bucket, obj); k != "" {
			t.Errorf("%s KMS key = %q, want none under the default", obj, k)
		}
	}
}
