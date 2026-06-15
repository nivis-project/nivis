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
	buildBinaries(t, root)

	bin := filepath.Join(root, "bin", "provider-alpha")
	mgr := plugin.NewManager()
	defer mgr.Close()

	client, err := mgr.Client("alpha", bin, map[string]interface{}{})
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

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
