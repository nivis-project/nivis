// Copyright 2026 TechNative B.V. and the nivis authors
// SPDX-License-Identifier: Apache-2.0

package state_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/sts"

	"github.com/nivis-project/nivis/internal/fakes3"
	"github.com/nivis-project/nivis/internal/fakests"
	"github.com/nivis-project/nivis/internal/state"
)

// The whole hermetic strategy for assume-role rests on one SDK behaviour: the
// global AWS_ENDPOINT_URL aims STS at a fake, while an explicit per-client
// BaseEndpoint (which the s3 backend sets from its own `endpoint` key) still wins
// for S3. If that were not so, the two services could not be aimed at different
// fakes and every test below would be meaningless.
func TestEndpointSplitBetweenSTSAndS3(t *testing.T) {
	stsSrv := fakests.New()
	defer stsSrv.Close()
	s3Srv := fakes3.New()
	defer s3Srv.Close()

	t.Setenv("AWS_ACCESS_KEY_ID", "test")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "test")
	t.Setenv("AWS_REGION", "us-east-1")
	t.Setenv("AWS_ENDPOINT_URL", stsSrv.URL())

	cfg, err := awsconfig.LoadDefaultConfig(context.Background(), awsconfig.WithRegion("us-east-1"))
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	// STS follows the environment.
	stsClient := sts.NewFromConfig(cfg)
	if _, err := stsClient.AssumeRole(context.Background(), &sts.AssumeRoleInput{
		RoleArn:         aws.String("arn:aws:iam::123456789012:role/state"),
		RoleSessionName: aws.String("probe"),
	}); err != nil {
		t.Fatalf("STS should have reached the fake: %v", err)
	}
	if stsSrv.Calls() != 1 {
		t.Fatalf("fake STS calls = %d, want 1 (AWS_ENDPOINT_URL did not take effect)", stsSrv.Calls())
	}

	// S3 follows its explicit BaseEndpoint, not the environment.
	s3Client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(s3Srv.URL())
		o.UsePathStyle = true
	})
	if _, err := s3Client.PutObject(context.Background(), &s3.PutObjectInput{
		Bucket: aws.String("b"), Key: aws.String("k"),
	}); err != nil {
		t.Fatalf("S3 should have reached its own fake: %v", err)
	}
	if !s3Srv.Has("b", "k") {
		t.Error("the object should exist on the fake S3, so the explicit endpoint won")
	}
	if stsSrv.Calls() != 1 {
		t.Errorf("the S3 call must not have gone to the STS fake (calls = %d)", stsSrv.Calls())
	}
}

// openWithRole opens an s3 store whose STS calls go to the fake STS and whose S3
// calls go to the fake S3, merging the given assumeRole block.
func openWithRole(t *testing.T, stsSrv *fakests.Server, s3Srv *fakes3.Server, bucket, key string, block map[string]interface{}) (state.Store, error) {
	t.Helper()
	t.Setenv("AWS_ACCESS_KEY_ID", "base-identity")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "base-secret")
	t.Setenv("AWS_REGION", "us-east-1")
	if stsSrv != nil {
		t.Setenv("AWS_ENDPOINT_URL", stsSrv.URL())
	}
	b := map[string]interface{}{
		"type": "s3", "bucket": bucket, "key": key, "region": "us-east-1",
		"endpoint": s3Srv.URL(),
	}
	if block != nil {
		b["assumeRole"] = block
	}
	return state.OpenBackend(b, "")
}

const testRoleARN = "arn:aws:iam::123456789012:role/landing_zone_devops_user"

