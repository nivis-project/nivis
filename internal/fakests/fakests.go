// Copyright 2026 TechNative B.V. and the nivis authors
// SPDX-License-Identifier: Apache-2.0

// Package fakests is a hermetic, in-memory AWS STS for tests: the STS analogue of
// fakes3 and the in-repo fake providers. It answers the one call the state backend
// makes, AssumeRole, with a credentials document the AWS SDK accepts, and records
// what it was asked so a test can assert the role, the session name and the
// external id that were sent.
//
// It exists because the backend's assume-role path cannot otherwise be exercised
// without real AWS credentials. Point the SDK at it with AWS_ENDPOINT_URL; an
// explicit per-client BaseEndpoint (which the s3 backend sets from its own
// `endpoint` key) still wins for that client, so S3 and STS can be aimed at
// different fakes in the same test.
package fakests

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"time"
)

// Call is what a single AssumeRole request asked for.
type Call struct {
	RoleARN     string
	SessionName string
	ExternalID  string
}

// Server is an in-memory fake STS. Use New to start one and URL() as the endpoint.
type Server struct {
	ts    *httptest.Server
	mu    sync.Mutex
	calls []Call
	deny  string // when non-empty, every assumption is refused with this STS error code

	// accessKeyID is the key the issued credentials carry. It is what makes an
	// assumed identity distinguishable from the ambient one, which fakes3's
	// AllowOnlyAccessKey uses to model a bucket only the assumed role may touch.
	accessKeyID string
}

// New starts a fake STS issuing credentials under a fixed, recognizable access key
// id. Call Close to stop it.
func New() *Server {
	s := &Server{accessKeyID: "ASIAFAKEASSUMEDROLE"}
	s.ts = httptest.NewServer(http.HandlerFunc(s.handle))
	return s
}

// URL is the endpoint to point the SDK at (AWS_ENDPOINT_URL).
func (s *Server) URL() string { return s.ts.URL }

// Close stops the server.
func (s *Server) Close() { s.ts.Close() }

// AccessKeyID is the access key the issued credentials carry, so a test can gate a
// fake S3 bucket on the assumed identity.
func (s *Server) AccessKeyID() string { return s.accessKeyID }

// Calls is how many assumptions have been made. A backend that caches credentials
// assumes once for many requests; one that does not shows it here.
func (s *Server) Calls() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.calls)
}

// Last is the most recent assumption, so a test can assert what was sent.
func (s *Server) Last() Call {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.calls) == 0 {
		return Call{}
	}
	return s.calls[len(s.calls)-1]
}

// Refuse makes every later assumption fail with the given STS error code (for
// example "AccessDenied" or "ExpiredToken"), so the failure path is coverable.
func (s *Server) Refuse(code string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.deny = code
}

// handle answers the query-protocol AssumeRole call. The SDK posts the parameters
// as a form body.
func (s *Server) handle(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}
	if action := r.Form.Get("Action"); action != "AssumeRole" {
		writeSTSError(w, http.StatusBadRequest, "InvalidAction",
			fmt.Sprintf("fakests only implements AssumeRole, got %q", action))
		return
	}

	s.mu.Lock()
	deny := s.deny
	if deny == "" {
		s.calls = append(s.calls, Call{
			RoleARN:     r.Form.Get("RoleArn"),
			SessionName: r.Form.Get("RoleSessionName"),
			ExternalID:  r.Form.Get("ExternalId"),
		})
	}
	key := s.accessKeyID
	s.mu.Unlock()

	if deny != "" {
		writeSTSError(w, http.StatusForbidden, deny, "fakests refused the assumption")
		return
	}

	// An hour out, so a test never trips the SDK's refresh window by accident.
	expiry := time.Now().Add(time.Hour).UTC().Format(time.RFC3339)
	w.Header().Set("Content-Type", "text/xml")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, `<?xml version="1.0" encoding="UTF-8"?>`+
		`<AssumeRoleResponse xmlns="https://sts.amazonaws.com/doc/2011-06-15/">`+
		`<AssumeRoleResult><Credentials>`+
		`<AccessKeyId>%s</AccessKeyId>`+
		`<SecretAccessKey>fake-secret</SecretAccessKey>`+
		`<SessionToken>fake-session-token</SessionToken>`+
		`<Expiration>%s</Expiration>`+
		`</Credentials><AssumedRoleUser>`+
		`<Arn>%s</Arn><AssumedRoleId>AROAFAKE:%s</AssumedRoleId>`+
		`</AssumedRoleUser></AssumeRoleResult>`+
		`<ResponseMetadata><RequestId>fakests-request</RequestId></ResponseMetadata>`+
		`</AssumeRoleResponse>`,
		key, expiry, r.Form.Get("RoleArn"), r.Form.Get("RoleSessionName"))
}

// writeSTSError returns the query-protocol error shape the SDK maps to a typed
// API error, so a refused assumption surfaces its own cause.
func writeSTSError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "text/xml")
	w.WriteHeader(status)
	fmt.Fprintf(w, `<?xml version="1.0" encoding="UTF-8"?>`+
		`<ErrorResponse xmlns="https://sts.amazonaws.com/doc/2011-06-15/">`+
		`<Error><Type>Sender</Type><Code>%s</Code><Message>%s</Message></Error>`+
		`<RequestId>fakests-request</RequestId></ErrorResponse>`, code, message)
}
