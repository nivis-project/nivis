// Copyright 2026 TechNative B.V. and the nivis authors
// SPDX-License-Identifier: Apache-2.0

package state_test

import (
	"strings"
	"testing"

	"github.com/nivis-project/nivis/internal/fakes3"
	"github.com/nivis-project/nivis/internal/state"
)

// openS3With opens an s3 store with extra backend keys merged over the base.
func openS3With(t *testing.T, srv *fakes3.Server, bucket, key string, extra map[string]interface{}) (state.Store, error) {
	t.Helper()
	t.Setenv("AWS_ACCESS_KEY_ID", "test")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "test")
	t.Setenv("AWS_REGION", "us-east-1")
	b := s3Backend(srv.URL(), bucket, key)
	for k, v := range extra {
		b[k] = v
	}
	return state.OpenBackend(b, "")
}

func mustOpenS3With(t *testing.T, srv *fakes3.Server, bucket, key string, extra map[string]interface{}) state.Store {
	t.Helper()
	st, err := openS3With(t, srv, bucket, key, extra)
	if err != nil {
		t.Fatalf("OpenBackend(%v): %v", extra, err)
	}
	return st
}

const testKeyARN = "arn:aws:kms:eu-central-1:104144963194:key/abc-123"

// The three modes produce the three request shapes, and an absent sseAlgorithm is
// AES256 (the behaviour configurations written before the setting existed rely on).
func TestS3EncryptionModesOnStateObject(t *testing.T) {
	cases := []struct {
		name    string
		extra   map[string]interface{}
		wantSSE string
		wantKey string
	}{
		{"absent defaults to AES256", nil, "AES256", ""},
		{"explicit AES256", map[string]interface{}{"sseAlgorithm": "AES256"}, "AES256", ""},
		{"bucket-default sends nothing", map[string]interface{}{"sseAlgorithm": "bucket-default"}, "", ""},
		{"aws:kms sends algorithm and key", map[string]interface{}{"sseAlgorithm": "aws:kms", "kmsKeyId": testKeyARN}, "aws:kms", testKeyARN},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := fakes3.New()
			defer srv.Close()
			st := mustOpenS3With(t, srv, "b", "k", tc.extra)

			if err := st.Set(state.ResourceState{ID: "a.t.n", Type: "t", Attrs: map[string]interface{}{"id": "x"}}); err != nil {
				t.Fatalf("set: %v", err)
			}
			if got := srv.SSEFor("b", "k"); got != tc.wantSSE {
				t.Errorf("state object SSE = %q, want %q", got, tc.wantSSE)
			}
			if got := srv.KMSKeyFor("b", "k"); got != tc.wantKey {
				t.Errorf("state object KMS key = %q, want %q", got, tc.wantKey)
			}
		})
	}
}

// The lock object carries EXACTLY the same encryption parameters as a state write.
// The lock is written first in a mutating run, so a drift between the two sites
// still fails against an enforcing bucket.
func TestS3LockObjectMatchesStateObjectEncryption(t *testing.T) {
	modes := []map[string]interface{}{
		nil,
		{"sseAlgorithm": "bucket-default"},
		{"sseAlgorithm": "aws:kms", "kmsKeyId": testKeyARN},
	}
	for _, extra := range modes {
		srv := fakes3.New()
		st := mustOpenS3With(t, srv, "b", "k", extra)

		lk, ok := st.(state.Locker)
		if !ok {
			srv.Close()
			t.Fatal("the s3 store should implement state.Locker")
		}
		if _, err := lk.Lock(state.NewLockInfo("apply")); err != nil {
			srv.Close()
			t.Fatalf("lock (%v): %v", extra, err)
		}
		if err := st.Set(state.ResourceState{ID: "a.t.n", Type: "t", Attrs: map[string]interface{}{}}); err != nil {
			srv.Close()
			t.Fatalf("set (%v): %v", extra, err)
		}

		if lockSSE, stateSSE := srv.SSEFor("b", "k.lock"), srv.SSEFor("b", "k"); lockSSE != stateSSE {
			t.Errorf("mode %v: lock SSE %q != state SSE %q", extra, lockSSE, stateSSE)
		}
		if lockKey, stateKey := srv.KMSKeyFor("b", "k.lock"), srv.KMSKeyFor("b", "k"); lockKey != stateKey {
			t.Errorf("mode %v: lock KMS key %q != state KMS key %q", extra, lockKey, stateKey)
		}
		srv.Close()
	}
}

