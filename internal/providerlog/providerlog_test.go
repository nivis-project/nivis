// Copyright 2026 TechNative B.V. and the nivis authors
// SPDX-License-Identifier: Apache-2.0

package providerlog_test

// The renderer, tested as the pure bytes-to-lines seam it is: no plugin, no
// subprocess, no provider. The headline fixture is the real-world entry from the
// bug report (bean nixform2-ceoh) that motivated the change.

import (
	"bytes"
	"strings"
	"testing"

	"github.com/hashicorp/go-hclog"
	"github.com/nivis-project/nivis/internal/providerlog"
)

// awsEntry is the VERBATIM real-world entry from the bean, as it reaches our
// logger: go-plugin lifted @level/@message out of the provider's line and passed
// every other field through, appending the original timestamp.
const awsEntry = `{"@level":"warn","@message":"unable to require attribute replacement",` +
	`"@module":"sdk.helper_schema","@caller":"/opt/aws/helper/customdiff/force_new.go:66",` +
	`"error":"ForceNew: No changes for description","tf_attribute_path":"description",` +
	`"tf_mux_provider":"*schema.GRPCProviderServer","tf_provider_addr":"registry.terraform.io/hashicorp/aws",` +
	`"tf_resource_type":"aws_amplify_app","tf_rpc":"PlanResourceChange",` +
	`"tf_req_id":"dd296e15-1f2a-4c7b-9d3e-6a1b2c3d4e5f","timestamp":"2026-08-31T23:58:27.974+0200"}`

// noteFor renders one entry at the given level and returns the printed output.
func noteFor(t *testing.T, level providerlog.Level, provider, entry string) string {
	t.Helper()
	var buf bytes.Buffer
	s := providerlog.NewSink(&buf, level, false)
	s.Note(provider, []byte(entry))
	s.Summary()
	return buf.String()
}

// The headline case: the bean's entry becomes one readable line, the telemetry
// is gone, and the `error` field appears as a detail rather than as an error.
func TestRenderTheReportedAWSEntry(t *testing.T) {
	got := noteFor(t, providerlog.DefaultLevel, "aws", awsEntry)

	if lines := strings.Count(strings.TrimSpace(got), "\n"); lines != 0 {
		t.Errorf("expected exactly one line, got %d:\n%s", lines+1, got)
	}
	for _, want := range []string{
		"provider note",                                // marked as a note, not an error
		"aws_amplify_app.description",                  // subject: type narrowed by attribute
		"unable to require attribute replacement",      // the message
		"detail: ForceNew: No changes for description", // the cause, subordinated
	} {
		if !strings.Contains(got, want) {
			t.Errorf("rendered note lacks %q:\n%s", want, got)
		}
	}
	// The telemetry must be gone, and the note must not read as a failure.
	for _, unwanted := range []string{
		"tf_req_id", "@caller", "tf_mux_provider", "tf_provider_addr", "tf_rpc",
		"dd296e15", "force_new.go", "sdk.helper_schema", "2026-08-31T23:58:27",
		"error=", "error:",
	} {
		if strings.Contains(got, unwanted) {
			t.Errorf("rendered note still contains %q:\n%s", unwanted, got)
		}
	}
}

// Subject derivation: type + attribute, type alone, neither.
func TestSubjectDerivation(t *testing.T) {
	cases := []struct {
		name  string
		entry string
		want  string
	}{
		{
			name:  "type and attribute",
			entry: `{"@level":"warn","@message":"m","tf_resource_type":"aws_s3_bucket","tf_attribute_path":"acl"}`,
			want:  "aws_s3_bucket.acl",
		},
		{
			name:  "type only",
			entry: `{"@level":"warn","@message":"m","tf_resource_type":"aws_s3_bucket"}`,
			want:  "aws_s3_bucket",
		},
		{
			name:  "neither falls back to the provider",
			entry: `{"@level":"warn","@message":"m"}`,
			want:  "aws",
		},
		{
			name:  "attribute without a type falls back to the provider",
			entry: `{"@level":"warn","@message":"m","tf_attribute_path":"acl"}`,
			want:  "aws",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := noteFor(t, providerlog.DefaultLevel, "aws", tc.entry)
			if !strings.Contains(got, tc.want+": m") {
				t.Errorf("subject: got %q, want it to name %q", strings.TrimSpace(got), tc.want)
			}
		})
	}
}

