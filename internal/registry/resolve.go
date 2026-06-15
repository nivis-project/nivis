// Copyright 2026 WeareTechnative B.V. and the nixform authors
// SPDX-License-Identifier: Apache-2.0

// Package registry resolves provider addresses against the OpenTofu registry,
// downloads the platform binary from its release host, verifies it against the
// published SHA256SUMS, and caches the unpacked executable. Network is used only
// on a cache miss.
package registry

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
)

// DefaultHost is the OpenTofu registry.
const DefaultHost = "registry.opentofu.org"

// Address is a parsed provider address.
type Address struct {
	Host      string
	Namespace string
	Name      string
}

// ParseAddress parses "<host>/<ns>/<name>" or "<ns>/<name>" (host defaults to
// DefaultHost). It is intentionally strict: a value that doesn't look like a
// registry address (e.g. a filesystem path) should not be passed here.
func ParseAddress(s string) (Address, error) {
	parts := strings.Split(strings.Trim(s, "/"), "/")
	switch len(parts) {
	case 2:
		return Address{Host: DefaultHost, Namespace: parts[0], Name: parts[1]}, nil
	case 3:
		return Address{Host: parts[0], Namespace: parts[1], Name: parts[2]}, nil
	default:
		return Address{}, fmt.Errorf("registry: %q is not a provider address (<ns>/<name> or <host>/<ns>/<name>)", s)
	}
}

// LooksLikeAddress reports whether s is plausibly a registry address rather than
// a filesystem path: 2–3 slash-separated non-empty segments and no path-y
// prefixes. Callers should still prefer "is it an existing file?" first.
func LooksLikeAddress(s string) bool {
	if strings.HasPrefix(s, ".") || strings.HasPrefix(s, "/") || strings.HasPrefix(s, "~") {
		return false
	}
	n := len(strings.Split(strings.Trim(s, "/"), "/"))
	return n == 2 || n == 3
}

// Download describes a resolved, platform-specific download.
type Download struct {
	Version     string
	DownloadURL string
	ShasumsURL  string
	Filename    string
}

type versionsResp struct {
	Versions []struct {
		Version string `json:"version"`
	} `json:"versions"`
}

type downloadResp struct {
	DownloadURL string `json:"download_url"`
	ShasumsURL  string `json:"shasums_url"`
	Filename    string `json:"filename"`
}

// Resolve picks a version (latest by semver) and returns the platform download
// for the current GOOS/GOARCH.
func (c *Client) Resolve(ctx context.Context, addr Address) (Download, error) {
	return c.resolvePlatform(ctx, addr, runtime.GOOS, runtime.GOARCH)
}

func (c *Client) resolvePlatform(ctx context.Context, addr Address, os, arch string) (Download, error) {
	base := fmt.Sprintf("https://%s/v1/providers/%s/%s", addr.Host, addr.Namespace, addr.Name)

	var vr versionsResp
	if err := c.getJSON(ctx, base+"/versions", &vr); err != nil {
		return Download{}, fmt.Errorf("registry: list versions for %s/%s: %w", addr.Namespace, addr.Name, err)
	}
	if len(vr.Versions) == 0 {
		return Download{}, fmt.Errorf("registry: no versions for %s/%s", addr.Namespace, addr.Name)
	}
	latest := latestVersion(vr.Versions)

	var dr downloadResp
	dlURL := fmt.Sprintf("%s/%s/download/%s/%s", base, latest, os, arch)
	if err := c.getJSON(ctx, dlURL, &dr); err != nil {
		return Download{}, fmt.Errorf("registry: resolve %s/%s %s %s/%s: %w", addr.Namespace, addr.Name, latest, os, arch, err)
	}
	return Download{Version: latest, DownloadURL: dr.DownloadURL, ShasumsURL: dr.ShasumsURL, Filename: dr.Filename}, nil
}

var numRe = regexp.MustCompile(`\d+`)

// latestVersion returns the highest version by numeric (semver-ish) ordering.
func latestVersion(vs []struct {
	Version string `json:"version"`
}) string {
	type kv struct {
		raw string
		key []int
	}
	out := make([]kv, 0, len(vs))
	for _, v := range vs {
		nums := numRe.FindAllString(v.Version, -1)
		key := make([]int, 0, 3)
		for i := 0; i < 3; i++ {
			if i < len(nums) {
				n, _ := strconv.Atoi(nums[i])
				key = append(key, n)
			} else {
				key = append(key, 0)
			}
		}
		out = append(out, kv{raw: v.Version, key: key})
	}
	sort.Slice(out, func(i, j int) bool {
		for k := 0; k < 3; k++ {
			if out[i].key[k] != out[j].key[k] {
				return out[i].key[k] < out[j].key[k]
			}
		}
		return out[i].raw < out[j].raw
	})
	return out[len(out)-1].raw
}

func (c *Client) getJSON(ctx context.Context, url string, dst interface{}) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("GET %s -> HTTP %d: %s", url, resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return json.NewDecoder(resp.Body).Decode(dst)
}
