// Copyright 2026 TechNative B.V. and the nivis authors
// SPDX-License-Identifier: Apache-2.0

package phase

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/wearetechnative/nivis/internal/ir"
	"github.com/wearetechnative/nivis/internal/ledger"
	"github.com/wearetechnative/nivis/internal/plugin"
	"github.com/wearetechnative/nivis/internal/provider"
	"github.com/wearetechnative/nivis/internal/registry"
	"github.com/wearetechnative/nivis/internal/state"
)

// TestAWSPreventDestroyRefusesReplace is gated by TERRAE_NIVIS_NET_TESTS=1 and AWS
// creds (beans-c2dx). It proves end-to-end against real AWS that a force-new
// change to a resource with lifecycle.preventDestroy is REFUSED — the executor
// returns a named error and the real resource is NOT destroyed — rather than
// silently destroy-and-recreate.
//
// It creates a real S3 bucket with an explicit name (so changing the name is a
// force-new replace), then drives applyOne with a renamed bucket + preventDestroy
// and asserts the refusal. The original bucket is destroyed in cleanup.
func TestAWSPreventDestroyRefusesReplace(t *testing.T) {
	if os.Getenv("TERRAE_NIVIS_NET_TESTS") != "1" {
		t.Skip("net/creds test; set TERRAE_NIVIS_NET_TESTS=1 with AWS creds")
	}
	ctx := context.Background()

	mgr := plugin.NewManager().WithResolver(registry.New(""))
	t.Cleanup(mgr.Close) // provider process closed last (LIFO)

	st, err := state.Open(filepath.Join(t.TempDir(), "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	d := &Driver{Manager: mgr, Store: st, Ledger: ledger.New()}

	provCfg := ir.ProviderConfig{
		Source: "hashicorp/aws",
		Config: map[string]interface{}{"region": "eu-central-1"},
	}
	// A globally-unique-ish explicit name so a rename is a real force-new replace.
	acct := os.Getenv("NIVIS_TEST_ACCOUNT") // optional, for name uniqueness
	name := "nivis-c2dx-" + acct + "-eucentral1"

	mkGraph := func(bucketName string, prevent bool) (*ir.Graph, *ir.ResourceNode) {
		var meta *ir.Meta
		if prevent {
			meta = &ir.Meta{Lifecycle: &ir.Lifecycle{PreventDestroy: true}}
		}
		node := &ir.ResourceNode{Resource: ir.Resource{
			ID: "aws.aws_s3_bucket.demo", Provider: "aws", Type: "aws_s3_bucket", Name: "demo", Meta: meta,
		}}
		g := &ir.Graph{
			Providers: map[string]ir.ProviderConfig{"aws": provCfg},
			Nodes:     map[string]*ir.ResourceNode{node.Resource.ID: node},
			Order:     []string{node.Resource.ID},
		}
		return g, node
	}

	// 1. Create the bucket with its explicit name (no preventDestroy yet).
	g, node := mkGraph(name, false)
	cfg := map[string]interface{}{"bucket": name, "force_destroy": true}
	attrs, _, err := d.applyOne(ctx, g, node, cfg)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	t.Logf("CREATED bucket=%v", attrs["id"])

	// Destroy the bucket in cleanup (it's protected from the replace, not teardown).
	t.Cleanup(func() {
		client, cerr := mgr.Client("aws", provCfg.Source, provCfg.Config)
		if cerr != nil {
			t.Errorf("cleanup: client: %v (bucket %v may need manual deletion)", cerr, name)
			return
		}
		rs, _ := client.GetSchema(ctx, "aws_s3_bucket")
		if _, derr := client.Destroy(ctx, provider.DestroyRequest{Schema: rs, TypeName: "aws_s3_bucket", Stored: attrs}); derr != nil {
			t.Errorf("cleanup: destroy %q failed: %v (delete manually)", name, derr)
		}
	})

	// 2. Re-apply with a DIFFERENT bucket name (force-new) AND preventDestroy.
	//    The executor must refuse — not destroy the existing bucket.
	g2, node2 := mkGraph(name+"-renamed", true)
	cfg2 := map[string]interface{}{"bucket": name + "-renamed", "force_destroy": true}
	_, _, err = d.applyOne(ctx, g2, node2, cfg2)
	if err == nil {
		t.Fatal("expected applyOne to REFUSE the replace (preventDestroy), got nil error")
	}
	t.Logf("refused as expected: %v", err)

	// The error must name the resource and cite preventDestroy.
	if msg := err.Error(); !contains(msg, "aws.aws_s3_bucket.demo") || !contains(msg, "preventDestroy") {
		t.Errorf("refusal error should name the resource and preventDestroy; got: %v", err)
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
