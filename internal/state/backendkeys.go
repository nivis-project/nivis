// Copyright 2026 TechNative B.V. and the nivis authors
// SPDX-License-Identifier: Apache-2.0

package state

import (
	"fmt"
	"sort"
	"strings"
)

// keySet is what one backend type, or one closed block inside it, accepts. keys is
// ordered: the error lists accepted keys in the order the documentation introduces
// them, which a map would scramble into alphabetical.
//
// blocks is separate from keys so the walker knows a value MUST be a block rather
// than inferring it from the value's type, and so only declared blocks are walked:
// recursing into anything map-shaped would reject caller-chosen keys the day a
// backend key legitimately holds an open map.
type keySet struct {
	keys   []string
	blocks map[string]keySet
}

// accepted is every name this set allows, scalars first and blocks after, which is
// the order the error message lists them in.
func (k keySet) accepted() []string {
	out := append([]string(nil), k.keys...)
	names := make([]string, 0, len(k.blocks))
	for name := range k.blocks {
		names = append(names, name)
	}
	sort.Strings(names)
	return append(out, names...)
}

// The complete set of keys each backend type accepts, nested blocks included. A key
// outside its set is rejected rather than ignored: a key no backend reads is inert,
// so a misspelled setting silently takes effect as its default and the failure that
// follows points nowhere near the cause. Validation reaches INSIDE a declared
// block, because a block whose inner keys went unchecked would be the same
// silent-drop failure one level down.
var backendKeys = map[string]keySet{
	"s3": {
		keys: []string{"type", "bucket", "key", "region", "endpoint", "sseAlgorithm", "kmsKeyId"},
		blocks: map[string]keySet{
			"assumeRole": {keys: []string{"roleArn", "sessionName", "externalId"}},
		},
	},
	"local": {keys: []string{"type"}},
}

// blockOf returns the block a key belongs inside, when it was written at the top
// level by mistake. OpenTofu's S3 backend takes the role settings at the top level
// as well as in a block, and the .tfbackend files these are transcribed from use
// the flat form, so this is the mistake callers actually make.
func blockOf(typ, normalized string) (block, key string) {
	set, ok := backendKeys[typ]
	if !ok {
		return "", ""
	}
	for name, inner := range set.blocks {
		for _, k := range inner.keys {
			if normalizeKey(k) == normalized {
				return name, k
			}
		}
	}
	return "", ""
}

// Known Terraform S3 backend settings that have no nivis equivalent, mapped to
// what nivis does instead. Keys are normalized (see normalizeKey), so a setting
// transcribed in either convention is recognized.
var terraformBackendKeys = map[string]string{
	"dynamodbtable": "nivis takes its advisory lock with an S3 object at <key>.lock, so no table is needed",
	"encrypt":       `use sseAlgorithm ("AES256", "bucket-default" or "aws:kms")`,
	"profile": "credentials come from the AWS default credential chain; set AWS_PROFILE in the environment. " +
		"A profile name is not a backend key because it points into one machine's shared config, " +
		"while the backend is committed configuration: it resolves differently per machine and is " +
		"commonly absent in CI, unlike a role ARN, which means the same thing everywhere",
}

// normalizeKey lowercases a key and drops word separators, so sse_algorithm,
// SSEAlgorithm and sseAlgorithm all compare equal. Someone moving a backend across
// from Terraform carries the naming convention with them, which is the realistic
// error, not a random typo.
func normalizeKey(k string) string {
	var b strings.Builder
	for _, r := range k {
		if r == '_' || r == '-' || r == ' ' {
			continue
		}
		b.WriteRune(r)
	}
	return strings.ToLower(b.String())
}

