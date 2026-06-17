// Copyright 2026 TechNative B.V. and the nivis authors
// SPDX-License-Identifier: Apache-2.0

// Package vars resolves configuration variables for a Nivis run from the CLI and
// environment, applying Terraform's precedence (lowest to highest, last wins):
//
//	NIVIS_VAR_<name> env  <  --var-file <path> (JSON; later wins)  <  --var name=value (later wins)
//
// The resolved map is injected as the ledger `vars` object, constant across all
// phases (docs/IR-CONTRACT.md). Declared defaults are NOT applied here; that is
// the Nix layer's job (nivis.mkVars fills an unset variable from its default).
package vars

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// envPrefix is the environment-variable prefix for a variable: NIVIS_VAR_region.
const envPrefix = "NIVIS_VAR_"

// Resolve merges the variable sources in precedence order and returns the
// resolved map (nil when nothing is set, so the ledger omits `vars`). `environ`
// is the process environment in "KEY=value" form (os.Environ()); passing it in
// keeps the function pure and testable. `varFiles` are paths to JSON objects;
// `varFlags` are "name=value" strings. Later files/flags override earlier ones,
// and flags override files override env.
func Resolve(environ []string, varFiles []string, varFlags []string) (map[string]interface{}, error) {
	out := map[string]interface{}{}

	// 1. environment (lowest)
	for _, kv := range environ {
		eq := strings.IndexByte(kv, '=')
		if eq < 0 {
			continue
		}
		key, val := kv[:eq], kv[eq+1:]
		if name, ok := strings.CutPrefix(key, envPrefix); ok && name != "" {
			out[name] = val
		}
	}

	// 2. --var-file (JSON objects; later files win)
	for _, path := range varFiles {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("--var-file %q: %w", path, err)
		}
		var m map[string]interface{}
		if err := json.Unmarshal(data, &m); err != nil {
			return nil, fmt.Errorf("--var-file %q: not a JSON object: %w", path, err)
		}
		for k, v := range m {
			out[k] = v
		}
	}

	// 3. --var name=value (highest; later flags win)
	for _, f := range varFlags {
		eq := strings.IndexByte(f, '=')
		if eq < 0 {
			return nil, fmt.Errorf("--var %q: must be name=value", f)
		}
		name := f[:eq]
		if name == "" {
			return nil, fmt.Errorf("--var %q: empty variable name", f)
		}
		out[name] = f[eq+1:]
	}

	if len(out) == 0 {
		return nil, nil
	}
	return out, nil
}
