// Copyright 2026 TechNative B.V. and the nivis authors
// SPDX-License-Identifier: Apache-2.0

// Package providerlog renders a spawned provider's log entries as readable
// notes.
//
// A provider writes its own log format to stderr; go-plugin parses it and
// re-emits each entry through the logger the plugin manager installs, with the
// level it declared and every remaining field as a key/value pair. That logger
// used to be hclog's text formatter, which faithfully printed the provider's
// internal telemetry (request ids, caller sites, transport internals) and — worst
// of all — an `error` field on a non-error entry, which reads as a failure.
//
// This package is the replacement formatter: the manager gives hclog
// JSONFormat + a Sink writer, so hclog becomes a serializer and every decision
// about what a user sees lives here, in one place, as a function from bytes to
// lines.
package providerlog

import (
	"fmt"
	"strings"

	"github.com/hashicorp/go-hclog"
)

// Level selects which provider log levels are surfaced. It is the value of the
// CLI's --provider-log-level.
type Level int

const (
	// LevelOff surfaces nothing at all.
	LevelOff Level = iota
	// LevelError surfaces only provider errors (a user who wants no provider
	// commentary).
	LevelError
	// LevelWarn is the DEFAULT: provider warnings are surfaced, as rendered
	// notes. Warnings are NOT hidden by default: a level low enough to hide the
	// providers' benign internal chatter also hides deprecation notices and
	// replacement warnings, permanently and invisibly. Noise is fixed by
	// rendering and collapsing (see Sink), not by suppression.
	LevelWarn
	// LevelInfo surfaces informational entries too.
	LevelInfo
	// LevelDebug surfaces debug entries.
	LevelDebug
	// LevelTrace surfaces everything, UNABRIDGED: at this level the fields a
	// rendered note drops (request id, caller site, rpc name) are printed,
	// because debugging a provider is the whole purpose of asking for trace.
	LevelTrace
)

// DefaultLevel is the level used when the flag is not given.
const DefaultLevel = LevelWarn

// levelNames is the accepted spelling of each level, in ascending verbosity.
var levelNames = []struct {
	name  string
	level Level
}{
	{"off", LevelOff},
	{"error", LevelError},
	{"warn", LevelWarn},
	{"info", LevelInfo},
	{"debug", LevelDebug},
	{"trace", LevelTrace},
}

// ParseLevel resolves a level name. An unrecognised value is an error naming
// every accepted value, so a typo cannot silently fall back to a default.
func ParseLevel(s string) (Level, error) {
	want := strings.ToLower(strings.TrimSpace(s))
	for _, l := range levelNames {
		if l.name == want {
			return l.level, nil
		}
	}
	return DefaultLevel, fmt.Errorf("unsupported provider log level %q (accepted: %s)", s, strings.Join(LevelNames(), ", "))
}

// LevelNames lists the accepted level names, ascending in verbosity.
func LevelNames() []string {
	out := make([]string, 0, len(levelNames))
	for _, l := range levelNames {
		out = append(out, l.name)
	}
	return out
}

// String is the level's canonical name.
func (l Level) String() string {
	for _, n := range levelNames {
		if n.level == l {
			return n.name
		}
	}
	return "warn"
}

// HCLog maps the level onto the hclog level the plugin manager installs, so
// entries below it are dropped before they are ever formatted. go-plugin checks
// the logger's level too and skips preparing an entry it would discard, which is
// what keeps a trace-happy provider cheap.
func (l Level) HCLog() hclog.Level {
	switch l {
	case LevelOff:
		return hclog.Off
	case LevelError:
		return hclog.Error
	case LevelInfo:
		return hclog.Info
	case LevelDebug:
		return hclog.Debug
	case LevelTrace:
		return hclog.Trace
	default:
		return hclog.Warn
	}
}

// verbatim reports whether the level asks for unabridged entries (trace).
func (l Level) verbatim() bool { return l >= LevelTrace }
