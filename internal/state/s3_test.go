// Copyright 2026 TechNative B.V. and the nivis authors
// SPDX-License-Identifier: Apache-2.0

package state_test

import (
	"testing"

	"github.com/wearetechnative/nivis/internal/fakes3"
	"github.com/wearetechnative/nivis/internal/state"
)

// s3 backend config pointing at a fake S3 server. Region/creds are irrelevant to
// the fake but the SDK requires a region; credentials resolve to anonymous-ish
// values that the fake ignores.
func s3Backend(endpoint, bucket, key string) map[string]interface{} {
	return map[string]interface{}{
		"type":     "s3",
		"bucket":   bucket,
		"key":      key,
		"region":   "us-east-1",
		"endpoint": endpoint,
	}
}

func openS3(t *testing.T, srv *fakes3.Server, bucket, key string) state.Store {
	t.Helper()
	// The SDK needs *some* credentials present; set static dummies for the test.
	t.Setenv("AWS_ACCESS_KEY_ID", "test")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "test")
	t.Setenv("AWS_REGION", "us-east-1")
	st, err := state.OpenBackend(s3Backend(srv.URL(), bucket, key), "")
	if err != nil {
		t.Fatalf("OpenBackend(s3): %v", err)
	}
	return st
}

// A fresh (missing) object reads as empty, and Set/Get round-trips through S3.
func TestS3StoreRoundTrip(t *testing.T) {
	srv := fakes3.New()
	defer srv.Close()
	st := openS3(t, srv, "my-state", "prod/app.json")

	// Empty before any write.
	if items, err := st.List(); err != nil {
		t.Fatalf("list (empty): %v", err)
	} else if len(items) != 0 {
		t.Fatalf("fresh store should be empty, got %d", len(items))
	}

	rs := state.ResourceState{ID: "alpha.alpha_token.A", Type: "alpha_token", Attrs: map[string]interface{}{"id": "alpha-0", "value": "v"}}
	if err := st.Set(rs); err != nil {
		t.Fatalf("set: %v", err)
	}
	// The object now exists in S3 and SSE was requested.
	if !srv.Has("my-state", "prod/app.json") {
		t.Error("state object was not created in S3")
	}
	if sse := srv.SSEFor("my-state", "prod/app.json"); sse != "AES256" {
		t.Errorf("server-side encryption = %q, want AES256", sse)
	}

	// Reopen against the same bucket/key: state persisted.
	st2 := openS3(t, srv, "my-state", "prod/app.json")
	got, found, err := st2.Get("alpha.alpha_token.A")
	if err != nil || !found {
		t.Fatalf("get after reopen: found=%v err=%v", found, err)
	}
	if got.Attrs["value"] != "v" {
		t.Errorf("round-tripped value = %v, want v", got.Attrs["value"])
	}
}

// Delete removes a resource from the S3 document.
func TestS3StoreDelete(t *testing.T) {
	srv := fakes3.New()
	defer srv.Close()
	st := openS3(t, srv, "b", "k")

	_ = st.Set(state.ResourceState{ID: "x.t.a", Type: "t", Attrs: map[string]interface{}{}})
	_ = st.Set(state.ResourceState{ID: "x.t.b", Type: "t", Attrs: map[string]interface{}{}})
	if err := st.Delete("x.t.a"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	items, _ := st.List()
	if len(items) != 1 || items[0].ID != "x.t.b" {
		t.Errorf("after delete, items = %v, want only x.t.b", items)
	}
}

// Snapshot/Restore round-trip the whole document through S3 (the pull/push seam).
func TestS3StoreSnapshotRestore(t *testing.T) {
	srv := fakes3.New()
	defer srv.Close()
	src := openS3(t, srv, "b", "src.json")
	_ = src.Set(state.ResourceState{ID: "a.b.c", Type: "t", Attrs: map[string]interface{}{"k": 1}})
	snap, err := src.Snapshot()
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}

	dst := openS3(t, srv, "b", "dst.json")
	if err := dst.Restore(snap); err != nil {
		t.Fatalf("restore: %v", err)
	}
	items, _ := dst.List()
	if len(items) != 1 || items[0].ID != "a.b.c" {
		t.Errorf("restored items = %v, want a.b.c", items)
	}
}
