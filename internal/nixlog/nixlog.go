// Copyright 2026 TechNative B.V. and the nivis authors
// SPDX-License-Identifier: Apache-2.0

// Package nixlog decodes the event stream Nix emits under
// `--log-format internal-json`, so a build can be reported in Nivis's own form
// instead of being silent for minutes.
//
// # The interface this reads is not a contract
//
// Nix does not document `internal-json` as a stable interface. Its payload is
// POSITIONAL and UNNAMED, and — the trap — an event's meaning depends on the
// `action` and the `type` TOGETHER, never the type alone:
//
//	{"action":"start", "type":105, ...}   a build is starting
//	{"action":"result","type":105, ...}   a progress report
//	{"action":"start", "type":101, ...}   a file is downloading
//	{"action":"result","type":101, ...}   a line of a build's output
//
// Nothing in the payload says so. A decoder that switches on `type` alone is
// wrong in a way that looks like it works.
//
// # Degrade, never fail
//
// The build is the user's actual work; this is decoration. An unrecognised
// event is ignored, a malformed line is skipped, and the caller is expected to
// fall back to passing the subprocess's bytes through raw if the stream stops
// making sense. Nothing here returns an error that could abort a build.
package nixlog

import (
	"encoding/json"
	"path"
	"strings"
)

// prefix marks a line of the structured stream. Anything without it is ordinary
// output and is not ours to interpret.
const prefix = "@nix "

// action values.
const (
	actionStart  = "start"
	actionResult = "result"
	actionStop   = "stop"
	actionMsg    = "msg"
)

// Activity types, as they appear on a `start`. From Nix's internal ActivityType
// enum; only the ones we act on are named.
const (
	actBuild  = 105 // fields[0] is the .drv being built
	actBuilds = 104 // the aggregate whose progress is the build total
)

// Result types, as they appear on a `result`. From Nix's internal ResultType
// enum. Note these SHARE VALUES with the activity types above and are a
// different enum entirely.
const (
	resBuildLogLine = 101 // fields[0] is one line of a build's output
	resProgress     = 105 // fields are [done, expected, running, failed]
)

// Update is what a decoded stream currently says about a build. It is a
// snapshot, not an event: the caller reports whatever the latest one holds.
type Update struct {
	// Building is the readable name of the derivation being built, empty when
	// none is.
	Building string
	// Done and Expected count derivations across the whole realise. Expected
	// MOVES — Nix discovers work as it proceeds, and the total was observed to
	// rise 1, 2, 3, 4 and to shift as a running derivation is counted or not.
	// Report what it currently says rather than fixing the first value seen.
	Done, Expected int
	// LastLine is the most recent line of the build's own output.
	LastLine string
}

// Decoder reads the stream and maintains the current Update.
//
// It tracks live ACTIVITIES by id, because a progress report only means
// something in the context of the activity that emitted it. A substituter query
// reporting "3 of 3" before any build starts is a real, observed case; read as
// build progress it announces a build complete before it began.
type Decoder struct {
	// activity maps a live activity id to its type.
	activity map[uint64]int
	// buildsID is the id of the aggregate activity carrying build totals, once
	// seen. Progress from any other activity is not build progress.
	buildsID     uint64
	haveBuildsID bool

	cur Update
}

// New returns a Decoder ready to read a stream.
func New() *Decoder {
	return &Decoder{activity: map[uint64]int{}}
}

// event is the wire shape. Every field is optional: a decoder that requires any
// of them would reject events a future Nix adds.
type event struct {
	Action string        `json:"action"`
	ID     uint64        `json:"id"`
	Type   *int          `json:"type"`
	Fields []interface{} `json:"fields"`
}

