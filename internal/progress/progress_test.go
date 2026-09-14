// Copyright 2026 TechNative B.V. and the nivis authors
// SPDX-License-Identifier: Apache-2.0

package progress_test

import (
	"errors"
	"testing"
	"time"

	"github.com/nivis-project/nivis/internal/plan"
	"github.com/nivis-project/nivis/internal/progress"
)

// A nil observer is the "no observer" case every engine starts from. It must be
// safe for every kind, or adding an emit site to an engine would break every
// caller that does not want progress.
func TestNilObserverIsSafeForEveryKind(t *testing.T) {
	kinds := []progress.Kind{
		progress.EvalStart, progress.EvalDone, progress.PhaseStart,
		progress.NodeStart, progress.NodeDone,
		progress.BuildStart, progress.BuildDone,
		progress.ProviderSpawn, progress.Note,
	}
	for _, k := range kinds {
		// Must not panic.
		progress.Emit(nil, progress.Event{Kind: k})
	}
}

// Every kind must survive the round trip to an observer unchanged: the renderer
// reads these fields directly, so a dropped one is a silently missing detail.
func TestEveryKindReachesTheObserverIntact(t *testing.T) {
	boom := errors.New("boom")
	want := []progress.Event{
		{Kind: progress.EvalStart, Phase: 2},
		{Kind: progress.EvalDone, Phase: 2, Duration: 1400 * time.Millisecond},
		{Kind: progress.EvalDone, Phase: 3, Err: boom},
		{Kind: progress.PhaseStart, Phase: 1, Count: 6},
		{Kind: progress.NodeStart, ID: "aws.aws_s3_bucket.b", Type: "aws_s3_bucket", Op: plan.OpCreate},
		{Kind: progress.NodeStart, ID: "aws.data.aws_ami.x", IsData: true},
		{
			Kind: progress.NodeDone, ID: "aws.aws_s3_bucket.b", Type: "aws_s3_bucket",
			Op: plan.OpUpdate, ResourceID: "bucket-123", Duration: time.Second,
		},
		{Kind: progress.NodeDone, ID: "aws.aws_instance.i", Drifted: true},
		{Kind: progress.NodeDone, ID: "aws.aws_instance.i", Err: boom},
		{Kind: progress.BuildStart, Owner: "aws.aws_s3_object.o", Name: "nixos-image", Path: "/nix/store/h-nixos-image"},
		{Kind: progress.BuildDone, Owner: "aws.aws_s3_object.o", Name: "nixos-image", Duration: 2 * time.Minute},
		{Kind: progress.ProviderSpawn, Name: "aws"},
		{Kind: progress.Note, Message: "provider said something"},
	}

	var got []progress.Event
	obs := progress.Func(func(e progress.Event) { got = append(got, e) })
	for _, e := range want {
		progress.Emit(obs, e)
	}

	if len(got) != len(want) {
		t.Fatalf("got %d events, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("event %d:\n got %+v\nwant %+v", i, got[i], want[i])
		}
	}
}

// Op is meaningful only for a resource. A datasource carries IsData and its Op
// must be ignored rather than rendered as a create.
func TestDatasourceCarriesIsData(t *testing.T) {
	var got progress.Event
	obs := progress.Func(func(e progress.Event) { got = e })
	progress.Emit(obs, progress.Event{Kind: progress.NodeDone, ID: "d", IsData: true})
	if !got.IsData {
		t.Error("IsData lost; a datasource read would render as a create")
	}
}
