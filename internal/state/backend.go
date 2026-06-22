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
		return newS3Store(context.Background(), bucket, key, region, endpoint)
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