// A configured role is assumed and its credentials are what reach S3. The bucket
// is gated on the assumed identity, so this fails unless the assumption happened.
func TestAssumeRoleCredentialsReachS3(t *testing.T) {
	stsSrv := fakests.New()
	defer stsSrv.Close()
	s3Srv := fakes3.New()
	defer s3Srv.Close()
	s3Srv.AllowOnlyAccessKey("member", stsSrv.AccessKeyID())

	st, err := openWithRole(t, stsSrv, s3Srv, "member", "app.json",
		map[string]interface{}{"roleArn": testRoleARN})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := st.Set(state.ResourceState{ID: "a.t.n", Type: "t", Attrs: map[string]interface{}{"id": "x"}}); err != nil {
		t.Fatalf("a write under the assumed role should be accepted: %v", err)
	}
	if !s3Srv.Has("member", "app.json") {
		t.Error("the state object should exist")
	}
	if stsSrv.Calls() == 0 {
		t.Error("the role should have been assumed")
	}
	if got := stsSrv.Last().RoleARN; got != testRoleARN {
		t.Errorf("assumed role = %q, want %q", got, testRoleARN)
	}
}

// Without assumeRole the base identity is used, which the gated bucket refuses.
// This is the negative control for the test above: it proves the gate is real.
func TestWithoutAssumeRoleTheBaseIdentityIsRefused(t *testing.T) {
	stsSrv := fakests.New()
	defer stsSrv.Close()
	s3Srv := fakes3.New()
	defer s3Srv.Close()
	s3Srv.AllowOnlyAccessKey("member", stsSrv.AccessKeyID())

	st, err := openWithRole(t, stsSrv, s3Srv, "member", "app.json", nil)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := st.Set(state.ResourceState{ID: "a.t.n", Type: "t"}); err == nil {
		t.Fatal("the base identity should be refused by the gated bucket")
	}
	if stsSrv.Calls() != 0 {
		t.Errorf("no assumeRole means no STS call at all, got %d", stsSrv.Calls())
	}
}

// The session name defaults to nivis, and an external id is sent when configured.
func TestAssumeRoleSessionNameAndExternalID(t *testing.T) {
	cases := []struct {
		name        string
		block       map[string]interface{}
		wantSession string
		wantExtID   string
	}{
		{"session name defaults", map[string]interface{}{"roleArn": testRoleARN}, "nivis", ""},
		{"session name is configurable", map[string]interface{}{"roleArn": testRoleARN, "sessionName": "pim-laptop"}, "pim-laptop", ""},
		{"external id is sent", map[string]interface{}{"roleArn": testRoleARN, "externalId": "shared-secret"}, "nivis", "shared-secret"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			stsSrv := fakests.New()
			defer stsSrv.Close()
			s3Srv := fakes3.New()
			defer s3Srv.Close()

			st, err := openWithRole(t, stsSrv, s3Srv, "b", "k", tc.block)
			if err != nil {
				t.Fatalf("open: %v", err)
			}
			if err := st.Set(state.ResourceState{ID: "a.t.n", Type: "t"}); err != nil {
				t.Fatalf("set: %v", err)
			}
			got := stsSrv.Last()
			if got.SessionName != tc.wantSession {
				t.Errorf("session name = %q, want %q", got.SessionName, tc.wantSession)
			}
			if got.ExternalID != tc.wantExtID {
				t.Errorf("external id = %q, want %q", got.ExternalID, tc.wantExtID)
			}
		})
	}
}

// Credentials are cached: many state operations assume ONCE. Without the cache
// every request drags an STS call behind it, which this asserts on the call count
// rather than on behaviour that would look identical either way.
func TestAssumedCredentialsAreCached(t *testing.T) {
	stsSrv := fakests.New()
	defer stsSrv.Close()
	s3Srv := fakes3.New()
	defer s3Srv.Close()

	st, err := openWithRole(t, stsSrv, s3Srv, "b", "k",
		map[string]interface{}{"roleArn": testRoleARN})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	for i := 0; i < 5; i++ {
		if err := st.Set(state.ResourceState{ID: "a.t.n", Type: "t", Attrs: map[string]interface{}{"i": i}}); err != nil {
			t.Fatalf("set %d: %v", i, err)
		}
	}
	if _, err := st.List(); err != nil {
		t.Fatalf("list: %v", err)
	}
	// Five read-modify-writes plus a list is a dozen or so S3 calls.
	if n := stsSrv.Calls(); n != 1 {
		t.Errorf("STS calls = %d, want 1 (the credentials cache is missing or ineffective)", n)
	}
}

