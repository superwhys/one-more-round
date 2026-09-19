package photo

import (
	"testing"
	"time"
)

func TestPhotoLifecyclePreventsPartialReadsAndAttachment(t *testing.T) {
	for _, state := range []State{StateUploading, StateDeleting} {
		p := &Photo{Owner: "owner", State: state}
		if p.ReadableBy("owner") || p.Attach("round", "owner") == nil {
			t.Fatalf("photo in state %s became available", state)
		}
	}
	p := &Photo{Owner: "owner", State: StateReady}
	if !p.ReadableBy("owner") || p.ReadableBy("other") {
		t.Fatal("unattached access changed")
	}
	if err := p.Attach("round", "owner"); err != nil || !p.ReadableBy("other") {
		t.Fatal("attached photo not available to members")
	}
}

func TestCleanupClaimAndRetention(t *testing.T) {
	now := time.Now().UTC()
	p := &Photo{State: StateReady, Created: now.Add(-8 * 24 * time.Hour), RoundID: "round"}
	if p.BeginDeletion(now) {
		t.Fatal("attached photo claimed")
	}
	p.Detach(now)
	if p.BeginDeletion(now.Add(-7 * 24 * time.Hour)) {
		t.Fatal("detached photo did not get a fresh retention window")
	}
	if !p.BeginDeletion(now.Add(time.Second)) || p.State != StateDeleting {
		t.Fatal("expired photo not claimed")
	}
	if p.Attach("new-round", "") == nil || !p.BeginDeletion(now.Add(-24*time.Hour)) {
		t.Fatal("claimed photo must remain unavailable and retryable")
	}
}
