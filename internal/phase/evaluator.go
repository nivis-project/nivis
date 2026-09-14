// Copyright 2026 TechNative B.V. and the nivis authors
// SPDX-License-Identifier: Apache-2.0

package phase

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/nivis-project/nivis/internal/ledger"
	"github.com/nivis-project/nivis/internal/nixlog"
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
	Attr     string // e.g. "nivis.plan"
	WorkDir  string // dir to run nix in (so a relative flake ref resolves)
	// Term controls how this subprocess reaches the user's terminal.
	Term Terminal
}

// Terminal carries how a subprocess Nivis runs should interact with the user's
// terminal.
//
// Nix produces a perfectly good progress display of its own, and capturing it
// into a buffer that is only read on failure means a build of an
// operating-system image shows nothing for minutes. But Nix's display and a
// live region both drive the cursor, so they can never both be active: hence
// Suspend, which hands the terminal over for the subprocess's duration.
type Terminal struct {
	// Stderr, if set, receives the subprocess's own stderr AS IT RUNS. The
	// stderr is still captured in full regardless, because the error path
	// depends on having all of it (see cleanNixStderr); this is a tee, not a
	// redirect. nil means capture only, as before.
	Stderr io.Writer
	// Suspend, if set, runs fn with the caller's terminal display released.
	// nil runs fn directly.
	Suspend func(fn func() error) error
	// OnBuild, if set, receives the decoded state of a running Nix build as it
	// changes. Setting it makes the subprocess emit its structured event
	// stream, which is read INSTEAD of being shown raw — so the caller's live
	// display can stay up and report the build itself.
	//
	// It is mutually exclusive with Stderr in practice: Stderr hands the
	// terminal to the subprocess, OnBuild consumes it. The verbosity picks one.
	OnBuild func(nixlog.Update)
}

// suspend runs fn with the terminal released, or directly when there is nothing
// to release.
func (t Terminal) suspend(fn func() error) error {
	if t.Suspend == nil {
		return fn()
	}
	return t.Suspend(fn)
}

// tee returns the writer a subprocess's stderr should be copied to alongside
// buf: both, when passthrough is on, and just buf otherwise.
func (t Terminal) tee(buf io.Writer) io.Writer {
	if t.Stderr == nil {
		return buf
	}
	return io.MultiWriter(buf, t.Stderr)
}

func (n NixEval) Eval(ctx context.Context, l *ledger.Ledger) ([]byte, error) {
	data, err := json.Marshal(l)
	if err != nil {
		return nil, fmt.Errorf("nixeval: marshal ledger: %w", err)
	}
	f, err := os.CreateTemp("", "nivis-ledger-*.json")
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

	// Capture stderr in full AND, when passthrough is on, stream it to the user
	// as it arrives. Both matter: the error path below needs the complete text
	// to extract the actionable lines from, and a user watching a slow
	// evaluation needs to see that it is alive.
	var stderr bytes.Buffer
	cmd.Stderr = n.Term.tee(&stderr)

	// cmd.Output() cannot be used once Stderr is set (it refuses, to protect its
	// own capture), so stdout is captured explicitly.
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	if err = n.Term.suspend(cmd.Run); err != nil {
		if _, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("nix evaluation of %s#%s failed:\n%s",
				n.FlakeRef, n.Attr, cleanNixStderr(stderr.String()))
		}
		return nil, fmt.Errorf("running nix eval: %w", err)
	}
	return stdout.Bytes(), nil
}

// nixRealiser is the default Realiser: it makes a __build leaf's output path
// exist by realising the DERIVATION that produces it.
//
// The distinction is the whole point. `nix-store --realise <outputPath>` can
// reuse a path that is already valid, or fetch it from a substituter — but it
// cannot BUILD it, because an output path names a result and is not a recipe:
//
//	$ nix-store --realise /nix/store/<hash>-never-built
//	don't know how to build these paths:
//	  /nix/store/<hash>-never-built
//	error: path '...' is required, but there is no substituter that can build it
//
// Realising the derivation builds it (or substitutes, if the store prefers).
// A leaf with no derivation path comes from a Nix library predating that field;
// for it the old output-path realise is the only option, and when that fails the
// error says why, because the remedy is to update the library rather than
// anything about the user's config.
type nixRealiser struct{ term Terminal }

func (r nixRealiser) Realise(ctx context.Context, b Build) error {
	target, buildable := realiseTarget(b)
	out, err := r.term.run(ctx, "nix-store", "--realise", target)
	if err == nil {
		return nil
	}
	if buildable {
		return fmt.Errorf("nix-store --realise %s (the derivation for %s): %w\n%s",
			target, b.Path, err, cleanNixStderr(out))
	}
	return fmt.Errorf("nix-store --realise %s: %w\n%s\n"+
		"  This __build leaf carries no derivation path, so it can only be substituted, never built:\n"+
		"  an output path names a result, not a recipe. The Nix library that produced this IR predates\n"+
		"  buildable realisation — update your `nivis` flake input (or pre-build the path).",
		target, err, cleanNixStderr(out))
}

// realiseTarget picks what to hand `nix-store --realise` for a build: the
// DERIVATION when the leaf carries one, which can actually be built, otherwise
// the output root, which can only be reused or substituted. buildable reports
// which case it is, so a failure can explain itself.
func realiseTarget(b Build) (target string, buildable bool) {
	if b.Drv != "" {
		return b.Drv, true
	}
	return storeRoot(b.Path), false
}

