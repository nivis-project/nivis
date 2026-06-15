package phase

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/wearetechnative/nixform/internal/ledger"
)

// NixEvaluator produces the IR JSON for a given outputs ledger. The real impl
// shells out to `nix eval`; tests inject a deterministic stub. This is the seam
// that lets the loop logic be unit-tested hermetically while the integration
// test exercises real Nix.
type NixEvaluator interface {
	Eval(ctx context.Context, l *ledger.Ledger) ([]byte, error)
}

// NixEval evaluates `<FlakeRef>#<Attr>` as a function of the ledger:
//
//	nix eval <flake>#<attr> \
//	  --apply 'p: p (builtins.fromJSON (builtins.readFile <ledgerFile>))' \
//	  --json --impure
//
// The ledger is written to a temp 0600 file (it may carry sensitive outputs, so
// it never goes on the command line or into the store).
type NixEval struct {
	FlakeRef string // e.g. "." or "/path/to/repo"
	Attr     string // e.g. "nixform.plan"
	WorkDir  string // dir to run nix in (so a relative flake ref resolves)
}

func (n NixEval) Eval(ctx context.Context, l *ledger.Ledger) ([]byte, error) {
	data, err := json.Marshal(l)
	if err != nil {
		return nil, fmt.Errorf("nixeval: marshal ledger: %w", err)
	}
	f, err := os.CreateTemp("", "nixform-ledger-*.json")
	if err != nil {
		return nil, fmt.Errorf("nixeval: temp ledger: %w", err)
	}
	defer os.Remove(f.Name())
	if err := os.Chmod(f.Name(), 0o600); err != nil {
		return nil, fmt.Errorf("nixeval: chmod ledger: %w", err)
	}
	if _, err := f.Write(data); err != nil {
		f.Close()
		return nil, fmt.Errorf("nixeval: write ledger: %w", err)
	}
	f.Close()

	apply := fmt.Sprintf("p: p (builtins.fromJSON (builtins.readFile %s))", absPath(f.Name()))
	cmd := exec.CommandContext(ctx, "nix", "eval",
		n.FlakeRef+"#"+n.Attr,
		"--apply", apply,
		"--json", "--impure",
	)
	cmd.Dir = n.WorkDir
	out, err := cmd.Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("nixeval: nix eval failed: %w\nstderr: %s", err, ee.Stderr)
		}
		return nil, fmt.Errorf("nixeval: %w", err)
	}
	return out, nil
}

func absPath(p string) string {
	if abs, err := filepath.Abs(p); err == nil {
		return abs
	}
	return p
}

// StubEvaluator returns canned IR per phase, for hermetic loop tests. It selects
// the IR by how many resources are already in the ledger (i.e. how far the loop
// has progressed), letting a test model "re-eval resolves more each phase".
type StubEvaluator struct {
	// IRForLedger maps a count of known resources -> the IR JSON to return when
	// that many resources have outputs in the ledger.
	IRForLedger func(l *ledger.Ledger) []byte
	Calls       int
}

func (s *StubEvaluator) Eval(_ context.Context, l *ledger.Ledger) ([]byte, error) {
	s.Calls++
	return s.IRForLedger(l), nil
}
