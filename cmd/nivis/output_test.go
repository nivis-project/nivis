// Copyright 2026 TechNative B.V. and the nivis authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"strings"
	"testing"
)

// A buffer is not a *os.File, so newOutput disables color: plain markers, no ANSI.
func TestOutputPlainWhenNotTTY(t *testing.T) {
	var buf bytes.Buffer
	o := newOutput(&buf)
	if o.color {
		t.Fatal("color must be off for a non-TTY writer")
	}
	o.create("aws.aws_s3_bucket.demo", "aws_s3_bucket")
	o.update("aws.aws_instance.web", "aws_instance")
	o.replace("aws.aws_ami.x", "aws_ami")
	o.destroy("aws.aws_s3_object.note", "aws_s3_object")
	o.noop("aws.aws_iam_role.r", "aws_iam_role")
	o.read("data.aws.aws_ami.ubuntu", "aws_ami")

	got := buf.String()
	if strings.Contains(got, "\x1b[") {
		t.Errorf("plain output must contain no ANSI escapes; got %q", got)
	}
	// markers and ids are present and stable regardless of color
	for _, want := range []string{
		"+ aws.aws_s3_bucket.demo (aws_s3_bucket)",
		"~ aws.aws_instance.web",
		"-/+ aws.aws_ami.x",
		"- aws.aws_s3_object.note",
		"= aws.aws_iam_role.r",
		"r data.aws.aws_ami.ubuntu",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
}

// With color forced on, each change type emits an ANSI color.
func TestOutputColoredWhenEnabled(t *testing.T) {
	var buf bytes.Buffer
	o := &output{w: &buf, color: true}
	o.create("a", "")
	o.update("b", "")
	o.replace("c", "")
	o.destroy("d", "")
	o.noop("e", "")
	o.read("f", "")
	got := buf.String()
	for _, code := range []string{ansiGreen, ansiYellow, ansiRed, ansiDim} {
		if !strings.Contains(got, code) {
			t.Errorf("expected ANSI %q in colored output:\n%q", code, got)
		}
	}
	if !strings.Contains(got, ansiReset) {
		t.Error("colored output must reset")
	}
}

// NO_COLOR disables color even on a path that would otherwise enable it.
func TestOutputNoColorEnvDisables(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	// colorEnabled short-circuits on NO_COLOR regardless of writer type.
	if colorEnabled(&bytes.Buffer{}) {
		t.Error("NO_COLOR must disable color")
	}
}

// phaseHeading prints the phase number; plain mode has no ANSI.
func TestOutputPhaseHeading(t *testing.T) {
	var buf bytes.Buffer
	o := newOutput(&buf)
	o.phaseHeading(2)
	got := buf.String()
	if !strings.Contains(got, "Phase 2") {
		t.Errorf("missing phase heading; got %q", got)
	}
	if strings.Contains(got, "\x1b[") {
		t.Errorf("plain phase heading must have no ANSI; got %q", got)
	}
}
