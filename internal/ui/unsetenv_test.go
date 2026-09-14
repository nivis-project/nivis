// Copyright 2026 TechNative B.V. and the nivis authors
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"os"
	"testing"
)

// unsetEnvForTest removes a variable for the duration of the test. t.Setenv
// cannot express "absent" — setting a variable to "" still leaves it PRESENT,
// which NO_COLOR explicitly treats as enabled.
func unsetEnvForTest(t *testing.T, key string) {
	t.Helper()
	old, had := os.LookupEnv(key)
	if err := os.Unsetenv(key); err != nil {
		t.Fatalf("unset %s: %v", key, err)
	}
	t.Cleanup(func() {
		if had {
			_ = os.Setenv(key, old)
		} else {
			_ = os.Unsetenv(key)
		}
	})
}

// errTest is a sentinel for error-propagation assertions.
var errTest = errSentinel{}

type errSentinel struct{}

func (errSentinel) Error() string { return "test sentinel" }
