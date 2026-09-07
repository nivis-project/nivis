// Copyright 2026 TechNative B.V. and the nivis authors
// SPDX-License-Identifier: Apache-2.0

package state_test

// The S3 backend against a fake that models bucket EXISTENCE: a missing bucket is
// an actionable error on every path (read, write, and lock acquisition), never an
// empty state document, and a permission failure is not reported as a missing
// bucket.

import (
	"errors"
	"strings"
	"testing"

	"github.com/nivis-project/nivis/internal/fakes3"
	"github.com/nivis-project/nivis/internal/state"
)

// assertMissingBucket fails unless err is the actionable missing-bucket error for
// the given bucket, carrying the bootstrap recipe.
func assertMissingBucket(t *testing.T, err error, bucket string) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected a missing-bucket error for %q, got nil (an empty state was assumed)", bucket)
	}
	var mb *state.MissingBucketError
	if !errors.As(err, &mb) {
		t.Fatalf("error is not a *MissingBucketError: %T: %v", err, err)
	}
	if mb.Bucket != bucket {
		t.Errorf("MissingBucketError.Bucket = %q, want %q", mb.Bucket, bucket)
	}
	msg := err.Error()
	for _, want := range []string{bucket, "us-east-1", "nivis apply --backend=local", "nivis state migrate --to-remote"} {
		if !strings.Contains(msg, want) {
			t.Errorf("missing-bucket message lacks %q:\n%s", want, msg)
		}
	}
}

// Reads against a nonexistent bucket error out instead of reporting empty state.
func TestS3MissingBucketOnRead(t *testing.T) {
	srv := fakes3.NewWithBuckets() // no buckets exist
	defer srv.Close()
	st := openS3(t, srv, "not-yet", "prod/app.json")

	_, _, getErr := st.Get("alpha.alpha_token.app")
	assertMissingBucket(t, getErr, "not-yet")
	_, listErr := st.List()
	assertMissingBucket(t, listErr, "not-yet")
	_, snapErr := st.Snapshot()
	assertMissingBucket(t, snapErr, "not-yet")
}

// Writes against a nonexistent bucket error out with the same actionable message.
func TestS3MissingBucketOnWrite(t *testing.T) {
	srv := fakes3.NewWithBuckets()
	defer srv.Close()
	st := openS3(t, srv, "not-yet", "prod/app.json")

	err := st.Set(state.ResourceState{ID: "alpha.alpha_token.app", Type: "alpha_token", Attrs: map[string]interface{}{"id": "t1"}})
	assertMissingBucket(t, err, "not-yet")

	assertMissingBucket(t, st.Restore([]byte(`{"resources":{}}`)), "not-yet")
	assertMissingBucket(t, st.Delete("alpha.alpha_token.app"), "not-yet")
}

// The bootstrap failure the bean records happens at LOCK acquisition, which is an
// apply's first S3 call — before any state read. It must report the missing bucket
// actionably, not a raw SDK error.
func TestS3MissingBucketOnLockAcquisition(t *testing.T) {
	srv := fakes3.NewWithBuckets()
	defer srv.Close()
	st := openS3(t, srv, "not-yet", "prod/app.json")

	lk, ok := st.(state.Locker)
	if !ok {
		t.Fatal("the s3 store should implement state.Locker")
	}
	_, err := lk.Lock(state.NewLockInfo("apply"))
	assertMissingBucket(t, err, "not-yet")

	// force-unlock hits DeleteObject on the same absent bucket: same message.
	assertMissingBucket(t, lk.ForceUnlock(), "not-yet")
}

// Once the bucket exists (the run created it), the same store works — the error
// was about the location, not the configuration.
func TestS3WorksAfterBucketIsCreated(t *testing.T) {
	srv := fakes3.NewWithBuckets()
	defer srv.Close()
	st := openS3(t, srv, "made-later", "prod/app.json")

	assertMissingBucket(t, st.Set(state.ResourceState{ID: "a", Type: "alpha_token"}), "made-later")

	srv.CreateBucket("made-later")
	if err := st.Set(state.ResourceState{ID: "a", Type: "alpha_token", Attrs: map[string]interface{}{"id": "t1"}}); err != nil {
		t.Fatalf("Set after the bucket exists: %v", err)
	}
	got, found, err := st.Get("a")
	if err != nil || !found || got.Attrs["id"] != "t1" {
		t.Errorf("Get after the bucket exists = (%+v, %v, %v)", got, found, err)
	}
}

// A permission failure keeps surfacing its own cause: it is neither a missing
// bucket nor an empty state.
func TestS3AccessDeniedIsNotAMissingBucket(t *testing.T) {
	srv := fakes3.NewWithBuckets("exists")
	defer srv.Close()
	srv.DenyBucket("exists")
	st := openS3(t, srv, "exists", "prod/app.json")

	_, err := st.List()
	if err == nil {
		t.Fatal("a denied bucket should fail, not report empty state")
	}
	var mb *state.MissingBucketError
	if errors.As(err, &mb) {
		t.Errorf("AccessDenied was reported as a missing bucket: %v", err)
	}
	if !strings.Contains(err.Error(), "AccessDenied") && !strings.Contains(err.Error(), "403") {
		t.Errorf("the access failure is not surfaced in the error: %v", err)
	}
}
