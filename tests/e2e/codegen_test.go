package e2e_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/wearetechnative/nivis/internal/gen"
	"github.com/wearetechnative/nivis/internal/plugin"
)

// TestCodegenAgainstFake runs the codegen end-to-end against the real
// provider-alpha binary, then verifies the generated constructor imports with
// the lib and produces a valid mkResource (hermetic, no network).
func TestCodegenAgainstFake(t *testing.T) {
	requireNix(t)
	root := repoRoot(t)
	buildBinaries(t, root) // puts provider-alpha/beta/epsilon on $PATH

	mgr := plugin.NewManager()
	defer mgr.Close()

	// bare name resolves via $PATH (as `nix shell .#fake-providers` provides it).
	// Codegen uses the schema-only path: GetProviderSchema without Configure.
	client, err := mgr.ClientForSchema("alpha", "provider-alpha")
	if err != nil {
		t.Fatalf("spawn: %v", err)
	}
	resources, err := gen.Fetch(context.Background(), client)
	if err != nil {
		t.Fatalf("fetch schema: %v", err)
	}
	if len(resources) != 1 || resources[0].Type != "alpha_token" {
		t.Fatalf("schema = %+v, want one alpha_token", resources)
	}

	// Emit and write the constructor.
	out := t.TempDir()
	genFile := filepath.Join(out, "alpha_token.nix")
	if err := os.WriteFile(genFile, []byte(gen.Emit("alpha", resources[0])), 0o644); err != nil {
		t.Fatal(err)
	}

	// Evaluate: import the generated constructor with the lib, build a resource,
	// and confirm its id/config are what we expect.
	expr := `
	  let
	    nf = import ` + root + `/nix/lib { };
	    ctor = import ` + genFile + ` { nivis = nf; };
	    r = ctor { name = "A"; label = "hi"; };
	  in builtins.toJSON { id = r.id; config = r.config; }
	`
	cmd := exec.Command("nix", "eval", "--impure", "--raw", "--expr", expr)
	cmd.Dir = root
	res, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("evaluating generated constructor failed: %v\n%s", err, res)
	}
	got := string(res)
	if !contains(got, `"alpha.alpha_token.A"`) {
		t.Errorf("generated resource id wrong: %s", got)
	}
	if !contains(got, `"label":"hi"`) {
		t.Errorf("generated resource config wrong: %s", got)
	}
}

// TestCodegenSkipsConfigure proves the jcpm fix: a provider that REJECTS an
// unconfigured Configure (provider-epsilon, mimicking proxmox/azurerm/google
// validating credentials at configure time) is still extractable by `nivis gen`.
// The schema-only path (ClientForSchema) must succeed; the plan/apply path
// (Client, which configures) must still error — proving Configure is unchanged
// for everything except codegen. Hermetic, no network, no credentials.
func TestCodegenSkipsConfigure(t *testing.T) {
	root := repoRoot(t)
	buildBinaries(t, root) // puts provider-epsilon on $PATH

	// The plan/apply path configures, and epsilon rejects an all-null Configure:
	// this must fail (Configure is unchanged for plan/apply/refresh/destroy).
	mgrCfg := plugin.NewManager()
	defer mgrCfg.Close()
	if _, err := mgrCfg.Client("epsilon", "provider-epsilon", map[string]interface{}{}); err == nil {
		t.Fatal("Client (configuring path) unexpectedly succeeded on the configure-rejecting fake; Configure must still run for plan/apply")
	}

	// The codegen path fetches the schema without configuring: this must succeed.
	mgr := plugin.NewManager()
	defer mgr.Close()
	client, err := mgr.ClientForSchema("epsilon", "provider-epsilon")
	if err != nil {
		t.Fatalf("ClientForSchema on configure-rejecting fake: %v", err)
	}
	resources, err := gen.Fetch(context.Background(), client)
	if err != nil {
		t.Fatalf("fetch schema without configure: %v", err)
	}
	if len(resources) != 1 || resources[0].Type != "epsilon_thing" {
		t.Fatalf("schema = %+v, want one epsilon_thing", resources)
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
