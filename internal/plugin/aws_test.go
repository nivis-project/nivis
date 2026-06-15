// Copyright 2026 WeareTechnative B.V. and the terrae-nivis authors
// SPDX-License-Identifier: Apache-2.0

package plugin_test

import (
	"context"
	"os"
	"testing"

	"github.com/wearetechnative/terrae-nivis/internal/plugin"
	"github.com/wearetechnative/terrae-nivis/internal/provider"
	"github.com/wearetechnative/terrae-nivis/internal/registry"
)

// TestAWSConfigureAndPlan is gated by TERRAE_NIVIS_NET_TESTS=1 and requires AWS
// credentials in the environment (e.g. AWS_PROFILE + AWS_REGION). It fetches the
// real AWS provider via the registry, lets the manager negotiate the protocol
// (v5) and Configure it (all-null config -> the AWS SDK default credential/region
// chain), then PLANS aws_s3_bucket with no inputs. Read-only: no resource is
// created. This validates the whole real-provider stack end to end.
func TestAWSConfigureAndPlan(t *testing.T) {
	if os.Getenv("TERRAE_NIVIS_NET_TESTS") != "1" {
		t.Skip("net/creds test; set TERRAE_NIVIS_NET_TESTS=1 with AWS creds")
	}
	ctx := context.Background()

	mgr := plugin.NewManager().WithResolver(registry.New(""))
	defer mgr.Close()

	// "hashicorp/aws" resolves via the registry; configure with empty config so
	// region/creds come from the environment (AWS_PROFILE/AWS_REGION).
	client, err := mgr.Client("aws", "hashicorp/aws", map[string]interface{}{})
	if err != nil {
		t.Fatalf("fetch+spawn+configure aws: %v", err)
	}

	rs, err := client.GetSchema(ctx, "aws_s3_bucket")
	if err != nil {
		t.Fatalf("schema: %v", err)
	}

	pr, err := client.Plan(ctx, provider.PlanRequest{
		Schema:      rs,
		TypeName:    "aws_s3_bucket",
		ResolvedCfg: map[string]interface{}{}, // no inputs; bucket name generated
	})
	if err != nil {
		t.Fatalf("plan aws_s3_bucket: %v", err)
	}
	if pr.PlannedState == nil {
		t.Fatal("expected a planned state from a real AWS plan")
	}
	t.Logf("AWS plan ok: %d attrs known-after-apply: %v", len(pr.UnknownAfterApply), pr.UnknownAfterApply)
}