// run executes a command and returns its combined output.
// run executes a subprocess, capturing its combined output in full and — when
// passthrough is on — streaming it to the user as it arrives. A realise of an
// operating-system image takes minutes and is otherwise entirely silent.
//
// With OnBuild set it instead asks Nix for its structured event stream and
// decodes that, so the build can be reported in the caller's own form. The
// output is still captured in full either way: the error path needs all of it.
func (t Terminal) run(ctx context.Context, name string, args ...string) (string, error) {
	if t.OnBuild != nil {
		return t.runDecoding(ctx, name, args...)
	}
	var buf bytes.Buffer
	cmd := exec.CommandContext(ctx, name, args...)
	w := t.tee(&buf)
	cmd.Stdout = w
	cmd.Stderr = w
	err := t.suspend(cmd.Run)
	return buf.String(), err
}

// maxUndecodable is how many consecutive structured lines may fail to decode
// before we stop trying and pass the rest through raw. A future Nix could
// change the framing; when it does, the user should get Nix's output rather
// than our silence.
const maxUndecodable = 20

// runDecoding runs the subprocess with Nix's structured log format and decodes
// it, reporting build state through OnBuild.
//
// Two properties matter more than the decoding itself:
//
//   - The full output is still captured, because a failing realise must be able
//     to report its actionable error text (see cleanNixStderr).
//   - Decoding can never break the build. A line that is not part of the
//     stream is passed through to the user as-is, and if the structured lines
//     stop being decodable we give up on them and pass everything through. The
//     subprocess is never restarted or interrupted: it is the expensive thing
//     in the run, and re-running an OS image for prettier output would be
//     absurd.
func (t Terminal) runDecoding(ctx context.Context, name string, args ...string) (string, error) {
	args = append(args, "--log-format", "internal-json")

	var buf bytes.Buffer
	pr, pw := io.Pipe()

	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdout = pw
	cmd.Stderr = pw

	done := make(chan struct{})
	go func() {
		defer close(done)
		t.consume(pr, &buf)
	}()

	err := t.suspend(cmd.Run)
	_ = pw.Close()
	<-done
	return buf.String(), err
}

// consume reads the subprocess's output line by line, decoding the structured
// events and capturing everything into buf.
//
// It is separate from the subprocess plumbing on purpose: this is the logic —
// what counts as readable, when to give up, what the user sees — and it must be
// testable without running nix. The coverage gate runs in a sandbox with no nix
// binary, so anything only reachable through a real subprocess is unexercised
// there, including the degradation rule that has already been wrong once.
func (t Terminal) consume(r io.Reader, buf *bytes.Buffer) {
	dec := nixlog.New()
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)

	undecodable, degraded := 0, false
	for sc.Scan() {
		line := sc.Text()
		buf.WriteString(line)
		buf.WriteByte('\n')

		if degraded || !strings.HasPrefix(line, "@nix ") {
			// Not part of the stream (or we have given up on it): ordinary
			// output, and the user's to see. This is also the continuous
			// fallback — if Nix stops framing its output, it simply flows
			// through.
			t.passThrough(line)
			continue
		}

		changed, understood := dec.Line(line)
		if understood {
			// Reset on UNDERSTOOD, not on changed. Most of a real stream is
			// well-formed events that change nothing — one narinfo query emits
			// a dozen identical progress tuples — so counting those as
			// failures degrades before the build even starts, and does it
			// silently, because falling back is not an error.
			undecodable = 0
		} else if undecodable++; undecodable >= maxUndecodable {
			degraded = true
		}
		if changed && t.OnBuild != nil {
			t.OnBuild(dec.Update())
		}
	}
	_, _ = io.Copy(buf, r) // drain anything the scanner rejected
}

// passThrough writes a line to the user, if there is anywhere to put it.
func (t Terminal) passThrough(line string) {
	if t.Stderr == nil {
		return
	}
	_, _ = io.WriteString(t.Stderr, line+"\n")
}

// cleanNixStderr keeps the actionable lines from nix's stderr (the `error:` and
// its indented context) and drops non-actionable noise like the "Git tree is
// dirty" warning, so the user sees the real cause, not Nix's internal verbiage.
func cleanNixStderr(stderr string) string {
	lines := strings.Split(strings.TrimRight(stderr, "\n"), "\n")
	var kept []string
	inError := false
	for _, ln := range lines {
		trimmed := strings.TrimSpace(ln)
		switch {
		case strings.HasPrefix(trimmed, "warning:"):
			inError = false // skip warnings (e.g. dirty tree) and their context
		case strings.HasPrefix(trimmed, "error:"):
			inError = true
			kept = append(kept, ln)
		case inError:
			kept = append(kept, ln) // indented context under an error
		}
	}
	if len(kept) == 0 {
		// Fall back to the raw stderr if we couldn't find an error line.
		return strings.TrimSpace(stderr)
	}
	return strings.Join(kept, "\n")
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

// NixVersion reports the version of Nix on PATH, or "unknown" when it cannot be
// determined. It never fails the run: the version is reported to the user as
// context, and not knowing it is itself worth reporting rather than fatal.
//
// It exists because the event stream this package decodes is an undocumented
// interface. A user on a version Nivis has not been exercised against should be
// able to see that fact from the tool.
func NixVersion(ctx context.Context) string {
	out, err := exec.CommandContext(ctx, "nix", "--version").Output()
	if err != nil {
		return "unknown"
	}
	// "nix (Nix) 2.34.8" -> "2.34.8"
	fields := strings.Fields(string(out))
	if len(fields) == 0 {
		return "unknown"
	}
	return fields[len(fields)-1]
}