// Reads send no encryption parameters, so every mode round-trips.
func TestS3ReadsAreUnaffectedByEncryptionMode(t *testing.T) {
	modes := []map[string]interface{}{
		nil,
		{"sseAlgorithm": "bucket-default"},
		{"sseAlgorithm": "aws:kms", "kmsKeyId": testKeyARN},
	}
	for _, extra := range modes {
		srv := fakes3.New()
		st := mustOpenS3With(t, srv, "b", "k", extra)
		rs := state.ResourceState{ID: "a.t.n", Type: "t", Attrs: map[string]interface{}{"v": "1"}}
		if err := st.Set(rs); err != nil {
			srv.Close()
			t.Fatalf("set (%v): %v", extra, err)
		}
		st2 := mustOpenS3With(t, srv, "b", "k", extra)
		got, found, err := st2.Get("a.t.n")
		if err != nil || !found {
			srv.Close()
			t.Fatalf("get (%v): found=%v err=%v", extra, found, err)
		}
		if got.Attrs["v"] != "1" {
			t.Errorf("mode %v: round-tripped value = %v, want 1", extra, got.Attrs["v"])
		}
		srv.Close()
	}
}

// A bucket that enforces SSE-KMS accepts a bucket-default run and denies the same
// run under the default AES256. This is the failure the change exists for.
func TestS3EnforcingBucketAcceptsBucketDefault(t *testing.T) {
	srv := fakes3.New()
	defer srv.Close()
	srv.DenyMismatchedSSE("hardened", "aws:kms")

	deflt := mustOpenS3With(t, srv, "hardened", "k", nil)
	if err := deflt.Set(state.ResourceState{ID: "a.t.n", Type: "t"}); err == nil {
		t.Fatal("the default AES256 mode should be denied by an enforcing bucket")
	}

	st := mustOpenS3With(t, srv, "hardened", "k", map[string]interface{}{"sseAlgorithm": "bucket-default"})
	lk, ok := st.(state.Locker)
	if !ok {
		t.Fatal("the s3 store should implement state.Locker")
	}
	if _, err := lk.Lock(state.NewLockInfo("apply")); err != nil {
		t.Fatalf("lock under bucket-default should be accepted: %v", err)
	}
	if err := st.Set(state.ResourceState{ID: "a.t.n", Type: "t", Attrs: map[string]interface{}{"id": "x"}}); err != nil {
		t.Fatalf("state write under bucket-default should be accepted: %v", err)
	}
	if !srv.Has("hardened", "k") {
		t.Error("the state object should exist after an accepted write")
	}
}

// A denied write keeps the underlying error AND gains a hint matched to the mode.
//
// Each case needs a bucket that ACCEPTS the read and denies the write, since Set
// is a read-modify-write: a bucket that denies everything fails at the GET, which
// carries no hint by design.
func TestS3DeniedWriteHints(t *testing.T) {
	cases := []struct {
		name  string
		extra map[string]interface{}
		deny  func(*fakes3.Server, string)
		wants []string
	}{
		{
			"AES256 points at bucket-default",
			nil,
			func(s *fakes3.Server, b string) { s.DenyMismatchedSSE(b, "aws:kms") },
			[]string{"AES256", `"bucket-default"`},
		},
		{
			"aws:kms points at the key identifier",
			map[string]interface{}{"sseAlgorithm": "aws:kms", "kmsKeyId": testKeyARN},
			func(s *fakes3.Server, b string) { s.DenyMismatchedSSE(b, "AES256") },
			[]string{"kmsKeyId", testKeyARN, "exact key identifier"},
		},
		{
			"bucket-default points at aws:kms",
			map[string]interface{}{"sseAlgorithm": "bucket-default"},
			func(s *fakes3.Server, b string) { s.DenyMissingSSE(b) },
			[]string{"no encryption header", `"aws:kms"`},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := fakes3.New()
			defer srv.Close()
			tc.deny(srv, "hardened")
			st := mustOpenS3With(t, srv, "hardened", "k", tc.extra)

			err := st.Set(state.ResourceState{ID: "a.t.n", Type: "t"})
			if err == nil {
				t.Fatal("a denied write should fail")
			}
			got := err.Error()
			// The underlying denial is preserved above the hint: it may have
			// nothing to do with encryption.
			if !strings.Contains(got, "AccessDenied") {
				t.Errorf("the underlying error should still be present, got: %s", got)
			}
			if !strings.Contains(got, "encryption mismatch") {
				t.Errorf("expected an encryption hint, got: %s", got)
			}
			for _, w := range tc.wants {
				if !strings.Contains(got, w) {
					t.Errorf("hint should mention %q, got: %s", w, got)
				}
			}
		})
	}
}

