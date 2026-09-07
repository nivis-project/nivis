// Copyright 2026 TechNative B.V. and the nivis authors
// SPDX-License-Identifier: Apache-2.0

package fakes3_test

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/nivis-project/nivis/internal/fakes3"
)

// get issues a raw GET against the fake and returns the status and body, so the
// fake's own wire behaviour is asserted without involving the AWS SDK.
func get(t *testing.T, srv *fakes3.Server, bucket, key string) (int, string) {
	t.Helper()
	resp, err := http.Get(srv.URL() + "/" + bucket + "/" + key)
	if err != nil {
		t.Fatalf("GET %s/%s: %v", bucket, key, err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(body)
}

// put issues a raw PUT against the fake and returns the status and body.
func put(t *testing.T, srv *fakes3.Server, bucket, key, body string) (int, string) {
	t.Helper()
	req, err := http.NewRequest(http.MethodPut, srv.URL()+"/"+bucket+"/"+key, strings.NewReader(body))
	if err != nil {
		t.Fatalf("new PUT: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PUT %s/%s: %v", bucket, key, err)
	}
	defer resp.Body.Close()
	out, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(out)
}

// A server built with NewWithBuckets knows only the buckets it was given: the
// known one behaves normally, an unknown one is NoSuchBucket (not NoSuchKey).
func TestWithBucketsKnownAndUnknown(t *testing.T) {
	srv := fakes3.NewWithBuckets("known")
	defer srv.Close()

	if code, _ := put(t, srv, "known", "state.json", `{"resources":{}}`); code != http.StatusOK {
		t.Fatalf("PUT to a known bucket: status = %d, want 200", code)
	}
	code, body := get(t, srv, "known", "state.json")
	if code != http.StatusOK || !strings.Contains(body, "resources") {
		t.Errorf("GET from a known bucket: status = %d, body = %q", code, body)
	}

	// A missing OBJECT in a known bucket is still NoSuchKey.
	code, body = get(t, srv, "known", "absent.json")
	if code != http.StatusNotFound || !strings.Contains(body, "NoSuchKey") {
		t.Errorf("GET a missing object: status = %d, body = %q; want 404 NoSuchKey", code, body)
	}

	// An unknown BUCKET is NoSuchBucket, on both read and write.
	code, body = get(t, srv, "nope", "state.json")
	if code != http.StatusNotFound || !strings.Contains(body, "NoSuchBucket") {
		t.Errorf("GET from an unknown bucket: status = %d, body = %q; want 404 NoSuchBucket", code, body)
	}
	code, body = put(t, srv, "nope", "state.json", "{}")
	if code != http.StatusNotFound || !strings.Contains(body, "NoSuchBucket") {
		t.Errorf("PUT to an unknown bucket: status = %d, body = %q; want 404 NoSuchBucket", code, body)
	}
}

// No buckets at all: every bucket is missing until CreateBucket makes one exist,
// which is how a bootstrap run's own bucket comes into being mid-test.
func TestCreateBucketMakesItExist(t *testing.T) {
	srv := fakes3.NewWithBuckets()
	defer srv.Close()

	if code, body := get(t, srv, "later", "state.json"); code != http.StatusNotFound || !strings.Contains(body, "NoSuchBucket") {
		t.Fatalf("before CreateBucket: status = %d, body = %q; want 404 NoSuchBucket", code, body)
	}
	srv.CreateBucket("later")
	if code, _ := put(t, srv, "later", "state.json", `{"resources":{}}`); code != http.StatusOK {
		t.Errorf("after CreateBucket, PUT status = %d, want 200", code)
	}
}

// New() keeps the historical accept-any-bucket behaviour, so existing tests that
// do not care about bucket existence are unaffected.
func TestNewAcceptsAnyBucket(t *testing.T) {
	srv := fakes3.New()
	defer srv.Close()

	if code, _ := put(t, srv, "whatever", "state.json", `{"resources":{}}`); code != http.StatusOK {
		t.Errorf("PUT to an arbitrary bucket: status = %d, want 200", code)
	}
	srv.CreateBucket("whatever") // no-op, must not panic
}

// A denied bucket answers AccessDenied, distinct from both NoSuchBucket and
// NoSuchKey, so the store can be shown not to conflate them.
func TestDenyBucket(t *testing.T) {
	srv := fakes3.NewWithBuckets("locked")
	defer srv.Close()
	srv.DenyBucket("locked")

	code, body := get(t, srv, "locked", "state.json")
	if code != http.StatusForbidden || !strings.Contains(body, "AccessDenied") {
		t.Errorf("GET a denied bucket: status = %d, body = %q; want 403 AccessDenied", code, body)
	}
	code, body = put(t, srv, "locked", "state.json", "{}")
	if code != http.StatusForbidden || !strings.Contains(body, "AccessDenied") {
		t.Errorf("PUT a denied bucket: status = %d, body = %q; want 403 AccessDenied", code, body)
	}
}