// Line feeds one line of the subprocess's output to the decoder.
//
// It returns TWO booleans, and the difference between them matters:
//
//   - changed: the Update now says something new, so the caller should report it.
//   - understood: the line was part of the stream and could be read — even if
//     it changed nothing.
//
// Conflating the two is a trap. Most of a real stream is well-formed events
// that change nothing: a single narinfo query emits a dozen identical progress
// tuples. A caller that treats "did not change" as "could not read" will
// conclude the format is broken within the first activity, fall back to raw
// output, and never report a build — silently, because falling back is not an
// error.
//
// A line that is not part of the structured stream, is malformed, or carries an
// event this decoder does not know is IGNORED — never an error. That is the
// safety property: decoration cannot break the build it describes.
func (d *Decoder) Line(line string) (changed, understood bool) {
	raw, ok := strings.CutPrefix(strings.TrimRight(line, "\r\n"), prefix)
	if !ok {
		return false, false // not part of the stream
	}
	var e event
	if err := json.Unmarshal([]byte(raw), &e); err != nil {
		return false, false // malformed: skip it, keep reading
	}

	switch e.Action {
	case actionStart:
		return d.start(e), true
	case actionResult:
		return d.result(e), true
	case actionStop:
		delete(d.activity, e.ID)
		return false, true
	case actionMsg:
		return false, true
	}
	return false, false // an action we do not know
}

func (d *Decoder) start(e event) bool {
	if e.Type == nil {
		return false
	}
	d.activity[e.ID] = *e.Type

	switch *e.Type {
	case actBuilds:
		// The aggregate that carries the build totals. Remember it so its
		// progress can be told apart from every other activity's.
		d.buildsID, d.haveBuildsID = e.ID, true
	case actBuild:
		// fields[0] is the .drv path of the derivation now building.
		if drv, ok := firstString(e.Fields); ok {
			d.cur.Building = DerivationName(drv)
			d.cur.LastLine = "" // a new derivation, not the previous one's output
			return true
		}
	}
	return false
}

func (d *Decoder) result(e event) bool {
	if e.Type == nil {
		return false
	}
	switch *e.Type {
	case resBuildLogLine:
		if s, ok := firstString(e.Fields); ok {
			d.cur.LastLine = strings.TrimRight(s, "\r\n")
			return true
		}
	case resProgress:
		// ONLY from the activity that aggregates builds. Every other activity
		// reports progress of its own, and attributing it here is how a build
		// reads as finished before it starts.
		if !d.haveBuildsID || e.ID != d.buildsID {
			return false
		}
		done, expected, ok := firstTwoInts(e.Fields)
		if !ok {
			return false
		}
		if done == d.cur.Done && expected == d.cur.Expected {
			return false
		}
		d.cur.Done, d.cur.Expected = done, expected
		return true
	}
	return false
}

// Update returns the current snapshot.
func (d *Decoder) Update() Update { return d.cur }

// DerivationName reduces a store path to the readable part: the name a user
// recognises, with the hash and the `.drv` suffix removed.
//
//	/nix/store/<hash>-nixos-image.drv -> nixos-image
//
// A path that does not have that shape is returned as its base name, so an
// unexpected input degrades to something printable rather than to nothing.
func DerivationName(p string) string {
	base := strings.TrimSuffix(path.Base(p), ".drv")
	if i := strings.IndexByte(base, '-'); i >= 0 && i+1 < len(base) {
		// A store hash is 32 chars; only strip a prefix that looks like one.
		if i == 32 {
			return base[i+1:]
		}
	}
	return base
}

// firstString reads fields[0] as a string.
func firstString(fields []interface{}) (string, bool) {
	if len(fields) == 0 {
		return "", false
	}
	s, ok := fields[0].(string)
	return s, ok
}

// firstTwoInts reads fields[0] and fields[1] as counts. JSON numbers decode as
// float64; anything else means a shape we do not understand, which is ignored
// rather than guessed at.
func firstTwoInts(fields []interface{}) (a, b int, ok bool) {
	if len(fields) < 2 {
		return 0, 0, false
	}
	x, ok1 := fields[0].(float64)
	y, ok2 := fields[1].(float64)
	if !ok1 || !ok2 {
		return 0, 0, false
	}
	return int(x), int(y), true
}