// An error-LEVEL entry is still marked as an error; a warn entry carrying an
// `error` FIELD is not.
func TestErrorLevelVersusErrorField(t *testing.T) {
	warnWithErrField := noteFor(t, providerlog.DefaultLevel, "aws",
		`{"@level":"warn","@message":"considered a replacement","error":"ForceNew: no changes"}`)
	if !strings.Contains(warnWithErrField, "provider note") {
		t.Errorf("a warn entry with an error field must be a note:\n%s", warnWithErrField)
	}
	if strings.Contains(warnWithErrField, "provider error") {
		t.Errorf("a warn entry must not be marked as an error:\n%s", warnWithErrField)
	}
	if !strings.Contains(warnWithErrField, "detail: ForceNew: no changes") {
		t.Errorf("the error field should be a detail:\n%s", warnWithErrField)
	}

	errLevel := noteFor(t, providerlog.LevelError, "aws",
		`{"@level":"error","@message":"the provider failed"}`)
	if !strings.Contains(errLevel, "provider error") {
		t.Errorf("an error-level entry must be marked as an error:\n%s", errLevel)
	}
}

// Rendering is total: nothing about a weird entry drops the line or panics.
func TestRenderingIsTotal(t *testing.T) {
	long := strings.Repeat("x", 8000)
	cases := []struct {
		name  string
		entry string
	}{
		{"not json at all", `this is not json`},
		{"empty", ``},
		{"json but not an object", `["a","b"]`},
		{"no message", `{"@level":"warn","tf_resource_type":"aws_s3_bucket"}`},
		{"no level", `{"@message":"levelless"}`},
		{"numeric field", `{"@level":"warn","@message":"m","count":42,"ratio":0.5}`},
		{"boolean field", `{"@level":"warn","@message":"m","forced":true}`},
		{"null field", `{"@level":"warn","@message":"m","thing":null}`},
		{"nested field", `{"@level":"warn","@message":"m","obj":{"a":[1,2]}}`},
		{"message with newlines", "{\"@level\":\"warn\",\"@message\":\"line one\\nline two\"}"},
		{"very long message", `{"@level":"warn","@message":"` + long + `"}`},
		{"invalid utf8", "{\"@level\":\"warn\",\"@message\":\"bad \xff\xfe byte\"}"},
		{"detail with newlines", "{\"@level\":\"warn\",\"@message\":\"m\",\"error\":\"a\\nb\"}"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := noteFor(t, providerlog.LevelTrace, "aws", tc.entry)
			if tc.entry == "" {
				return // an empty line legitimately renders nothing through Write
			}
			if strings.TrimSpace(got) == "" {
				t.Fatalf("entry produced no output at all")
			}
			// One entry is always one line: a field containing newlines must not
			// break the layout.
			if n := strings.Count(strings.TrimSpace(got), "\n"); n != 0 {
				t.Errorf("entry rendered on %d lines, want 1:\n%s", n+1, got)
			}
		})
	}
}

// Repeats collapse: printed once, counted, reported at the end of the run.
func TestCollapseRepeats(t *testing.T) {
	var buf bytes.Buffer
	s := providerlog.NewSink(&buf, providerlog.DefaultLevel, false)
	for i := 0; i < 12; i++ {
		s.Note("aws", []byte(awsEntry))
	}
	s.Summary()
	got := buf.String()

	if n := strings.Count(got, "unable to require attribute replacement"); n != 2 {
		// once inline, once in the summary
		t.Errorf("message appears %d times, want 2 (one note + one summary):\n%s", n, got)
	}
	if !strings.Contains(got, "11 further occurrence(s) not shown") {
		t.Errorf("summary should report the 11 repeats:\n%s", got)
	}
}