// A denied LOCK acquisition hints too: it is a write, and it is the first S3 call
// of a mutating run, so it is where an enforcing bucket bites first.
func TestS3DeniedLockHints(t *testing.T) {
	srv := fakes3.New()
	defer srv.Close()
	srv.DenyMismatchedSSE("locked", "aws:kms")
	st := mustOpenS3With(t, srv, "locked", "k", nil)

	lk, ok := st.(state.Locker)
	if !ok {
		t.Fatal("the s3 store should implement state.Locker")
	}
	_, err := lk.Lock(state.NewLockInfo("apply"))
	if err == nil {
		t.Fatal("a denied lock should fail")
	}
	if !strings.Contains(err.Error(), "encryption mismatch") {
		t.Errorf("expected an encryption hint on a denied lock, got: %s", err)
	}
}

// A denied READ carries no encryption hint: a read sends no encryption parameters,
// so pointing at the mode would be a wrong lead.
func TestS3DeniedReadHasNoEncryptionHint(t *testing.T) {
	srv := fakes3.New()
	defer srv.Close()
	srv.DenyBucket("locked")
	st := mustOpenS3With(t, srv, "locked", "k", nil)

	_, _, err := st.Get("a.t.n")
	if err == nil {
		t.Fatal("a denied read should fail")
	}
	if strings.Contains(err.Error(), "encryption mismatch") {
		t.Errorf("a read must not carry an encryption hint, got: %s", err)
	}
	if !strings.Contains(err.Error(), "AccessDenied") {
		t.Errorf("the access failure should surface, got: %s", err)
	}
}

// A migration between two s3 backends with DIFFERENT encryption settings works
// with each side honouring its own: the migration path needs no special casing,
// because both stores were resolved through the same OpenBackend.
func TestMigrateBetweenBackendsWithDifferentEncryption(t *testing.T) {
	srv := fakes3.New()
	defer srv.Close()

	src := mustOpenS3With(t, srv, "old", "state.json", nil)
	dst := mustOpenS3With(t, srv, "new", "state.json", map[string]interface{}{"sseAlgorithm": "bucket-default"})

	want := []state.ResourceState{
		{ID: "alpha.alpha_token.a", Type: "alpha_token", Attrs: map[string]interface{}{"id": "t1"}},
		{ID: "beta.beta_record.b", Type: "beta_record", Attrs: map[string]interface{}{"id": "r1"}},
	}
	for _, rs := range want {
		if err := src.Set(rs); err != nil {
			t.Fatalf("seed source: %v", err)
		}
	}

	// The source wrote under ITS setting. Read this before migrating: the final
	// step of a migration removes the source document, so nothing about it is
	// observable afterwards.
	if sse := srv.SSEFor("old", "state.json"); sse != "AES256" {
		t.Errorf("source object SSE = %q, want AES256", sse)
	}

	res, err := state.Migrate(src, dst, state.MigrateOptions{})
	if err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if res.Resources != len(want) {
		t.Errorf("migrated %d resources, want %d", res.Resources, len(want))
	}

	got, err := dst.List()
	if err != nil {
		t.Fatalf("list destination: %v", err)
	}
	if len(got) != len(want) {
		t.Fatalf("destination holds %d resources, want %d", len(got), len(want))
	}

	// The destination wrote under ITS setting, and the source is gone.
	if sse := srv.SSEFor("new", "state.json"); sse != "" {
		t.Errorf("destination object SSE = %q, want none (bucket-default)", sse)
	}
	if srv.Has("old", "state.json") {
		t.Error("the source document should have been removed by the migration")
	}
}
