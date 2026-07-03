// Copyright 2026 TechNative B.V. and the nivis authors
// SPDX-License-Identifier: Apache-2.0

package plugin_test

import (
	"context"
	"os"
	"testing"

	"github.com/nivis-project/nivis/internal/plugin"
	"github.com/nivis-project/nivis/internal/provider"
	"github.com/nivis-project/nivis/internal/registry"
)

// TestAWSApplyDestroyS3Bucket is gated by TERRAE_NIVIS_NET_TESTS=1 and AWS creds. It
// CREATES a real S3 bucket and then DESTROYS it, proving the full real-provider
// round trip (plan -> apply -> destroy) against AWS.
//
// Safety:
//   - `bucket` is omitted, so AWS assigns a globally-unique generated name.
//   - `force_destroy = true` so the bucket can be deleted unconditionally.
//   - The manager (provider process) is closed via t.Cleanup, and the destroy is
//     registered with t.Cleanup AFTER it — cleanups run LIFO, so destroy runs
//     while the provider is still alive, then the process is closed. A failed
//     destroy is reported loudly so the bucket can be cleaned up manually.
func TestAWSApplyDestroyS3Bucket(t *testing.T) {
	if os.Getenv("TERRAE_NIVIS_NET_TESTS") != "1" {
		t.Skip("net/creds test; set TERRAE_NIVIS_NET_TESTS=1 with AWS creds")
	}
	ctx := context.Background()

	mgr := plugin.NewManager().WithResolver(registry.New(""))
	// Close the provider process LAST (registered first => runs last under LIFO).
	t.Cleanup(mgr.Close)

	client, err := mgr.Client("aws", "hashicorp/aws", map[string]interface{}{})
	if err != nil {
		t.Fatalf("configure aws: %v", err)
	}
	rs, err := client.GetSchema(ctx, "aws_s3_bucket")
	if err != nil {
		t.Fatalf("schema: %v", err)
	}

	cfg := map[string]interface{}{
		"force_destroy": true,
		"tags":          map[string]interface{}{"nivis-test": "apply-destroy"},
	}

	pr, err := client.Plan(ctx, provider.PlanRequest{Schema: rs, TypeName: "aws_s3_bucket", ResolvedCfg: cfg})
	if err != nil {
		t.Fatalf("plan: %v", err)
	}

	ar, err := client.Apply(ctx, provider.ApplyRequest{
		Schema: rs, TypeName: "aws_s3_bucket", ResolvedCfg: cfg, PlannedState: pr.PlannedState,
	})
	if err != nil {
		t.Fatalf("apply (create): %v", err)
	}
	bucket, _ := ar.Attrs["bucket"].(string)
	t.Logf("CREATED bucket=%q arn=%v", bucket, ar.Attrs["arn"])

	// Register teardown AFTER apply (runs before mgr.Close under LIFO, so the
	// provider is still alive). Guarantees the bucket is destroyed even if the
	// assertions below fail.
	destroyed := false
	t.Cleanup(func() {
		if destroyed {
			return
		}
		if _, derr := client.Destroy(ctx, provider.DestroyRequest{
			Schema: rs, TypeName: "aws_s3_bucket", Stored: ar.Attrs,
		}); derr != nil {
			t.Errorf("DESTROY FAILED — manually delete bucket %q: %v", bucket, derr)
		} else {
			t.Logf("DESTROYED bucket=%q (via cleanup)", bucket)
		}
	})

	if bucket == "" {
		t.Error("expected a generated bucket name in apply output")
	}
	if ar.Attrs["arn"] == nil || ar.Attrs["arn"] == "" {
		t.Error("expected a computed arn in apply output")
	}

	// Destroy inline as the happy path (and mark done so cleanup is a no-op).
	if _, err := client.Destroy(ctx, provider.DestroyRequest{
		Schema: rs, TypeName: "aws_s3_bucket", Stored: ar.Attrs,
	}); err != nil {
		t.Fatalf("destroy: %v", err) // cleanup will retry
	}
	destroyed = true
	t.Logf("DESTROYED bucket=%q", bucket)
}
