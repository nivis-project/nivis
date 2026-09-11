// Copyright 2026 TechNative B.V. and the nivis authors
// SPDX-License-Identifier: Apache-2.0

package main

// Tests for the per-run evaluation caches. The point of the cache is a count:
// several consumers need the SAME phase-0 evaluation, and before it each paid for
// its own `nix eval`. These tests pin the count, because nothing else would
// notice a regression — the results are identical either way, only the latency
// differs.

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"

	"github.com/nivis-project/nivis/internal/ir"
	"github.com/nivis-project/nivis/internal/ledger"
)

// countingEval records how many times it was asked to evaluate, and what it
// returns. It stands in for `nix eval`.
type countingEval struct {
	mu     sync.Mutex
	calls  int
	byCall []int // the ledger phase of each call, in order
	irJSON []byte
	err    error
}

func (c *countingEval) Eval(_ context.Context, l *ledger.Ledger) ([]byte, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.calls++
	c.byCall = append(c.byCall, l.Phase)
	return c.irJSON, c.err
}

// withCountingEval installs a counting evaluator behind the run's cache and
// returns it. It resets the run caches on both sides so the count is this test's.
func withCountingEval(t *testing.T, irJSON []byte, err error) *countingEval {
	t.Helper()
	resetRunCache()
	inner := &countingEval{irJSON: irJSON, err: err}
	runEvalOnce.Do(func() {}) // consume the Once so runEval keeps our value
	runEvalV = &evalCache{inner: inner, byLedger: map[string]evalResult{}}
	t.Cleanup(resetRunCache)
	return inner
}

// minimalIR is the smallest IR that ingests, with no resources.
const minimalIR = `{"schemaVersion":1,"providers":{},"resources":[]}`

// The core of the fix: openStore's backend discovery and the phase loop's phase 0
// are the same evaluation, and must cost one `nix eval`, not two.
func TestPhase0IsEvaluatedOnceAcrossConsumers(t *testing.T) {
	inner := withCountingEval(t, []byte(minimalIR), nil)

	ctx := context.Background()
	// Consumer 1: backend discovery.
	if _, err := configGraph(ctx); err != nil {
		t.Fatalf("configGraph: %v", err)
	}
	// Consumer 2: the phase loop's phase 0, which evaluates with an equal ledger.
	l, err := newLedger()
	if err != nil {
		t.Fatalf("newLedger: %v", err)
	}
	if _, err := evaluator().Eval(ctx, l); err != nil {
		t.Fatalf("Eval: %v", err)
	}

	if inner.calls != 1 {
		t.Errorf("evaluations = %d, want 1 (backend discovery and phase 0 share one eval); phases seen: %v",
			inner.calls, inner.byCall)
	}
}

// configGraph is what destroy/refresh/state-migrate call. Asking repeatedly must
// not re-evaluate: destroy used to call phase0Graph directly and then openStore
// called it again.
func TestConfigGraphIsMemoized(t *testing.T) {
	inner := withCountingEval(t, []byte(minimalIR), nil)

	ctx := context.Background()
	first, err := configGraph(ctx)
	if err != nil {
		t.Fatalf("configGraph: %v", err)
	}
	second, err := configGraph(ctx)
	if err != nil {
		t.Fatalf("configGraph (again): %v", err)
	}

	if inner.calls != 1 {
		t.Errorf("evaluations = %d, want 1", inner.calls)
	}
	if first != second {
		t.Error("configGraph returned a different graph the second time; it must serve one snapshot")
	}
}

// A failed evaluation must be cached too, or each consumer retries it — turning
// one slow failure into several.
func TestFailedEvaluationIsNotRetriedPerConsumer(t *testing.T) {
	want := errors.New("no flake here")
	inner := withCountingEval(t, nil, want)

	ctx := context.Background()
	if _, err := configGraph(ctx); !errors.Is(err, want) {
		t.Fatalf("configGraph error = %v, want %v", err, want)
	}
	if _, err := configGraph(ctx); !errors.Is(err, want) {
		t.Fatalf("configGraph error (again) = %v, want %v", err, want)
	}

	if inner.calls != 1 {
		t.Errorf("evaluations = %d, want 1 (the failure is cached, not retried)", inner.calls)
	}
}

// Sharing applies only to IDENTICAL inputs. A later phase carries a different
// ledger and must evaluate again, or the fixpoint loop could never advance.
func TestLaterPhasesStillEvaluate(t *testing.T) {
	inner := withCountingEval(t, []byte(minimalIR), nil)

	ctx := context.Background()
	l, err := newLedger()
	if err != nil {
		t.Fatalf("newLedger: %v", err)
	}
	for phase := 0; phase < 3; phase++ {
		l.Phase = phase
		if _, err := evaluator().Eval(ctx, l); err != nil {
			t.Fatalf("phase %d: %v", phase, err)
		}
	}
	// And phase 0 again: that one IS a repeat and must be served from the cache.
	l.Phase = 0
	if _, err := evaluator().Eval(ctx, l); err != nil {
		t.Fatalf("phase 0 repeat: %v", err)
	}

	if inner.calls != 3 {
		t.Errorf("evaluations = %d, want 3 (one per distinct ledger); phases seen: %v",
			inner.calls, inner.byCall)
	}
}

