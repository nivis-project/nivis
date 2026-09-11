// Copyright 2026 TechNative B.V. and the nivis authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"encoding/json"
	"sync"

	"github.com/nivis-project/nivis/internal/ir"
	"github.com/nivis-project/nivis/internal/ledger"
	"github.com/nivis-project/nivis/internal/phase"
)

// Evaluating the configuration is the most expensive step of a run: it spawns a
// Nix process, resolves the flake, and evaluates the whole configuration. Several
// consumers need the SAME phase-0 evaluation — backend discovery (openStore), the
// destroy/refresh engines' resource graph, `state migrate`'s backend lookup, and
// the phase loop's own phase 0 — and each used to pay for its own `nix eval` with
// identical inputs. The cost lands before any output appears, so it presented as
// dead air at the start of every command.
//
// Two caches, at two levels, because consumers want two different things:
//
//   - evalCache memoizes the raw IR JSON per LEDGER, so anything evaluating with
//     the same ledger shares one subprocess. This is what makes the phase loop's
//     phase 0 free once backend discovery has already evaluated.
//   - configGraph memoizes the INGESTED phase-0 graph, so the consumers that want
//     a graph (destroy, refresh, state migrate, openStore) also skip re-ingesting.
//
// Both cache the ERROR as well as the value. A failed evaluation must not be
// retried once per consumer, and each consumer keeps its existing handling of that
// failure: openStore falls back to the local store, while a command that needs the
// configuration surfaces the error. Sharing one evaluation must not turn a
// tolerated failure into a fatal one, or the reverse.
//
// Sharing is also a correctness property, not only a saving. The evaluation is
// `--impure`, so two evaluations within one run can legitimately disagree; one
// snapshot means a run sees one configuration.

// evalResult is one memoized evaluation: the IR JSON, or the error that evaluation
// produced.
type evalResult struct {
	irJSON []byte
	err    error
}

// evalCache is a phase.NixEvaluator that memoizes its delegate per ledger. The
// ledger's JSON is the key because it is exactly what is handed to Nix: identical
// JSON means identical evaluation inputs.
type evalCache struct {
	inner phase.NixEvaluator

	mu       sync.Mutex
	byLedger map[string]evalResult
}

func (c *evalCache) Eval(ctx context.Context, l *ledger.Ledger) ([]byte, error) {
	key, err := json.Marshal(l)
	if err != nil {
		// A ledger that will not marshal cannot be keyed. Delegate so the
		// evaluator reports the real failure rather than this one.
		return c.inner.Eval(ctx, l)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if r, ok := c.byLedger[string(key)]; ok {
		return r.irJSON, r.err
	}
	irJSON, evalErr := c.inner.Eval(ctx, l)
	c.byLedger[string(key)] = evalResult{irJSON: irJSON, err: evalErr}
	return irJSON, evalErr
}

// The run-scoped caches. A production process runs exactly one command, so "per
// run" and "per process" coincide; tests share a process and reset them.
var (
	runEvalOnce sync.Once
	runEvalV    *evalCache

	configGraphOnce sync.Once
	configGraphV    *ir.Graph
	configGraphErr  error
)

// runEval returns the run's shared evaluator. It is built lazily so that the
// --flake and --attr flags are parsed before their values are captured.
func runEval() phase.NixEvaluator {
	runEvalOnce.Do(func() {
		runEvalV = &evalCache{
			inner:    phase.NixEval{FlakeRef: flakeRef, Attr: attr, WorkDir: ""},
			byLedger: map[string]evalResult{},
		}
	})
	return runEvalV
}

// configGraph returns the run's single phase-0 graph, evaluating and ingesting it
// at most once however many consumers ask. Every caller that needs the
// configuration's phase-0 shape SHOULD use this rather than calling graphFn or
// phase0Graph directly — calling those directly is what made destroy and refresh
// evaluate twice.
func configGraph(ctx context.Context) (*ir.Graph, error) {
	configGraphOnce.Do(func() { configGraphV, configGraphErr = graphFn(ctx) })
	return configGraphV, configGraphErr
}

// resetRunCache clears the per-run caches. Production never calls it: a process
// runs one command and exits. Tests share a process, so without it an evaluation
// (or a swapped graphFn's graph, or a --flake value) leaks from one test into the
// next.
func resetRunCache() {
	runEvalOnce = sync.Once{}
	runEvalV = nil
	configGraphOnce = sync.Once{}
	configGraphV, configGraphErr = nil, nil
}
