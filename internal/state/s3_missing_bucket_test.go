// Copyright 2026 TechNative B.V. and the nivis authors
// SPDX-License-Identifier: Apache-2.0

package state

// Unit tests for the not-found classification split: a missing OBJECT is an empty
// document, a missing BUCKET is an actionable error, and anything else (notably a
// permission failure) is neither. These are in-package because the classifiers are
// internal.

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"
)

// apiError is a synthesized SDK error carrying a wire error code, which is how the
// real deserializer surfaces S3's XML <Code> to callers.
type apiError struct{ code string }

func (e *apiError) Error() string                 { return e.code }
func (e *apiError) ErrorCode() string             { return e.code }
func (e *apiError) ErrorMessage() string          { return e.code }
func (e *apiError) ErrorFault() smithy.ErrorFault { return smithy.FaultServer }

func TestNotFoundClassification(t *testing.T) {
	cases := []struct {
		name          string
		err           error
		missingObject bool
		missingBucket bool
	}{
		{"nil", nil, false, false},
		{"typed NoSuchKey", &types.NoSuchKey{}, true, false},
		{"typed NotFound", &types.NotFound{}, true, false},
		{"typed NoSuchBucket", &types.NoSuchBucket{}, false, true},
		{"code NoSuchKey", &apiError{code: "NoSuchKey"}, true, false},
		{"code NotFound", &apiError{code: "NotFound"}, true, false},
		{"code 404", &apiError{code: "404"}, true, false},
		{"code NoSuchBucket", &apiError{code: "NoSuchBucket"}, false, true},
		{"code AccessDenied", &apiError{code: "AccessDenied"}, false, false},
		{"code InvalidAccessKeyId", &apiError{code: "InvalidAccessKeyId"}, false, false},
		{"code SlowDown", &apiError{code: "SlowDown"}, false, false},
		{"plain error", errors.New("dial tcp: connection refused"), false, false},
		{"wrapped NoSuchBucket", fmt.Errorf("get: %w", &apiError{code: "NoSuchBucket"}), false, true},
		{"wrapped NoSuchKey", fmt.Errorf("get: %w", &types.NoSuchKey{}), true, false},
		{"wrapped AccessDenied", fmt.Errorf("get: %w", &apiError{code: "AccessDenied"}), false, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isNotFound(tc.err); got != tc.missingObject {
				t.Errorf("isNotFound(%v) = %v, want %v", tc.err, got, tc.missingObject)
			}
			if got := isMissingBucket(tc.err); got != tc.missingBucket {
				t.Errorf("isMissingBucket(%v) = %v, want %v", tc.err, got, tc.missingBucket)
			}
			// The two conditions are mutually exclusive by construction: a missing
			// bucket must never be reported as an absent object (which would read as
			// an empty state document).
			if isNotFound(tc.err) && isMissingBucket(tc.err) {
				t.Errorf("%v classified as BOTH a missing object and a missing bucket", tc.err)
			}
		})
	}
}

// The missing-bucket error names the bucket and region, says it is not an empty
// state, and carries the bootstrap recipe.
func TestMissingBucketErrorMessage(t *testing.T) {
	st := &s3Store{bucket: "acme-nivis-state", key: "prod/app.json", region: "eu-west-1"}
	err := st.missingBucket(&apiError{code: "NoSuchBucket"})
	if err == nil {
		t.Fatal("missingBucket returned nil for a NoSuchBucket error")
	}
	var mb *MissingBucketError
	if !errors.As(err, &mb) {
		t.Fatalf("error is not a *MissingBucketError: %T", err)
	}
	if mb.Bucket != "acme-nivis-state" || mb.Region != "eu-west-1" {
		t.Errorf("MissingBucketError = %+v, want the store's bucket and region", mb)
	}
	msg := err.Error()
	for _, want := range []string{
		"acme-nivis-state", "eu-west-1", "not treated as an empty state",
		"nivis apply --backend=local", "nivis state migrate --to-remote",
		"backend.bucket", "backend.region",
	} {
		if !strings.Contains(msg, want) {
			t.Errorf("missing-bucket message lacks %q:\n%s", want, msg)
		}
	}

	// Anything that is not a missing bucket produces no MissingBucketError.
	if got := st.missingBucket(&apiError{code: "AccessDenied"}); got != nil {
		t.Errorf("missingBucket(AccessDenied) = %v, want nil", got)
	}
}