// Notes differing only in their detail are the same note.
func TestCollapseIgnoresDetail(t *testing.T) {
	var buf bytes.Buffer
	s := providerlog.NewSink(&buf, providerlog.DefaultLevel, false)
	s.Note("aws", []byte(`{"@level":"warn","@message":"m","tf_resource_type":"t","error":"cause one"}`))
	s.Note("aws", []byte(`{"@level":"warn","@message":"m","tf_resource_type":"t","error":"cause two"}`))
	s.Summary()
	got := buf.String()

	if strings.Contains(got, "cause two") {
		t.Errorf("the second occurrence should have collapsed:\n%s", got)
	}
	if !strings.Contains(got, "1 further occurrence(s) not shown") {
		t.Errorf("summary should report one repeat:\n%s", got)
	}
}

// Distinct notes are all printed, and nothing is summarised.
func TestDistinctNotesAreNotCollapsed(t *testing.T) {
	var buf bytes.Buffer
	s := providerlog.NewSink(&buf, providerlog.DefaultLevel, false)
	s.Note("aws", []byte(`{"@level":"warn","@message":"first","tf_resource_type":"t"}`))
	s.Note("aws", []byte(`{"@level":"warn","@message":"second","tf_resource_type":"t"}`))
	s.Summary()
	got := buf.String()

	for _, want := range []string{"first", "second"} {
		if !strings.Contains(got, want) {
			t.Errorf("distinct note %q missing:\n%s", want, got)
		}
	}
	if strings.Contains(got, "further occurrence") {
		t.Errorf("nothing repeated, so no summary was warranted:\n%s", got)
	}
}

// Exceeding the distinct-note cap is REPORTED, never silent.
func TestTrackingCapIsReported(t *testing.T) {
	var buf bytes.Buffer
	s := providerlog.NewSink(&buf, providerlog.DefaultLevel, false)
	for i := 0; i < 260; i++ {
		s.Note("aws", []byte(`{"@level":"warn","@message":"note `+string(rune('a'+i%26))+strings.Repeat("x", i)+`"}`))
	}
	s.Summary()
	got := buf.String()

	if !strings.Contains(got, "truncated") {
		t.Errorf("hitting the cap must be reported:\n%s", got[max(0, len(got)-400):])
	}
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// Level gates what is surfaced, and trace restores the dropped fields.
func TestLevelGating(t *testing.T) {
	if got := noteFor(t, providerlog.LevelOff, "aws", awsEntry); got != "" {
		t.Errorf("level off should surface nothing, got:\n%s", got)
	}
	if got := noteFor(t, providerlog.DefaultLevel, "aws", awsEntry); !strings.Contains(got, "provider note") {
		t.Errorf("the default level should surface a warn note:\n%s", got)
	}
	verbatim := noteFor(t, providerlog.LevelTrace, "aws", awsEntry)
	for _, want := range []string{"tf_req_id", "@caller", "tf_rpc", "dd296e15"} {
		if !strings.Contains(verbatim, want) {
			t.Errorf("trace should restore %q:\n%s", want, verbatim)
		}
	}
}

// Level names parse, map onto hclog, and an unknown name is refused by name.
func TestParseLevel(t *testing.T) {
	cases := map[string]struct {
		level providerlog.Level
		hc    hclog.Level
	}{
		"off":   {providerlog.LevelOff, hclog.Off},
		"error": {providerlog.LevelError, hclog.Error},
		"warn":  {providerlog.LevelWarn, hclog.Warn},
		"WARN":  {providerlog.LevelWarn, hclog.Warn},
		" info": {providerlog.LevelInfo, hclog.Info},
		"debug": {providerlog.LevelDebug, hclog.Debug},
		"trace": {providerlog.LevelTrace, hclog.Trace},
	}
	for name, want := range cases {
		got, err := providerlog.ParseLevel(name)
		if err != nil {
			t.Errorf("ParseLevel(%q): %v", name, err)
			continue
		}
		if got != want.level {
			t.Errorf("ParseLevel(%q) = %v, want %v", name, got, want.level)
		}
		if got.HCLog() != want.hc {
			t.Errorf("%q.HCLog() = %v, want %v", name, got.HCLog(), want.hc)
		}
	}

	_, err := providerlog.ParseLevel("verbose")
	if err == nil {
		t.Fatal("an unknown level must be refused")
	}
	for _, want := range []string{"verbose", "off", "error", "warn", "info", "debug", "trace"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the error should name %q: %v", want, err)
		}
	}
	if providerlog.DefaultLevel != providerlog.LevelWarn {
		t.Error("the default level must keep provider warnings visible (design decision 1)")
	}
}

