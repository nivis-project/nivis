// Copyright 2026 TechNative B.V. and the nivis authors
// SPDX-License-Identifier: Apache-2.0

package fakests_test

import (
	"context"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/aws/aws-sdk-go-v2/service/sts"

	"github.com/nivis-project/nivis/internal/fakests"
)

// stsClient points the real STS client at the fake.
func stsClient(srv *fakests.Server) *sts.Client {
	return sts.New(sts.Options{
		Region:       "us-east-1",
		BaseEndpoint: aws.String(srv.URL()),
		Credentials:  aws.AnonymousCredentials{},
	})
}

// stscreds against the fake yields credentials the SDK can use, carrying the
// fake's recognizable access key.
func TestAssumeRoleYieldsUsableCredentials(t *testing.T) {
	srv := fakests.New()
	defer srv.Close()

	p := stscreds.NewAssumeRoleProvider(stsClient(srv), "arn:aws:iam::123456789012:role/state")
	creds, err := p.Retrieve(context.Background())
	if err != nil {
		t.Fatalf("retrieve: %v", err)
	}
	if creds.AccessKeyID != srv.AccessKeyID() {
		t.Errorf("access key = %q, want %q", creds.AccessKeyID, srv.AccessKeyID())
	}
	if creds.SecretAccessKey == "" || creds.SessionToken == "" {
		t.Error("the issued credentials should carry a secret and a session token")
	}
	if creds.Expired() {
		t.Error("the issued credentials should not be expired on arrival")
	}
}

// The fake records what it was asked, so the role, session name and external id
// are assertable.
func TestRecordsWhatWasAsked(t *testing.T) {
	srv := fakests.New()
	defer srv.Close()

	const role = "arn:aws:iam::123456789012:role/landing_zone_devops_user"
	p := stscreds.NewAssumeRoleProvider(stsClient(srv), role, func(o *stscreds.AssumeRoleOptions) {
		o.RoleSessionName = "nivis"
		o.ExternalID = aws.String("shared-secret-id")
	})
	if _, err := p.Retrieve(context.Background()); err != nil {
		t.Fatalf("retrieve: %v", err)
	}

	got := srv.Last()
	if got.RoleARN != role {
		t.Errorf("role = %q, want %q", got.RoleARN, role)
	}
	if got.SessionName != "nivis" {
		t.Errorf("session name = %q, want nivis", got.SessionName)
	}
	if got.ExternalID != "shared-secret-id" {
		t.Errorf("external id = %q, want shared-secret-id", got.ExternalID)
	}
	if srv.Calls() != 1 {
		t.Errorf("calls = %d, want 1", srv.Calls())
	}
}

// A refused assumption surfaces the STS error rather than hanging or returning
// empty credentials.
func TestRefusedAssumption(t *testing.T) {
	srv := fakests.New()
	defer srv.Close()
	srv.Refuse("AccessDenied")

	p := stscreds.NewAssumeRoleProvider(stsClient(srv), "arn:aws:iam::123456789012:role/state")
	creds, err := p.Retrieve(context.Background())
	if err == nil {
		t.Fatalf("a refused assumption should fail, got credentials %q", creds.AccessKeyID)
	}
	if !strings.Contains(err.Error(), "AccessDenied") {
		t.Errorf("the STS error code should surface, got: %v", err)
	}
	if srv.Calls() != 0 {
		t.Errorf("a refused assumption should not be recorded as a successful call, got %d", srv.Calls())
	}
}
