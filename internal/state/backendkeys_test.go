// Copyright 2026 TechNative B.V. and the nivis authors
// SPDX-License-Identifier: Apache-2.0

package state_test

import (
	"strings"
	"testing"

	"github.com/nivis-project/nivis/internal/fakes3"
	"github.com/nivis-project/nivis/internal/state"
)

// s3 backend values that are valid apart from whatever the case under test adds.
func baseS3(extra map[string]interface{}) map[string]interface{} {
	b := map[string]interface{}{"type": "s3", "bucket": "b", "key": "k", "region": "r"}
	for k, v := range extra {
		b[k] = v
	}
	return b
}

// sseAlgorithm and kmsKeyId are validated against each other before any client is
// built, so a bad combination is a configuration error and not an opaque denial.
func TestOpenBackendEncryptionValidation(t *testing.T) {
	t.Setenv("AWS_ACCESS_KEY_ID", "test")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "test")

	ok := []struct {
		name  string
		extra map[string]interface{}
	}{
		{"absent", nil},
		{"AES256", map[string]interface{}{"sseAlgorithm": "AES256"}},
		{"bucket-default", map[string]interface{}{"sseAlgorithm": "bucket-default"}},
		{"aws:kms with a key", map[string]interface{}{"sseAlgorithm": "aws:kms", "kmsKeyId": "arn:aws:kms:eu-central-1:1:key/a"}},
	}
	for _, tc := range ok {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := state.OpenBackend(baseS3(tc.extra), ""); err != nil {
				t.Fatalf("%s should be accepted: %v", tc.name, err)
			}
		})
	}

	bad := []struct {
		name    string
		extra   map[string]interface{}
		wantSub []string
	}{
		{
			"unknown algorithm",
			map[string]interface{}{"sseAlgorithm": "sse-c"},
			[]string{`"sse-c" is not supported`, "AES256", "bucket-default", "aws:kms"},
		},
		{
			"aws:kms without a key",
			map[string]interface{}{"sseAlgorithm": "aws:kms"},
			[]string{"requires kmsKeyId"},
		},
		{
			"key without aws:kms",
			map[string]interface{}{"kmsKeyId": "arn:aws:kms:eu-central-1:1:key/a"},
			[]string{"only meaningful with", "AES256"},
		},
		{
			"key with bucket-default",
			map[string]interface{}{"sseAlgorithm": "bucket-default", "kmsKeyId": "arn:aws:kms:eu-central-1:1:key/a"},
			[]string{"only meaningful with", "bucket-default"},
		},
	}
	for _, tc := range bad {
		t.Run(tc.name, func(t *testing.T) {
			_, err := state.OpenBackend(baseS3(tc.extra), "")
			if err == nil {
				t.Fatalf("%s should be rejected", tc.name)
			}
			for _, w := range tc.wantSub {
				if !strings.Contains(err.Error(), w) {
					t.Errorf("error should mention %q, got: %v", w, err)
				}
			}
		})
	}
}

// Validation runs before any request: an invalid backend never reaches S3.
func TestInvalidBackendMakesNoRequest(t *testing.T) {
	srv := fakes3.New()
	defer srv.Close()
	t.Setenv("AWS_ACCESS_KEY_ID", "test")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "test")

	b := baseS3(map[string]interface{}{"endpoint": srv.URL(), "sseAlgorithm": "nonsense"})
	if _, err := state.OpenBackend(b, ""); err == nil {
		t.Fatal("an invalid sseAlgorithm should be rejected")
	}
	if srv.Has("b", "k") || srv.Has("b", "k.lock") {
		t.Error("no S3 object should have been touched by a rejected configuration")
	}
}

// An unrecognized key is an error rather than being silently ignored, for every
// backend type, with the most specific explanation available.
func TestOpenBackendRejectsUnknownKeys(t *testing.T) {
	t.Setenv("AWS_ACCESS_KEY_ID", "test")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "test")

	cases := []struct {
		name    string
		backend map[string]interface{}
		wantSub []string
	}{
		{
			"an unknown s3 key lists the accepted ones",
			baseS3(map[string]interface{}{"nonsense": "x"}),
			[]string{`unknown key`, `"nonsense"`, "sseAlgorithm", "kmsKeyId", "bucket", "region"},
		},
		{
			"a misspelling suggests the key it resembles",
			baseS3(map[string]interface{}{"sseAlgorythm": "aws:kms"}),
			[]string{`did you mean "sseAlgorithm"`},
		},
		{
			"another naming convention suggests the key",
			baseS3(map[string]interface{}{"sse_algorithm": "aws:kms"}),
			[]string{`did you mean "sseAlgorithm"`},
		},
		{
			"an upper-case spelling suggests the key",
			baseS3(map[string]interface{}{"SSEAlgorithm": "aws:kms"}),
			[]string{`did you mean "sseAlgorithm"`},
		},
		{
			"dynamodbTable explains the nivis lock",
			baseS3(map[string]interface{}{"dynamodbTable": "terraform-state-lock"}),
			[]string{"Terraform backend setting", "<key>.lock", "no table is needed"},
		},
		{
			"dynamodb_table is recognized in either convention",
			baseS3(map[string]interface{}{"dynamodb_table": "terraform-state-lock"}),
			[]string{"Terraform backend setting", "<key>.lock"},
		},
		{
			"encrypt points at sseAlgorithm",
			baseS3(map[string]interface{}{"encrypt": true}),
			[]string{"Terraform backend setting", "sseAlgorithm"},
		},
		{
			"roleArn says state assume-role is unsupported",
			baseS3(map[string]interface{}{"roleArn": "arn:aws:iam::1:role/r"}),
			[]string{"Terraform backend setting", "not supported yet", "AWS profile"},
		},
		{
			"profile points at the credential chain",
			baseS3(map[string]interface{}{"profile": "technative"}),
			[]string{"Terraform backend setting", "AWS_PROFILE"},
		},
		{
			"strictness applies to the local backend too",
			map[string]interface{}{"type": "local", "bucket": "b"},
			[]string{"local backend", `unknown key`, `"bucket"`},
		},
		{
			"every unknown key is reported, not just the first",
			baseS3(map[string]interface{}{"nonsense": "x", "alsoWrong": "y"}),
			[]string{`"alsoWrong"`, `"nonsense"`},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := state.OpenBackend(tc.backend, "")
			if err == nil {
				t.Fatalf("%s: expected a rejection", tc.name)
			}
			for _, w := range tc.wantSub {
				if !strings.Contains(err.Error(), w) {
					t.Errorf("error should mention %q, got:\n%v", w, err)
				}
			}
		})
	}
}

// The keys that ARE accepted stay accepted, so strictness does not break a valid
// configuration.
func TestOpenBackendAcceptsEveryDocumentedKey(t *testing.T) {
	t.Setenv("AWS_ACCESS_KEY_ID", "test")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "test")

	full := map[string]interface{}{
		"type": "s3", "bucket": "b", "key": "k", "region": "r",
		"endpoint": "http://127.0.0.1:1", "sseAlgorithm": "aws:kms",
		"kmsKeyId": "arn:aws:kms:eu-central-1:1:key/a",
	}
	if _, err := state.OpenBackend(full, ""); err != nil {
		t.Fatalf("a backend using every accepted key should open: %v", err)
	}
	if _, err := state.OpenBackend(map[string]interface{}{"type": "local"}, ""); err != nil {
		t.Fatalf("a local backend should open: %v", err)
	}
}