// validateBackendKeys rejects any key the named backend type does not accept,
// nested blocks included. It runs before a client is constructed, so a
// misconfiguration never reaches AWS.
func validateBackendKeys(typ string, backend map[string]interface{}) error {
	set, ok := backendKeys[typ]
	if !ok {
		return nil
	}
	var problems []string
	walkKeySet(typ, typ, set, backend, &problems)
	if len(problems) == 0 {
		return nil
	}
	var b strings.Builder
	fmt.Fprintf(&b, "state: %s backend: unknown key", typ)
	if len(problems) > 1 {
		b.WriteString("s")
	}
	for _, p := range problems {
		fmt.Fprintf(&b, "\n  %s", p)
	}
	fmt.Fprintf(&b, "\n  Accepted keys for %s: %s", typ, strings.Join(set.accepted(), ", "))
	return fmt.Errorf("%s", b.String())
}

// walkKeySet checks one level and recurses into the blocks the set declares. path
// is how a key is named in an error ("assumeRole.rolArn"); typ stays the backend
// type so a relocation hint can consult the whole tree.
func walkKeySet(typ, path string, set keySet, m map[string]interface{}, problems *[]string) {
	allowed := map[string]bool{}
	for _, k := range set.keys {
		allowed[k] = true
	}
	var unknown []string
	for k := range m {
		if _, isBlock := set.blocks[k]; isBlock || allowed[k] {
			continue
		}
		unknown = append(unknown, k)
	}
	sort.Strings(unknown)
	for _, k := range unknown {
		*problems = append(*problems, fmt.Sprintf("%q%s", qualify(path, typ, k),
			explainUnknownKey(typ, k, set, path == typ)))
	}

	names := make([]string, 0, len(set.blocks))
	for name := range set.blocks {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		raw, present := m[name]
		if !present {
			continue
		}
		inner, ok := raw.(map[string]interface{})
		if !ok {
			*problems = append(*problems, fmt.Sprintf("%q is a block, not a value: write %s = { %s = ...; }",
				qualify(path, typ, name), name, set.blocks[name].keys[0]))
			continue
		}
		walkKeySet(typ, path+"."+name, set.blocks[name], inner, problems)
	}
}

// qualify names a key the way the error should read: bare at the top level, and
// path-qualified inside a block.
func qualify(path, typ, key string) string {
	if path == typ {
		return key
	}
	return strings.TrimPrefix(path, typ+".") + "." + key
}

// explainUnknownKey adds the most specific thing that can be said about a rejected
// key: where it belongs when it was written at the wrong level, what nivis does
// instead of a known Terraform setting, or the accepted key it resembles.
// atTopLevel gates the relocation hint: a key that belongs in a block is only
// misplaced when it was written OUTSIDE that block. Inside it, the same name is an
// ordinary misspelling and wants the suggestion instead.
func explainUnknownKey(typ, k string, set keySet, atTopLevel bool) string {
	n := normalizeKey(k)
	if block, key := blockOf(typ, n); atTopLevel && block != "" {
		return fmt.Sprintf(": this belongs in the %s block, as %s.%s (nivis groups the role settings in one block)",
			block, block, key)
	}
	if hint, ok := terraformBackendKeys[n]; ok {
		return ": a Terraform backend setting, " + hint
	}
	if s := suggestKey(n, set.accepted()); s != "" {
		return fmt.Sprintf(": did you mean %q?", s)
	}
	return ""
}

// suggestKey returns the accepted key closest to the normalized key n, or empty
// when nothing is close enough to be worth guessing.
func suggestKey(n string, accepted []string) string {
	best, bestDist := "", 3
	for _, a := range accepted {
		d := editDistance(n, normalizeKey(a))
		if d < bestDist {
			best, bestDist = a, d
		}
	}
	return best
}

// editDistance is the Levenshtein distance between a and b.
func editDistance(a, b string) int {
	prev := make([]int, len(b)+1)
	cur := make([]int, len(b)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(a); i++ {
		cur[0] = i
		for j := 1; j <= len(b); j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			cur[j] = min(prev[j]+1, cur[j-1]+1, prev[j-1]+cost)
		}
		prev, cur = cur, prev
	}
	return prev[len(b)]
}
