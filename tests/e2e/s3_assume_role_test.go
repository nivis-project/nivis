// Copyright 2026 TechNative B.V. and the nivis authors
// SPDX-License-Identifier: Apache-2.0

package e2e_test

import (
	"context"
	"strings"
	"testing"

	"github.com/nivis-project/nivis/internal/fakes3"
	"github.com/nivis-project/nivis/internal/fakests"
	"github.com/nivis-project/nivis/internal/ledger"
	"github.com/nivis-project/nivis/internal/phase"
	"github.com/nivis-project/nivis/internal/plan"
	"github.com/nivis-project/nivis/internal/plugin"
	"github.com/nivis-project/nivis/internal/state"
)

const e2eRoleARN = "arn:aws:iam::104144963194:role/landing_zone_devops_user"

// assumeRoleFixture wires a run whose S3 goes to one fake and whose STS goes to
// another, against a bucket only the assumed identity may touch.
type assumeRoleFixture struct {
	sts    *fakests.Server
	s3     *fakes3.Server
	bucket string
	key    string
	root   string
}

func newAssumeRoleFixture(t *testing.T, gate bool) *assumeRoleFixture {
	t.Helper()
	requireNix(t)
	root := repoRoot(t)
	buildBinaries(t, root)
	t.Setenv("TERRAE_NIVIS_FAKE_COUNTER", "")
	t.Setenv("AWS_ACCESS_KEY_ID", "base-identity")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "base-secret")
	t.Setenv("AWS_REGION", "us-east-1")

	f := &assumeRoleFixture{
		sts: fakests.New(), s3: fakes3.New(),
		bucket: "member-account-state", key: "e2e/app.json", root: root,
	}
	t.Cleanup(func() { f.sts.Close(); f.s3.Close() })
	t.Setenv("AWS_ENDPOINT_URL", f.sts.URL())
	if gate {
		// Only the identity the fake STS issues may touch this bucket, which is
		// what a member-account bucket looks like from a management-account user.
		f.s3.AllowOnlyAccessKey(f.bucket, f.sts.AccessKeyID())
	}
	return f
}

func (f *assumeRoleFixture) open(t *testing.T, role bool) state.Store {
	t.Helper()
	b := map[string]interface{}{
		"type": "s3", "bucket": f.bucket, "key": f.key, "region": "us-east-1",
		"endpoint": f.s3.URL(),
	}
	if role {
		b["assumeRole"] = map[string]interface{}{"roleArn": e2eRoleARN, "sessionName": "nivis-e2e"}
	}
	st, err := state.OpenBackend(b, "")
	if err != nil {
		t.Fatalf("open backend (assumeRole=%v): %v", role, err)
	}
	return st
}

func (f *assumeRoleFixture) driver(st state.Store) (*phase.Driver, func()) {
	mgr := plugin.NewManager()
	return &phase.Driver{
		Eval:      phase.NixEval{FlakeRef: ".", Attr: "nivis.plan", WorkDir: f.root},
		Manager:   mgr,
		Store:     st,
		Ledger:    ledger.New(),
		MaxPhases: 10,
	}, mgr.Close
}

func (f *assumeRoleFixture) apply(t *testing.T, st state.Store) error {
	t.Helper()
	d, closer := f.driver(st)
	defer closer()
	_, err := d.Run(context.Background())
	return err
}

// The bean's acceptance, expressed hermetically: a bucket the base identity cannot
// reach becomes reachable by declaring a role in the backend. The run without
// assumeRole is the negative control that proves the gate is real.
func TestS3BackendAssumeRoleReachesAGatedBucket(t *testing.T) {
	f := newAssumeRoleFixture(t, true)

	if err := f.apply(t, f.open(t, false)); err == nil {
		t.Fatal("an apply as the base identity should be refused by the gated bucket")
	}
	if f.s3.Has(f.bucket, f.key) {
		t.Error("a refused run must not have written state")
	}
	if f.sts.Calls() != 0 {
		t.Errorf("a run without assumeRole must make no STS call, got %d", f.sts.Calls())
	}

	if err := f.apply(t, f.open(t, true)); err != nil {
		t.Fatalf("an apply under the declared role should succeed: %v", err)
	}
	if !f.s3.Has(f.bucket, f.key) {
		t.Fatal("state should have been written under the assumed role")
	}
	if f.sts.Calls() == 0 {
		t.Error("the role should have been assumed")
	}
	if got := f.sts.Last().RoleARN; got != e2eRoleARN {
		t.Errorf("assumed role = %q, want %q", got, e2eRoleARN)
	}
	if got := f.sts.Last().SessionName; got != "nivis-e2e" {
		t.Errorf("session name = %q, want nivis-e2e", got)
	}
}

// The assumed identity works for reads as well as writes: a re-plan plays the
// state back and reports no changes.
func TestS3BackendAssumeRoleReadsStateBack(t *testing.T) {
	f := newAssumeRoleFixture(t, true)

	if err := f.apply(t, f.open(t, true)); err != nil {
		t.Fatalf("apply under the assumed role: %v", err)
	}

	d, closer := f.driver(f.open(t, true))
	items, err := d.PlanReport(context.Background())
	closer()
	if err != nil {
		t.Fatalf("re-plan under the assumed role: %v", err)
	}
	if len(items) == 0 {
		t.Fatal("re-plan produced no items")
	}
	for _, it := range items {
		if it.Op != plan.OpNoop {
			t.Errorf("re-plan: %s op = %v, want OpNoop (state read back through the assumed role)", it.ID, it.Op)
		}
	}
}

// Back-compat: a configuration with no assumeRole behaves exactly as before and
// contacts no STS endpoint at all.
func TestS3BackendWithoutAssumeRoleIsUnchanged(t *testing.T) {
	f := newAssumeRoleFixture(t, false)

	if err := f.apply(t, f.open(t, false)); err != nil {
		t.Fatalf("an apply with no assumeRole should behave as before: %v", err)
	}
	if !f.s3.Has(f.bucket, f.key) {
		t.Error("state should have been written")
	}
	if f.sts.Calls() != 0 {
		t.Errorf("no STS call should be made without assumeRole, got %d", f.sts.Calls())
	}
	if sse := f.s3.SSEFor(f.bucket, f.key); sse != "AES256" {
		t.Errorf("SSE = %q, want AES256 (the unchanged default)", sse)
	}
}

// A refused assumption fails the run with the STS cause visible, rather than being
// swallowed into a generic state error. This is the failure nixform2-7xxn is about,
// and it should be legible even before that bean lands.
func TestS3BackendRefusedAssumptionIsLegible(t *testing.T) {
	f := newAssumeRoleFixture(t, true)
	f.sts.Refuse("ExpiredToken")

	err := f.apply(t, f.open(t, true))
	if err == nil {
		t.Fatal("a refused assumption should fail the run")
	}
	msg := err.Error()
	if !strings.Contains(msg, "ExpiredToken") && !strings.Contains(msg, "AssumeRole") {
		t.Errorf("the STS cause should be visible in the run's error, got: %s", msg)
	}
	if strings.Contains(msg, "does not exist") {
		t.Errorf("a refused assumption must not read as a missing bucket, got: %s", msg)
	}
}
