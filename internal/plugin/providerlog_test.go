// Copyright 2026 TechNative B.V. and the nivis authors
// SPDX-License-Identifier: Apache-2.0

package plugin_test

// The whole provider-log path, through a real spawned provider: provider stderr
// -> the plugin transport's parse -> the manager's logger -> the renderer. The
// log-emitting fake (cmd/provider-zeta) is the trigger; nothing here is
// simulated except the provider itself.

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/nivis-project/nivis/internal/plan"
	"github.com/nivis-project/nivis/internal/plugin"
	"github.com/nivis-project/nivis/internal/providerlog"
)

// planZeta spawns the log-emitting fake through a manager wired to a sink at the
// given level, plans its resource once, and returns everything the sink printed.
func planZeta(t *testing.T, level providerlog.Level) string {
	t.Helper()
	bin := buildProvider(t, "provider-zeta")

	var notes bytes.Buffer
	sink := providerlog.NewSink(&notes, level, false)
	mgr := plugin.NewManager().WithProviderLog(sink, level)
	defer mgr.Close()

	client, err := mgr.Client("zeta", bin, map[string]interface{}{})
	if err != nil {
		t.Fatalf("spawn provider-zeta: %v", err)
	}
	schema, err := client.GetSchema(context.Background(), "zeta_note")
	if err != nil {
		t.Fatalf("GetSchema: %v", err)
	}
	n := node("zeta.zeta_note.a", "zeta_note", "a", map[string]interface{}{"label": "x"})
	if _, err := plan.Plan(context.Background(), client, schema, n, n.Resource.Config, nil); err != nil {
		t.Fatalf("plan: %v", err)
	}
	mgr.Close() // flush: the provider's stderr is drained as the process goes away
	sink.Summary()
	return notes.String()
}

// At the default level the provider's benign warning arrives as ONE readable
// note, with the telemetry gone and the `error` field subordinated — and the
// plan itself succeeds.
func TestSpawnedProviderWarningIsRenderedAsANote(t *testing.T) {
	got := planZeta(t, providerlog.DefaultLevel)

	for _, want := range []string{
		"provider note",
		"zeta_note.description",
		"unable to require attribute replacement",
		"detail: ForceNew: No changes for description",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("rendered output lacks %q:\n%s", want, got)
		}
	}
	// The provider's internal telemetry must not reach the user.
	for _, unwanted := range []string{
		"tf_req_id", "dd296e15", "@caller", "force_new.go", "tf_mux_provider",
		"tf_provider_addr", "sdk.helper_schema", "error=",
	} {
		if strings.Contains(got, unwanted) {
			t.Errorf("rendered output still carries %q:\n%s", unwanted, got)
		}
	}
	// The plain-text route (level from a "[WARN]" prefix, no fields) also lands.
	if !strings.Contains(got, "planning with a legacy code path") {
		t.Errorf("the prefixed plain-text line did not arrive:\n%s", got)
	}
}

// Nothing reaches a terminal raw: every line the sink saw is a rendered note,
// never the provider's own entry. (ClientConfig.Stderr stays io.Discard, which
// is what makes the logger the only route.)
func TestNoRawProviderLineEscapesTheRenderer(t *testing.T) {
	got := planZeta(t, providerlog.DefaultLevel)

	for _, line := range strings.Split(strings.TrimSpace(got), "\n") {
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "{") || strings.Contains(line, `"@level"`) {
			t.Errorf("a raw provider entry escaped the renderer: %q", line)
		}
		if !strings.HasPrefix(line, "provider ") {
			t.Errorf("unrendered line reached the output: %q", line)
		}
	}
}

// The level governs what is surfaced: error hides the warn note entirely, trace
// restores the fields a note drops.
func TestSpawnedProviderLogLevels(t *testing.T) {
	if got := planZeta(t, providerlog.LevelError); strings.Contains(got, "unable to require") {
		t.Errorf("at error level the warn note should not surface:\n%s", got)
	}
	verbatim := planZeta(t, providerlog.LevelTrace)
	for _, want := range []string{"tf_req_id", "dd296e15", "tf_rpc"} {
		if !strings.Contains(verbatim, want) {
			t.Errorf("trace should restore %q:\n%s", want, verbatim)
		}
	}
}

// A manager with no sink discards provider logs rather than printing them.
func TestManagerWithoutSinkDiscardsProviderLogs(t *testing.T) {
	bin := buildProvider(t, "provider-zeta")
	mgr := plugin.NewManager() // no WithProviderLog
	defer mgr.Close()

	client, err := mgr.Client("zeta", bin, map[string]interface{}{})
	if err != nil {
		t.Fatalf("spawn: %v", err)
	}
	if _, err := client.GetSchema(context.Background(), "zeta_note"); err != nil {
		t.Fatalf("GetSchema: %v", err)
	}
	// Nothing to assert on output (there is none by construction); the point is
	// that a nil sink is not a nil-pointer panic.
}

// The existing fakes stay silent, so tests asserting exact CLI output are not
// destabilised by stray notes.
func TestOtherFakesEmitNoNotes(t *testing.T) {
	bin := buildProvider(t, "provider-alpha")

	var notes bytes.Buffer
	sink := providerlog.NewSink(&notes, providerlog.DefaultLevel, false)
	mgr := plugin.NewManager().WithProviderLog(sink, providerlog.DefaultLevel)
	defer mgr.Close()

	client, err := mgr.Client("alpha", bin, map[string]interface{}{})
	if err != nil {
		t.Fatalf("spawn provider-alpha: %v", err)
	}
	schema, err := client.GetSchema(context.Background(), "alpha_token")
	if err != nil {
		t.Fatalf("GetSchema: %v", err)
	}
	n := node("alpha.alpha_token.a", "alpha_token", "a", map[string]interface{}{})
	if _, err := plan.Plan(context.Background(), client, schema, n, n.Resource.Config, nil); err != nil {
		t.Fatalf("plan: %v", err)
	}
	mgr.Close()
	sink.Summary()

	if strings.TrimSpace(notes.String()) != "" {
		t.Errorf("provider-alpha should be silent, got:\n%s", notes.String())
	}
}
