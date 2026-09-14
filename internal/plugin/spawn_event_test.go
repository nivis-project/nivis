// Copyright 2026 TechNative B.V. and the nivis authors
// SPDX-License-Identifier: Apache-2.0

package plugin_test

import (
	"testing"

	"github.com/nivis-project/nivis/internal/plugin"
	"github.com/nivis-project/nivis/internal/progress"
)

// A provider spawn is a wait worth reporting: a registry-backed source is
// fetched and verified before the process runs, and none of that produces
// output of its own.
//
// It must fire once per IDENTITY, not once per resource: the manager pools
// clients, so a stack of fifty resources on one provider spawns one process. A
// per-call event would turn a single spawn into fifty lines.
func TestProviderSpawnFiresOncePerIdentity(t *testing.T) {
	bin := buildProvider(t, "provider-alpha")

	var spawns []string
	obs := progress.Func(func(e progress.Event) {
		if e.Kind == progress.ProviderSpawn {
			spawns = append(spawns, e.Name)
		}
	})

	mgr := plugin.NewManager().WithObserver(obs)
	defer mgr.Close()

	for i := 0; i < 3; i++ {
		if _, err := mgr.Client("alpha", bin, map[string]interface{}{}); err != nil {
			t.Fatal(err)
		}
	}

	if len(spawns) != 1 {
		t.Fatalf("spawn events = %d (%v), want 1 — the manager pools by identity", len(spawns), spawns)
	}
	if spawns[0] != "alpha" {
		t.Errorf("spawn event names %q, want the provider identity %q", spawns[0], "alpha")
	}
}

// Two identities are two processes, so two events.
func TestProviderSpawnFiresPerDistinctIdentity(t *testing.T) {
	alpha := buildProvider(t, "provider-alpha")
	beta := buildProvider(t, "provider-beta")

	var spawns []string
	obs := progress.Func(func(e progress.Event) {
		if e.Kind == progress.ProviderSpawn {
			spawns = append(spawns, e.Name)
		}
	})

	mgr := plugin.NewManager().WithObserver(obs)
	defer mgr.Close()

	if _, err := mgr.Client("alpha", alpha, map[string]interface{}{}); err != nil {
		t.Fatal(err)
	}
	if _, err := mgr.Client("beta", beta, map[string]interface{}{}); err != nil {
		t.Fatal(err)
	}

	if len(spawns) != 2 {
		t.Fatalf("spawn events = %d (%v), want 2", len(spawns), spawns)
	}
}

// A manager without an observer must behave exactly as before.
func TestManagerWithoutObserver(t *testing.T) {
	bin := buildProvider(t, "provider-alpha")
	mgr := plugin.NewManager()
	defer mgr.Close()
	if _, err := mgr.Client("alpha", bin, map[string]interface{}{}); err != nil {
		t.Fatal(err)
	}
}
