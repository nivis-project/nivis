// Copyright 2026 TechNative B.V. and the nivis authors
// SPDX-License-Identifier: Apache-2.0

// Package progress is the contract by which the engines report what they are
// doing while they do it.
//
// The engines emit FACTS — what started, what it was, what it resolved as, how
// long it took — and never presentation text. That split is the whole point: a
// verbosity level cannot filter `"Building foo for bar...\n"` without re-parsing
// prose, and a renderer cannot colour it without knowing which substring is the
// identity. Everything about wording, colour, terminal capability and verbosity
// belongs to the renderer in cmd/nivis, on the other side of this package.
//
// The zero value of an Observer is nil, and nil is safe: Emit is a free function
// that tolerates it, so an engine constructed without an observer behaves exactly
// as it did before this package existed.
package progress

import (
	"time"

	"github.com/nivis-project/nivis/internal/plan"
)

// Kind identifies what an Event reports. Fields on Event are meaningful only for
// the kinds documented against them.
type Kind int

const (
	// EvalStart / EvalDone bracket one Nix evaluation of the configuration.
	// Phase is the phase being evaluated for. EvalDone carries Duration and Err.
	EvalStart Kind = iota
	EvalDone

	// PhaseStart opens a phase of the fixpoint loop. Phase is its number
	// (0-based) and Count is how many nodes are ready in it — known before the
	// phase runs, unlike the run's grand total, which cannot be known in advance
	// because later phases reveal resources phase 0 cannot see.
	PhaseStart

	// NodeStart / NodeDone bracket one node being applied, read, destroyed or
	// refreshed. ID and Type identify it; Op is its resolved operation and IsData
	// distinguishes a datasource READ from a resource apply. NodeDone carries
	// Duration, Err, the provider-assigned ResourceID where there is one, and
	// Drifted for a refresh that found the real resource had changed.
	NodeStart
	NodeDone

	// BuildStart / BuildDone bracket realising one __build leaf. Name is the
	// readable store-path name, Owner the resource it is built for, and Path the
	// output path. BuildDone carries Duration and Err.
	BuildStart
	BuildDone

	// BuildProgress reports how a build that is already running is getting on:
	// which derivation is building now, how many are done of how many expected,
	// and the latest line of the build's own output. It arrives repeatedly
	// while a single BuildStart/BuildDone pair is open.
	//
	// Expected MOVES: Nix discovers work as it proceeds, so the total rises and
	// shifts as a running derivation is counted or not. A renderer shows what
	// it currently says rather than treating the first figure as final.
	BuildProgress

	// ProviderSpawn reports a provider process being started. Name is its
	// identity. Fires once per identity, since the manager pools them.
	ProviderSpawn

	// Note is a free-form line from a producer that has already rendered its own
	// text — today only the provider-log sink, whose collapsing and level
	// filtering happen before it reaches here. Message carries the text.
	//
	// It exists so that such a producer does not write to the terminal itself: a
	// live region rewrites lines in place, and an independent writer emitting a
	// line mid-redraw corrupts it with no way to recover, because the renderer's
	// model of what is on screen is then wrong.
	Note
)

// Event is one reported fact. It is a single struct rather than a sum type so
// that a renderer can filter by Kind and a future serialiser can marshal it
// without a type switch.
type Event struct {
	Kind Kind

	Phase int    // EvalStart, EvalDone, PhaseStart
	Count int    // PhaseStart: nodes ready this phase
	ID    string // NodeStart, NodeDone: the resource id
	Type  string // NodeStart, NodeDone: the resource type, where known

	Op     plan.Op // NodeStart, NodeDone: meaningful only when IsData is false
	IsData bool    // NodeStart, NodeDone: the node is a datasource READ

	Name  string // BuildStart/BuildDone: store-path name. ProviderSpawn: identity.
	Owner string // BuildStart, BuildDone: the resource the build is for
	Path  string // BuildStart, BuildDone: the output path

	ResourceID string // NodeDone: the provider-assigned id, where there is one
	Drifted    bool   // NodeDone: a refresh found the real resource had changed
	// Refresh marks a node event as a REFRESH read rather than an apply. The
	// distinction is not cosmetic: an unchanged apply is worth a line, while an
	// unchanged refresh is not — reporting forty unchanged reads buries the
	// three that drifted. A renderer cannot infer this from the other fields,
	// so it is stated.
	Refresh bool

	// BuildProgress fields.
	Derivation string // the derivation building now, by readable name
	Done       int    // derivations finished
	Expected   int    // derivations expected — this figure can change
	LastLine   string // the latest line of the build's own output

	Message string // Note

	Duration time.Duration // the *Done kinds
	Err      error         // the *Done kinds; nil on success
}

// Observer receives events as they happen. Implementations must tolerate being
// called from the engine's goroutine and must not block for long: an engine
// emits between units of real work.
type Observer interface {
	Emit(Event)
}

// Emit reports e to obs, tolerating a nil observer. Engines call this rather than
// obs.Emit so that "no observer" needs no nil check at every emit site.
func Emit(obs Observer, e Event) {
	if obs == nil {
		return
	}
	obs.Emit(e)
}

// Func adapts a plain function to Observer.
type Func func(Event)

func (f Func) Emit(e Event) { f(e) }
