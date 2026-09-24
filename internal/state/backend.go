// Copyright 2026 TechNative B.V. and the nivis authors
// SPDX-License-Identifier: Apache-2.0

package state

import (
	"context"
	"fmt"
)

// OpenBackend selects a Store from an IR `backend` block (see docs/IR-CONTRACT.md).
// A nil/empty backend, or `type == "local"`, uses the local file store at
// localPath (today's default and the --state path). `type == "s3"` opens the S3
// store from the backend's bucket/key/region (+ optional endpoint). The location
// comes from the backend; credentials never do (they come from the AWS chain).
//
// Required keys and an unsupported type fail with an actionable error.
func OpenBackend(backend map[string]interface{}, localPath string) (Store, error) {
	if len(backend) == 0 {
		return Open(localPath)
	}
	typ, _ := backend["type"].(string)
	if err := validateBackendKeys(typ, backend); err != nil {
		return nil, err
	}
	switch typ {
	case "", "local":
		return Open(localPath)
	case "s3":
		bucket, err := requireStr(backend, "bucket")
		if err != nil {
			return nil, err
		}
		key, err := requireStr(backend, "key")
		if err != nil {
			return nil, err
		}
		region, err := requireStr(backend, "region")
		if err != nil {
			return nil, err
		}
		endpoint, _ := backend["endpoint"].(string) // optional (tests / S3-compatible)
		sse, err := resolveSSE(backend)
		if err != nil {
			return nil, err
		}
		return newS3Store(context.Background(), bucket, key, region, endpoint, sse)
	default:
		return nil, fmt.Errorf("state: unsupported backend type %q (supported: \"s3\", \"local\")", typ)
	}
}

// requireStr extracts a required non-empty string key from a backend block.
func requireStr(backend map[string]interface{}, key string) (string, error) {
	v, ok := backend[key]
	if !ok {
		return "", fmt.Errorf("state: s3 backend requires %q (the backend declares the state location)", key)
	}
	s, ok := v.(string)
	if !ok || s == "" {
		return "", fmt.Errorf("state: s3 backend %q must be a non-empty string, got %v", key, v)
	}
	return s, nil
}

// Server-side encryption modes the s3 backend accepts for the objects it writes.
const (
	// sseAES256 requests SSE-S3. It is the value assumed when sseAlgorithm is
	// absent, so configurations written before the setting existed are unaffected.
	sseAES256 = "AES256"
	// sseBucketDefault sends no encryption header at all, leaving the bucket's own
	// default encryption rule to decide. This is the mode that works against a
	// bucket whose policy denies a write carrying a different header.
	sseBucketDefault = "bucket-default"
	// sseKMS requests SSE-KMS with an explicit key.
	sseKMS = "aws:kms"
)

// sseConfig is the resolved encryption setting, parsed once so every object the
// backend writes is built from one value. The state object and the lock object
// must agree: the lock is written first, so a setting honoured by only one of them
// still fails against an enforcing bucket.
type sseConfig struct {
	algorithm string
	kmsKeyID  string
}

// resolveSSE reads sseAlgorithm/kmsKeyId from a backend block and validates them
// against each other. It runs before any client is constructed, so a bad
// combination is reported as a configuration error and never as an opaque denial
// from S3.
func resolveSSE(backend map[string]interface{}) (sseConfig, error) {
	cfg := sseConfig{algorithm: sseAES256}

	if raw, present := backend["sseAlgorithm"]; present {
		alg, ok := raw.(string)
		if !ok {
			return cfg, fmt.Errorf("state: s3 backend %q must be a string, got %v", "sseAlgorithm", raw)
		}
		switch alg {
		case sseAES256, sseBucketDefault, sseKMS:
			cfg.algorithm = alg
		default:
			return cfg, fmt.Errorf("state: s3 backend sseAlgorithm %q is not supported (accepted: %q, %q, %q)",
				alg, sseAES256, sseBucketDefault, sseKMS)
		}
	}

	raw, present := backend["kmsKeyId"]
	if present {
		id, ok := raw.(string)
		if !ok || id == "" {
			return cfg, fmt.Errorf("state: s3 backend %q must be a non-empty string, got %v", "kmsKeyId", raw)
		}
		cfg.kmsKeyID = id
	}

	switch {
	case cfg.algorithm == sseKMS && cfg.kmsKeyID == "":
		return cfg, fmt.Errorf(`state: s3 backend sseAlgorithm = %q requires kmsKeyId (the KMS key to encrypt state with)`, sseKMS)
	case cfg.algorithm != sseKMS && present:
		return cfg, fmt.Errorf(`state: s3 backend kmsKeyId is only meaningful with sseAlgorithm = %q, but sseAlgorithm is %q; `+
			"remove kmsKeyId or set sseAlgorithm, rather than encrypting with a different key than the one named",
			sseKMS, cfg.algorithm)
	}
	return cfg, nil
}