// The ledger's accumulated outputs are part of the key, not just its phase
// number: two evaluations at the same phase with different outputs are different
// evaluations.
func TestLedgerOutputsArePartOfTheKey(t *testing.T) {
	inner := withCountingEval(t, []byte(minimalIR), nil)

	ctx := context.Background()
	l, err := newLedger()
	if err != nil {
		t.Fatalf("newLedger: %v", err)
	}
	if _, err := evaluator().Eval(ctx, l); err != nil {
		t.Fatalf("Eval: %v", err)
	}
	l.Append("alpha_token.a", map[string]interface{}{"id": "a"})
	if _, err := evaluator().Eval(ctx, l); err != nil {
		t.Fatalf("Eval (with outputs): %v", err)
	}

	if inner.calls != 2 {
		t.Errorf("evaluations = %d, want 2 (differing outputs are differing inputs)", inner.calls)
	}
}

// The tolerated-failure path: a directory with no evaluable configuration must
// still yield the local store, exactly as before the cache.
func TestUnevaluableConfigStillFallsBackToLocalStore(t *testing.T) {
	withCountingEval(t, nil, errors.New("no flake here"))
	statePath = t.TempDir() + "/nivis.state.json"
	t.Cleanup(func() { statePath = "./nivis.state.json" })

	store, err := openStore(context.Background(), &writerSink{})
	if err != nil {
		t.Fatalf("openStore fell through to an error; it must fall back to local: %v", err)
	}
	if store == nil {
		t.Fatal("openStore returned no store")
	}
	if _, err := store.List(); err != nil {
		t.Fatalf("the fallback store is unusable: %v", err)
	}
}

// The fatal-failure path: a consumer that NEEDS the configuration must still see
// the evaluation error, even though openStore tolerated the same one.
func TestConsumerThatNeedsTheConfigStillSeesTheError(t *testing.T) {
	want := errors.New("attribute 'nivis.plan' missing")
	withCountingEval(t, nil, want)
	statePath = t.TempDir() + "/nivis.state.json"
	t.Cleanup(func() { statePath = "./nivis.state.json" })

	ctx := context.Background()
	// openStore tolerates it...
	if _, err := openStore(ctx, &writerSink{}); err != nil {
		t.Fatalf("openStore must tolerate the failure, got: %v", err)
	}
	// ...and configGraph, which destroy/refresh use and cannot proceed without,
	// still reports it.
	if _, err := configGraph(ctx); !errors.Is(err, want) {
		t.Errorf("configGraph error = %v, want %v", err, want)
	}
}

// writerSink discards the announcement openStore may write.
type writerSink struct{}

func (*writerSink) Write(p []byte) (int, error) { return len(p), nil }

// Guard the assumption the cache key rests on: an ir.Graph ingested from the
// minimal IR is usable, so a test asserting counts is not accidentally asserting
// on a parse failure.
func TestMinimalIRIngests(t *testing.T) {
	g, err := ir.IngestIR([]byte(minimalIR))
	if err != nil {
		t.Fatalf("IngestIR: %v", err)
	}
	if g == nil {
		t.Fatal("IngestIR returned no graph")
	}
}

// The sharing rests on one invariant: the ledger backend discovery evaluates with
// is byte-identical to the one the phase loop hands the evaluator at phase 0. If
// a future change builds either differently, the cache silently stops hitting and
// the duplicate evaluation returns — with no test failing and nothing visible but
// a slower run. This test is the alarm for that.
func TestPhase0LedgersAreIdenticalAcrossConsumers(t *testing.T) {
	// The ledger phase0Graph evaluates with (backend discovery).
	discovery, err := newLedger()
	if err != nil {
		t.Fatalf("newLedger: %v", err)
	}
	// The ledger applyCmd builds and the driver evaluates with at phase 0: the
	// driver sets Phase = phaseNum before each eval, which is 0 for the first.
	loop, err := newLedger()
	if err != nil {
		t.Fatalf("newLedger: %v", err)
	}
	loop.Phase = 0

	a, err := json.Marshal(discovery)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	b, err := json.Marshal(loop)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if string(a) != string(b) {
		t.Errorf("phase-0 ledgers differ, so the two consumers will not share an evaluation:\n"+
			" discovery: %s\n phase loop: %s", a, b)
	}
}