// The Sink's Writer accepts fragmented writes, since an io.Writer contract does
// not promise whole lines.
func TestWriterHandlesFragmentedLines(t *testing.T) {
	var buf bytes.Buffer
	s := providerlog.NewSink(&buf, providerlog.DefaultLevel, false)
	w := s.Writer("aws")

	half := len(awsEntry) / 2
	if _, err := w.Write([]byte(awsEntry[:half])); err != nil {
		t.Fatalf("write first half: %v", err)
	}
	if strings.TrimSpace(buf.String()) != "" {
		t.Errorf("a partial line should not render yet:\n%s", buf.String())
	}
	if _, err := w.Write([]byte(awsEntry[half:] + "\n")); err != nil {
		t.Fatalf("write second half: %v", err)
	}
	if !strings.Contains(buf.String(), "aws_amplify_app.description") {
		t.Errorf("the completed line should render:\n%s", buf.String())
	}
}

// Two providers' notes share one Sink, so collapsing is run-wide.
func TestSinkIsSharedAcrossProviders(t *testing.T) {
	var buf bytes.Buffer
	s := providerlog.NewSink(&buf, providerlog.DefaultLevel, false)
	entry := `{"@level":"warn","@message":"shared","tf_resource_type":"t"}` + "\n"
	if _, err := s.Writer("alpha").Write([]byte(entry)); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Writer("beta").Write([]byte(entry)); err != nil {
		t.Fatal(err)
	}
	s.Summary()
	if n := strings.Count(buf.String(), "shared"); n != 2 {
		t.Errorf("expected one note + one summary line, got %d:\n%s", n, buf.String())
	}
}

// Colour is opt-in and off for piped output, so assertions stay stable.
func TestColorIsOptIn(t *testing.T) {
	plain := noteFor(t, providerlog.DefaultLevel, "aws", awsEntry)
	if strings.Contains(plain, "\x1b[") {
		t.Errorf("color=false must emit no ANSI codes:\n%q", plain)
	}
	var buf bytes.Buffer
	s := providerlog.NewSink(&buf, providerlog.DefaultLevel, true)
	s.Note("aws", []byte(awsEntry))
	if !strings.Contains(buf.String(), "\x1b[") {
		t.Errorf("color=true should emit ANSI codes:\n%q", buf.String())
	}
}

// A plain-text provider line carries its level in a "[WARN]"-style prefix, which
// the transport uses to level the entry and then passes through as part of the
// message. The note must not print the level twice.
func TestPlainTextLevelPrefixIsNotPrintedTwice(t *testing.T) {
	got := noteFor(t, providerlog.DefaultLevel, "zeta",
		`{"@level":"warn","@message":"[WARN] planning with a legacy code path"}`)

	if strings.Contains(got, "[WARN]") {
		t.Errorf("the redundant level marker should be stripped:\n%s", got)
	}
	if !strings.Contains(got, "provider note zeta: planning with a legacy code path") {
		t.Errorf("unexpected rendering:\n%s", got)
	}
}

// When the entry declared no level, the prefix supplies it.
func TestLevelPrefixSuppliesAMissingLevel(t *testing.T) {
	got := noteFor(t, providerlog.DefaultLevel, "zeta", `{"@message":"[ERROR] it broke"}`)
	if !strings.Contains(got, "provider error") {
		t.Errorf("the prefix should have supplied the error level:\n%s", got)
	}
	if strings.Contains(got, "[ERROR]") {
		t.Errorf("the marker should be stripped:\n%s", got)
	}
}
