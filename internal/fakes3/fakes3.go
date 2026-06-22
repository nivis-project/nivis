// Copyright 2026 TechNative B.V. and the nivis authors
// SPDX-License-Identifier: Apache-2.0

// Package fakes3 is a hermetic, in-memory S3 server for tests: the S3 analogue of
// the in-repo fake providers. It speaks the handful of S3 verbs the state backend
// uses (PutObject, GetObject, DeleteObject, HEAD) over an httptest server, backed
// by a map. No network, no credentials, no real AWS. Path-style addressing
// (/<bucket>/<key>), which the s3Store uses when an endpoint is set.
package fakes3

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
)

// Server is an in-memory fake S3. Use New to start one and URL() as the s3Store
// endpoint. Close it when done. Objects records the last-seen put metadata so a
// test can assert, e.g., that server-side encryption was requested.
type Server struct {
	ts   *httptest.Server
	mu   sync.Mutex
	objs map[string][]byte // "<bucket>/<key>" -> bytes
	sse  map[string]string // "<bucket>/<key>" -> x-amz-server-side-encryption header
}

// New starts a fake S3 server. Call Close to stop it.
func New() *Server {
	s := &Server{objs: map[string][]byte{}, sse: map[string]string{}}
	s.ts = httptest.NewServer(http.HandlerFunc(s.handle))
	return s
}

// URL is the endpoint to pass to the s3 backend (the BaseEndpoint override).
func (s *Server) URL() string { return s.ts.URL }

// Close stops the server.
func (s *Server) Close() { s.ts.Close() }

// SSEFor returns the server-side-encryption header recorded for the last put to
// "<bucket>/<key>" (empty if none/absent), so tests can assert encryption.
func (s *Server) SSEFor(bucket, key string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.sse[bucket+"/"+key]
}

// Has reports whether an object exists at "<bucket>/<key>".
func (s *Server) Has(bucket, key string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.objs[bucket+"/"+key]
	return ok
}

// handle routes by HTTP method, path-style (/<bucket>/<key...>).
func (s *Server) handle(w http.ResponseWriter, r *http.Request) {
	bucket, key, ok := parsePath(r.URL.Path)
	if !ok {
		http.Error(w, "bad path", http.StatusBadRequest)
		return
	}
	id := bucket + "/" + key

	switch r.Method {
	case http.MethodPut:
		body, _ := io.ReadAll(r.Body)
		// Conditional create-if-absent (IfNoneMatch: "*"): if the object already
		// exists, fail with 412 PreconditionFailed (how S3 enforces an atomic lock).
		if r.Header.Get("If-None-Match") == "*" {
			s.mu.Lock()
			_, exists := s.objs[id]
			s.mu.Unlock()
			if exists {
				writePreconditionFailed(w)
				return
			}
		}
		s.mu.Lock()
		s.objs[id] = body
		s.sse[id] = r.Header.Get("X-Amz-Server-Side-Encryption")
		s.mu.Unlock()
		w.WriteHeader(http.StatusOK)
	case http.MethodGet, http.MethodHead:
		s.mu.Lock()
		data, found := s.objs[id]
		s.mu.Unlock()
		if !found {
			writeNoSuchKey(w, key)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if r.Method == http.MethodGet {
			_, _ = w.Write(data)
		}
	case http.MethodDelete:
		s.mu.Lock()
		delete(s.objs, id)
		delete(s.sse, id)
		s.mu.Unlock()
		w.WriteHeader(http.StatusNoContent)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// parsePath splits a path-style S3 URL "/<bucket>/<key...>" into bucket and key.
func parsePath(p string) (bucket, key string, ok bool) {
	p = strings.TrimPrefix(p, "/")
	i := strings.IndexByte(p, '/')
	if i < 0 {
		return "", "", false // bucket-only requests (e.g. ListObjects) are unused
	}
	bucket, key = p[:i], p[i+1:]
	if bucket == "" || key == "" {
		return "", "", false
	}
	return bucket, key, true
}

// writePreconditionFailed returns the S3 412 the SDK surfaces as a
// PreconditionFailed API error, used for a conditional create-if-absent that
// loses the race (the lock is already held).
func writePreconditionFailed(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/xml")
	w.WriteHeader(http.StatusPreconditionFailed)
	_, _ = io.WriteString(w, `<?xml version="1.0" encoding="UTF-8"?>`+
		`<Error><Code>PreconditionFailed</Code>`+
		`<Message>At least one of the pre-conditions you specified did not hold</Message>`+
		`<Condition>If-None-Match</Condition></Error>`)
}

// writeNoSuchKey returns the S3 NoSuchKey error the SDK maps to a typed error the
// store treats as an empty document.
func writeNoSuchKey(w http.ResponseWriter, key string) {
	w.Header().Set("Content-Type", "application/xml")
	w.WriteHeader(http.StatusNotFound)
	_, _ = io.WriteString(w, `<?xml version="1.0" encoding="UTF-8"?>`+
		`<Error><Code>NoSuchKey</Code><Message>The specified key does not exist.</Message>`+
		`<Key>`+key+`</Key></Error>`)
}
