// Copyright 2026 TechNative B.V. and the nivis authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestFormatValue(t *testing.T) {
	if got := formatValue("hi"); got != "hi" {
		t.Errorf("string should pass through, got %q", got)
	}
	if got := formatValue(float64(3)); got != "3" {
		t.Errorf("number = %q, want 3", got)
	}
	if got := formatValue(true); got != "true" {
		t.Errorf("bool = %q, want true", got)
	}
	if got := formatValue([]interface{}{"a", "b"}); got != `["a","b"]` {
		t.Errorf("list = %q, want compact JSON", got)
	}
}

func TestWriteJSON(t *testing.T) {
	var buf bytes.Buffer
	if err := writeJSON(&buf, map[string]interface{}{"url": "x"}); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, `"url": "x"`) {
		t.Errorf("json out = %q", out)
	}
	if !strings.HasSuffix(out, "\n") {
		t.Error("json output should end with a newline")
	}
}
