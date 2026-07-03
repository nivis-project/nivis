// Copyright 2026 TechNative B.V. and the nivis authors
// SPDX-License-Identifier: Apache-2.0

package plugin_test

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/nivis-project/nivis/internal/plugin"
	"github.com/nivis-project/nivis/internal/provider"
	"github.com/nivis-project/nivis/internal/registry"
)

// TestAWSEC2NixOSServesHTTP is gated by TERRAE_NIVIS_NET_TESTS=1 + AWS creds, and
// takes a NixOS AMI id (one that runs an HTTP server on :80) via
// NIVIS_TEST_AMI. It launches an aws_security_group (ingress :80) and an
// aws_instance from that AMI, polls the instance's public IP until port 80
// returns HTTP 200 (the beans-rx5h "tested outcome"), then destroys both and
// confirms teardown. The AMI build/upload/register pipeline is exercised by the
// tutorial's `nivis apply` (nix/example/ec2.nix); this isolates the launch + the
// HTTP-200 assertion.
func TestAWSEC2NixOSServesHTTP(t *testing.T) {
	if os.Getenv("TERRAE_NIVIS_NET_TESTS") != "1" {
		t.Skip("net/creds test; set TERRAE_NIVIS_NET_TESTS=1 with AWS creds")
	}
	ami := os.Getenv("NIVIS_TEST_AMI")
	if ami == "" {
		t.Skip("set NIVIS_TEST_AMI to a NixOS AMI that serves HTTP on :80")
	}
	ctx := context.Background()

	mgr := plugin.NewManager().WithResolver(registry.New(""))
	t.Cleanup(mgr.Close) // provider process closed last

	client, err := mgr.Client("aws", "hashicorp/aws", map[string]interface{}{"region": "eu-central-1"})
	if err != nil {
		t.Fatalf("configure aws: %v", err)
	}

	// --- security group: ingress :80 -----------------------------------------
	sgSchema, err := client.GetSchema(ctx, "aws_security_group")
	if err != nil {
		t.Fatalf("sg schema: %v", err)
	}
	sgCfg := map[string]interface{}{
		"name":        fmt.Sprintf("nivis-ec2nix-test-%d", time.Now().Unix()),
		"description": "nivis ec2+nixos e2e: http",
		"ingress": []interface{}{map[string]interface{}{
			"from_port": float64(80), "to_port": float64(80), "protocol": "tcp",
			"cidr_blocks": []interface{}{"0.0.0.0/0"}, "description": "http",
		}},
		"egress": []interface{}{map[string]interface{}{
			"from_port": float64(0), "to_port": float64(0), "protocol": "-1",
			"cidr_blocks": []interface{}{"0.0.0.0/0"}, "description": "all",
		}},
	}
	sgPlan, err := client.Plan(ctx, provider.PlanRequest{Schema: sgSchema, TypeName: "aws_security_group", ResolvedCfg: sgCfg})
	if err != nil {
		t.Fatalf("sg plan: %v", err)
	}
	sg, err := client.Apply(ctx, provider.ApplyRequest{Schema: sgSchema, TypeName: "aws_security_group", ResolvedCfg: sgCfg, PlannedState: sgPlan.PlannedState})
	if err != nil {
		t.Fatalf("sg apply: %v", err)
	}
	sgAttrs := sg.Attrs
	t.Cleanup(func() {
		if _, derr := client.Destroy(ctx, provider.DestroyRequest{Schema: sgSchema, TypeName: "aws_security_group", Stored: sgAttrs}); derr != nil {
			t.Errorf("cleanup: destroy SG: %v", derr)
		}
	})

	// --- instance from the NixOS AMI -----------------------------------------
	instSchema, err := client.GetSchema(ctx, "aws_instance")
	if err != nil {
		t.Fatalf("instance schema: %v", err)
	}
	instCfg := map[string]interface{}{
		"ami":                    ami,
		"instance_type":          "t3.micro",
		"vpc_security_group_ids": []interface{}{sgAttrs["id"]},
		"tags":                   map[string]interface{}{"Name": "nivis-ec2nix-e2e", "managed-by": "nivis"},
	}
	instPlan, err := client.Plan(ctx, provider.PlanRequest{Schema: instSchema, TypeName: "aws_instance", ResolvedCfg: instCfg})
	if err != nil {
		t.Fatalf("instance plan: %v", err)
	}
	inst, err := client.Apply(ctx, provider.ApplyRequest{Schema: instSchema, TypeName: "aws_instance", ResolvedCfg: instCfg, PlannedState: instPlan.PlannedState})
	if err != nil {
		t.Fatalf("instance apply: %v", err)
	}
	instAttrs := inst.Attrs
	t.Cleanup(func() {
		if _, derr := client.Destroy(ctx, provider.DestroyRequest{Schema: instSchema, TypeName: "aws_instance", Stored: instAttrs}); derr != nil {
			t.Errorf("cleanup: destroy instance: %v (terminate manually)", derr)
		}
	})

	ip, _ := instAttrs["public_ip"].(string)
	if ip == "" {
		t.Fatalf("instance has no public_ip; attrs=%v", instAttrs)
	}
	t.Logf("instance %v public_ip=%s — polling :80 for HTTP 200", instAttrs["id"], ip)

	// --- poll :80 until HTTP 200 (boot + nginx start take a bit) --------------
	url := "http://" + ip + "/"
	deadline := time.Now().Add(4 * time.Minute)
	httpc := &http.Client{Timeout: 5 * time.Second}
	for {
		resp, err := httpc.Get(url)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				t.Logf("OK: %s returned 200", url)
				return
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s to return 200 (last err=%v)", url, err)
		}
		time.Sleep(10 * time.Second)
	}
}