// A refused assumption surfaces the STS cause, not a missing bucket or an empty
// state document.
func TestRefusedAssumptionSurfaces(t *testing.T) {
	stsSrv := fakests.New()
	defer stsSrv.Close()
	stsSrv.Refuse("AccessDenied")
	s3Srv := fakes3.New()
	defer s3Srv.Close()

	st, err := openWithRole(t, stsSrv, s3Srv, "b", "k",
		map[string]interface{}{"roleArn": testRoleARN})
	if err != nil {
		t.Fatalf("open should succeed; the assumption happens lazily: %v", err)
	}
	_, _, err = st.Get("a.t.n")
	if err == nil {
		t.Fatal("a refused assumption should fail the operation")
	}
	msg := err.Error()
	if !strings.Contains(msg, "AssumeRole") && !strings.Contains(msg, "AccessDenied") {
		t.Errorf("the STS cause should surface, got: %s", msg)
	}
	if strings.Contains(msg, "does not exist") {
		t.Errorf("a refused assumption must not be reported as a missing bucket, got: %s", msg)
	}
	var mbe *state.MissingBucketError
	if errors.As(err, &mbe) {
		t.Error("a refused assumption must not be a MissingBucketError")
	}
}

// An assumeRole block without a roleArn is rejected before any request.
func TestAssumeRoleRequiresRoleArn(t *testing.T) {
	s3Srv := fakes3.New()
	defer s3Srv.Close()

	_, err := openWithRole(t, nil, s3Srv, "b", "k", map[string]interface{}{"sessionName": "nivis"})
	if err == nil {
		t.Fatal("an assumeRole block without roleArn should be rejected")
	}
	if !strings.Contains(err.Error(), "requires roleArn") {
		t.Errorf("the error should name roleArn, got: %v", err)
	}
	if s3Srv.Has("b", "k") {
		t.Error("no request should have been made")
	}
}

// A denied write names BOTH causes when a role is configured, and only the
// encryption one when it is not. A run without assumeRole reads exactly as it did
// before this change.
func TestDeniedWriteNamesTheAssumedRole(t *testing.T) {
	withRole := func(t *testing.T, block map[string]interface{}) string {
		t.Helper()
		stsSrv := fakests.New()
		defer stsSrv.Close()
		s3Srv := fakes3.New()
		defer s3Srv.Close()
		s3Srv.DenyMismatchedSSE("hardened", "aws:kms")

		st, err := openWithRole(t, stsSrv, s3Srv, "hardened", "k", block)
		if err != nil {
			t.Fatalf("open: %v", err)
		}
		err = st.Set(state.ResourceState{ID: "a.t.n", Type: "t"})
		if err == nil {
			t.Fatal("a denied write should fail")
		}
		return err.Error()
	}

	with := withRole(t, map[string]interface{}{"roleArn": testRoleARN, "sessionName": "ci"})
	if !strings.Contains(with, "encryption mismatch") {
		t.Errorf("the encryption cause should be named, got: %s", with)
	}
	if !strings.Contains(with, "assumed role") || !strings.Contains(with, testRoleARN) {
		t.Errorf("the assumed role should be named as a cause, got: %s", with)
	}
	if !strings.Contains(with, `"ci"`) {
		t.Errorf("the session name should be named, got: %s", with)
	}

	without := withRole(t, nil)
	if strings.Contains(without, "assumed role") {
		t.Errorf("a run with no assumeRole must not mention a role, got: %s", without)
	}
	if !strings.Contains(without, "encryption mismatch") {
		t.Errorf("the encryption cause should still be named, got: %s", without)
	}
}
