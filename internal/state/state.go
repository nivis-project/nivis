// Copyright 2026 TechNative B.V. and the nivis authors
// SPDX-License-Identifier: Apache-2.0

// Package state is the executor's state backend. For the PoC it is a trivial
// lockable local JSON file (DESIGN: keep state trivial until the round trip
// works; no tfstate-format compatibility). The Store interface is the seam a
// remote backend would implement later.
package state

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"syscall"
)

// ResourceState is the persisted state of one resource: its known output/attr
// values after apply.
type ResourceState struct {
	ID    string                 `json:"id"`
	Type  string                 `json:"type"`
	Attrs map[string]interface{} `json:"attrs"`
}

// Store is the state backend seam.
type Store interface {
	Get(id string) (ResourceState, bool, error)
	Set(s ResourceState) error
	Delete(id string) error
	List() ([]ResourceState, error)
}

// fileStore is a JSON-file-backed Store with an advisory file lock. Every
// mutating op takes an exclusive lock, reads the current file, applies the
// change, and atomically rewrites it (temp + rename), so a crash mid-write
// cannot corrupt or lose previously-persisted state.
type fileStore struct {
	path string
}

type document struct {
	Resources map[string]ResourceState `json:"resources"`
}

// Open returns a Store backed by the JSON file at path, creating its directory
// if needed. The file itself is created lazily on first write.
func Open(path string) (Store, error) {
	if dir := filepath.Dir(path); dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("state: mkdir %q: %w", dir, err)
		}
	}
	return &fileStore{path: path}, nil
}

func (s *fileStore) lockPath() string { return s.path + ".lock" }

// withLock runs fn while holding an exclusive advisory lock, serializing
// concurrent operations across processes.
func (s *fileStore) withLock(fn func() error) error {
	lf, err := os.OpenFile(s.lockPath(), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return fmt.Errorf("state: open lock: %w", err)
	}
	defer lf.Close()
	if err := syscall.Flock(int(lf.Fd()), syscall.LOCK_EX); err != nil {
		return fmt.Errorf("state: acquire lock: %w", err)
	}
	defer syscall.Flock(int(lf.Fd()), syscall.LOCK_UN)
	return fn()
}

func (s *fileStore) read() (document, error) {
	doc := document{Resources: map[string]ResourceState{}}
	data, err := os.ReadFile(s.path)
	if os.IsNotExist(err) {
		return doc, nil
	}
	if err != nil {
		return doc, fmt.Errorf("state: read %q: %w", s.path, err)
	}
	if len(data) == 0 {
		return doc, nil
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		return doc, fmt.Errorf("state: parse %q: %w", s.path, err)
	}
	if doc.Resources == nil {
		doc.Resources = map[string]ResourceState{}
	}
	return doc, nil
}

// writeAtomic writes doc to a temp file then renames it over the target, so the
// state file is never observed half-written.
func (s *fileStore) writeAtomic(doc document) error {
	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return fmt.Errorf("state: marshal: %w", err)
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("state: write temp: %w", err)
	}
	if err := os.Rename(tmp, s.path); err != nil {
		return fmt.Errorf("state: rename temp: %w", err)
	}
	return nil
}

func (s *fileStore) Get(id string) (ResourceState, bool, error) {
	var (
		out   ResourceState
		found bool
	)
	err := s.withLock(func() error {
		doc, err := s.read()
		if err != nil {
			return err
		}
		out, found = doc.Resources[id]
		return nil
	})
	return out, found, err
}

func (s *fileStore) Set(rs ResourceState) error {
	if rs.ID == "" {
		return fmt.Errorf("state: cannot Set a resource with empty id")
	}
	return s.withLock(func() error {
		doc, err := s.read()
		if err != nil {
			return err
		}
		doc.Resources[rs.ID] = rs
		return s.writeAtomic(doc)
	})
}

func (s *fileStore) Delete(id string) error {
	return s.withLock(func() error {
		doc, err := s.read()
		if err != nil {
			return err
		}
		delete(doc.Resources, id)
		return s.writeAtomic(doc)
	})
}

func (s *fileStore) List() ([]ResourceState, error) {
	var out []ResourceState
	err := s.withLock(func() error {
		doc, err := s.read()
		if err != nil {
			return err
		}
		for _, rs := range doc.Resources {
			out = append(out, rs)
		}
		sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
		return nil
	})
	return out, err
}
