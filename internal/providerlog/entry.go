// Copyright 2026 TechNative B.V. and the nivis authors
// SPDX-License-Identifier: Apache-2.0

package providerlog

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// Entry is one provider log entry, as hclog serializes it with JSONFormat.
//
// The shape is a consequence of two hops: the provider emits its own entry, and
// go-plugin's parseJSON lifts @level/@message/@timestamp out of it and passes
// EVERY other field (including the provider's own @module and @caller) through
// as an ordinary key/value pair, appending the original timestamp as "timestamp".
// Our hclog logger then adds its own @level/@message/@timestamp/@module on top.
//
// That means "@module" can appear twice in the serialized line — hclog's own
// (provider.<binary>) and the provider's inner one (e.g. sdk.helper_schema) —
// and Go's decoder keeps the last. So the provider's IDENTITY is not taken from
// the entry at all; the Sink is told it when the manager spawns the provider.
type Entry struct {
	// Level is the entry's declared level ("warn", "error", …); empty when the
	// provider emitted something hclog could not level.
	Level string
	// Message is the human-readable message.
	Message string
	// Fields are the remaining key/values, stringified.
	Fields map[string]string
	// Raw is the undecoded line, for the verbatim (trace) path and for entries
	// that do not decode at all.
	Raw string
}

// Telemetry fields a rendered note drops: provider-internal plumbing that tells
// a user nothing about their infrastructure. Kept at trace level (see
// Level.verbatim), where they are exactly what a provider bug report needs.
var telemetryFields = map[string]bool{
	"@caller":             true,
	"@module":             true,
	"@timestamp":          true,
	"timestamp":           true,
	"tf_req_id":           true,
	"tf_rpc":              true,
	"tf_mux_provider":     true,
	"tf_provider_addr":    true,
	"tf_proto_version":    true,
	"tf_data_source_type": true,
}

// fieldMessage is the key hclog uses for the message; fieldLevel for the level.
const (
	fieldMessage = "@message"
	fieldLevel   = "@level"
	// fieldError is the trap this package exists to defuse: in a provider entry
	// this is a CAUSE annotation on a non-fatal event, not a failure. It is
	// rendered as a subordinate detail and never as an error.
	fieldError = "error"
	// fieldResourceType / fieldAttributePath give a note its subject.
	fieldResourceType  = "tf_resource_type"
	fieldAttributePath = "tf_attribute_path"
)

// Decode parses one serialized entry. It never fails: input that is not a
// well-formed entry yields an Entry carrying only Raw (and no level), because
// dropping a line the provider chose to emit would hide information, and
// crashing a run over a log line would be worse still.
func Decode(line []byte) Entry {
	raw := strings.TrimRight(string(line), "\n")
	e := Entry{Fields: map[string]string{}, Raw: raw}

	var decoded map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &decoded); err != nil {
		return e
	}
	for k, v := range decoded {
		switch k {
		case fieldMessage:
			e.Message = stringify(v)
		case fieldLevel:
			e.Level = strings.ToLower(stringify(v))
		default:
			e.Fields[k] = stringify(v)
		}
	}
	// A decoded entry with no message still gets one, so a note is never blank.
	if e.Message == "" {
		e.Message = raw
	}
	e.stripLevelPrefix()
	return e
}

// levelPrefixes are the bracketed level markers a provider writes when it logs
// plain text rather than a structured entry.
var levelPrefixes = []string{"TRACE", "DEBUG", "INFO", "WARN", "ERROR"}

// stripLevelPrefix removes a leading "[WARN] "-style marker from the message,
// adopting it as the entry's level when the entry declared none.
//
// The plugin transport infers a level from exactly this prefix for a plain-text
// line, but passes the WHOLE line through as the message — so a rendered note
// would otherwise print the level twice ("provider note …: [WARN] …"). The
// marker is the transport's input, not information for the reader.
func (e *Entry) stripLevelPrefix() {
	msg := strings.TrimSpace(e.Message)
	for _, p := range levelPrefixes {
		marker := "[" + p + "]"
		if !strings.HasPrefix(msg, marker) {
			continue
		}
		if e.Level == "" {
			e.Level = strings.ToLower(p)
		}
		e.Message = strings.TrimSpace(strings.TrimPrefix(msg, marker))
		return
	}
}

// stringify renders a JSON value as a single-line string. Numbers, booleans and
// nested values all appear; nothing is dropped for being the wrong shape.
func stringify(v interface{}) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return collapseWhitespace(t)
	case float64:
		// Render an integral float without its ".0" (hclog counts, ports, ids).
		if t == float64(int64(t)) {
			return fmt.Sprintf("%d", int64(t))
		}
		return fmt.Sprintf("%g", t)
	case bool:
		return fmt.Sprintf("%t", t)
	default:
		b, err := json.Marshal(t)
		if err != nil {
			return collapseWhitespace(fmt.Sprintf("%v", t))
		}
		return collapseWhitespace(string(b))
	}
}

// collapseWhitespace folds newlines and runs of blanks into single spaces, so one
// entry is always one line. A provider that logs a stack trace in a field cannot
// break the change list's layout.
func collapseWhitespace(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

// Subject is what a note is about, derived from the entry's own fields: the
// resource type it names, narrowed by the attribute path when present. When the
// entry names neither, the provider's identity stands in — a note is always
// attributed to something.
func (e Entry) Subject(provider string) string {
	typ := e.Fields[fieldResourceType]
	attr := e.Fields[fieldAttributePath]
	switch {
	case typ != "" && attr != "":
		return typ + "." + attr
	case typ != "":
		return typ
	default:
		return provider
	}
}

// Detail is the entry's `error` field: the cause annotation, rendered
// subordinate to the note. Empty when the provider attached none.
func (e Entry) Detail() string { return e.Fields[fieldError] }

// telemetry returns the dropped fields as "key=value" pairs, sorted for
// determinism. Used only by the verbatim (trace) path.
func (e Entry) telemetry() []string {
	out := make([]string, 0, len(e.Fields))
	for k, v := range e.Fields {
		if k == fieldError || k == fieldResourceType || k == fieldAttributePath {
			continue
		}
		out = append(out, k+"="+v)
	}
	sort.Strings(out)
	return out
}
