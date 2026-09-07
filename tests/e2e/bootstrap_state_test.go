// Copyright 2026 TechNative B.V. and the nivis authors
// SPDX-License-Identifier: Apache-2.0

package e2e_test

// The self-managed-state-bucket bootstrap, end to end through the REAL nivis
// binary against the in-repo fakes (fake providers + hermetic fake S3, no network
// and no AWS): a configuration whose declared s3 backend lives in a bucket the run
// itself is responsible for creating.
//
// The sequence under test is the one docs/REMOTE-STATE.md documents:
//
//	nivis apply --backend=local      # the bucket does not exist yet
//	nivis state migrate --to-remote  # move the document into it
//	nivis apply                      # the declared backend, no changes
//
// The bucket coming into existence between the first two steps is modelled with
// fakes3.CreateBucket: the fake providers cannot create S3 buckets, and the point
// of the test is the state sequence, not a provider's bucket call.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nivis-project/nivis/internal/fakes3"
)

// bootstrapRunner builds the CLI and returns a runner bound to one state path,
// backend location, and fake S3 endpoint.
func bootstrapRunner(t *testing.T, root, statePath, bucket, key, endpoint string) func(t *testing.T, args ...string) (string, error) {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "nivis")
	build := exec.Command("go", "build", "-o", bin, "./cmd/nivis")
	build.Dir = root
	if out, err := build.CombinedOutput(); err != nil {
		t.Skipf("cannot build nivis: %v\n%s", err, out)
	}

	return func(t *testing.T, args ...string) (string, error) {
		t.Helper()
		full := append([]string{}, args...)
		full = append(full,
			"--attr", "nivis.bootstrapState",
			"--state", statePath,
			"--var", "state_bucket="+bucket,
			"--var", "state_key="+key,
			"--var", "state_region=us-east-1",
			"--var", "state_endpoint="+endpoint,
		)
		cmd := exec.Command(bin, full...)
		cmd.Dir = root
		cmd.Env = append(os.Environ(),
			"NO_COLOR=1",
			// The SDK insists on credentials; the fake ignores them.
			"AWS_ACCESS_KEY_ID=test",
			"AWS_SECRET_ACCESS_KEY=test",
			"AWS_REGION=us-east-1",
		)
		out, err := cmd.CombinedOutput()
		return string(out), err
	}
}

// The documented bootstrap works from a clean checkout: apply locally, migrate the
// state into the bucket the run created, then apply against the declared backend
// with no changes.
func TestSelfManagedBucketBootstrap(t *testing.T) {
	requireNix(t)
	root := repoRoot(t)
	buildBinaries(t, root) // fake providers on $PATH
	t.Setenv("TERRAE_NIVIS_FAKE_COUNTER", "")

	srv := fakes3.NewWithBuckets() // no bucket exists yet
	defer srv.Close()
	bucket, key := "nivis-bootstrap-state", "bootstrap/app.json"
	statePath := filepath.Join(t.TempDir(), "nivis.state.json")
	run := bootstrapRunner(t, root, statePath, bucket, key, srv.URL())

	// 1. The first apply cannot use the declared backend (its bucket is what this
	// run creates), so it runs against local state and says so.
	out, err := run(t, "apply", "--backend=local")
	if err != nil {
		t.Fatalf("apply --backend=local: %v\n%s", err, out)
	}
	if !strings.Contains(out, "--backend=local overrides the s3 backend") {
		t.Errorf("the override should be announced:\n%s", out)
	}
	if !strings.Contains(out, "+ alpha.alpha_token.app") {
		t.Errorf("the first apply should create the resources:\n%s", out)
	}
	if _, err := os.Stat(statePath); err != nil {
		t.Fatalf("the local state file was not written: %v", err)
	}
	if srv.Has(bucket, key) {
		t.Error("the local-backend run wrote to the remote backend")
	}

	// The configuration created its own state bucket during that apply.
	srv.CreateBucket(bucket)

	// 2. Move the state document into the bucket.
	out, err = run(t, "state", "migrate", "--to-remote")
	if err != nil {
		t.Fatalf("state migrate --to-remote: %v\n%s", err, out)
	}
	if !strings.Contains(out, "Moved 2 resource(s)") {
		t.Errorf("the migration should report what moved:\n%s", out)
	}
	if !srv.Has(bucket, key) {
		t.Fatalf("the state document is not in the bucket:\n%s", out)
	}
	if _, err := os.Stat(statePath); !os.IsNotExist(err) {
		t.Errorf("the local state file survived the migration (err = %v)", err)
	}

	// 3. From here on the declared backend is the state of record, and the stack
	// is converged: no creates, only no-ops.
	out, err = run(t, "apply")
	if err != nil {
		t.Fatalf("apply against the declared backend: %v\n%s", err, out)
	}
	if strings.Contains(out, "+ alpha.alpha_token.app") || strings.Contains(out, "+ beta.beta_record.app") {
		t.Errorf("the post-migration apply re-created resources:\n%s", out)
	}
	if !strings.Contains(out, "= alpha.alpha_token.app") || !strings.Contains(out, "= beta.beta_record.app") {
		t.Errorf("the post-migration apply should report no-ops:\n%s", out)
	}
	if strings.Contains(out, "--backend=local") {
		t.Errorf("the third run should use the declared backend, not local state:\n%s", out)
	}
}

// The negative case: a declared bucket that nothing creates is a misconfiguration.
// It must fail with the actionable missing-bucket message rather than proceeding
// against an empty state (which would re-create everything).
func TestMissingStateBucketIsAnErrorNotEmptyState(t *testing.T) {
	requireNix(t)
	root := repoRoot(t)
	buildBinaries(t, root)
	t.Setenv("TERRAE_NIVIS_FAKE_COUNTER", "")

	srv := fakes3.NewWithBuckets() // the bucket is never created
	defer srv.Close()
	statePath := filepath.Join(t.TempDir(), "nivis.state.json")
	run := bootstrapRunner(t, root, statePath, "typo-in-the-bucket-name", "bootstrap/app.json", srv.URL())

	out, err := run(t, "plan")
	if err == nil {
		t.Fatalf("plan against a missing bucket should fail, got:\n%s", out)
	}
	for _, want := range []string{
		"typo-in-the-bucket-name", "us-east-1",
		"nivis apply --backend=local", "nivis state migrate --to-remote",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("the failure should mention %q:\n%s", want, out)
		}
	}

	// An apply fails the same way, and creates nothing.
	out, err = run(t, "apply")
	if err == nil {
		t.Fatalf("apply against a missing bucket should fail, got:\n%s", out)
	}
	if strings.Contains(out, "+ alpha.alpha_token.app") {
		t.Errorf("apply created resources despite the unreachable state location:\n%s", out)
	}
	if !strings.Contains(out, "does not exist") {
		t.Errorf("the failure should name the missing bucket:\n%s", out)
	}
}
