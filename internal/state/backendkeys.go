// Copyright 2026 TechNative B.V. and the nivis authors
// SPDX-License-Identifier: Apache-2.0

package state

import (
	"fmt"
	"sort"
	"strings"
)

// The complete set of keys each backend type accepts. A key outside its type's set
// is rejected rather than ignored: a key no backend reads is inert, so a misspelled
// setting silently takes effect as its default and the failure that follows points
// nowhere near the cause.
var backendKeys = map[string][]string{
	"s3":    {"type", "bucket", "key", "region", "endpoint", "sseAlgorithm", "kmsKeyId"},
	"local": {"type"},
}

// Known Terraform S3 backend settings that have no nivis equivalent, mapped to
// what nivis does instead. Keys are normalized (see normalizeKey), so a setting
// transcribed in either convention is recognized.
var terraformBackendKeys = map[string]string{
	"dynamodbtable": "nivis takes its advisory lock with an S3 object at <key>.lock, so no table is needed",
	"rolearn":       "assuming a role for state access is not supported yet; use a pre-assumed AWS profile for now",
	"assumerole":    "assuming a role for state access is not supported yet; use a pre-assumed AWS profile for now",
	"sessionname":   "only meaningful with a role to assume, which nivis does not support for state access yet",
	"encrypt":       `use sseAlgorithm ("AES256", "bucket-default" or "aws:kms")`,
	"profile":       "credentials come from the AWS default credential chain; set AWS_PROFILE in the environment",
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

// validateBackendKeys rejects any key the named backend type does not accept. It
// runs before a client is constructed, so a misconfiguration never reaches AWS.
func validateBackendKeys(typ string, backend map[string]interface{}) error {
	accepted, ok := backendKeys[typ]
	if !ok {
		return nil
	}
	allowed := map[string]bool{}
	for _, k := range accepted {
		allowed[k] = true
	}
	var unknown []string
	for k := range backend {
		if !allowed[k] {
			unknown = append(unknown, k)
		}
	}
	if len(unknown) == 0 {
		return nil
	}
	sort.Strings(unknown)
	var b strings.Builder
	fmt.Fprintf(&b, "state: %s backend: unknown key", typ)
	if len(unknown) > 1 {
		b.WriteString("s")
	}
	for _, k := range unknown {
		fmt.Fprintf(&b, "\n  %q%s", k, explainUnknownKey(k, accepted))
	}
	fmt.Fprintf(&b, "\n  Accepted keys for %s: %s", typ, strings.Join(accepted, ", "))
	return fmt.Errorf("%s", b.String())
}

// explainUnknownKey adds the most specific thing that can be said about a rejected
// key: what nivis does instead of a known Terraform setting, or the accepted key it
// looks like a misspelling of.
func explainUnknownKey(k string, accepted []string) string {
	n := normalizeKey(k)
	if hint, ok := terraformBackendKeys[n]; ok {
		return ": a Terraform backend setting, " + hint
	}
	if s := suggestKey(n, accepted); s != "" {
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
