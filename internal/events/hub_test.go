package events

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

const recvTimeout = 2 * time.Second

func recv(t *testing.T, ch <-chan Event) Event {
	t.Helper()
	select {
	case ev := <-ch:
		return ev
	case <-time.After(recvTimeout):
		t.Fatal("timed out waiting for event")
		return Event{}
	}
}

func assertNoEvent(t *testing.T, ch <-chan Event) {
	t.Helper()
	select {
	case ev, ok := <-ch:
		if ok {
			t.Fatalf("expected no event, got %+v", ev)
		}
	case <-time.After(100 * time.Millisecond):
	}
}

func TestWorkspaceSegregation(t *testing.T) {
	hub := NewEventHub().(*EventHubImpl)
	ctx := context.Background()

	chA, _, cancelA := hub.Subscribe("ws_a", "")
	defer cancelA()
	chB, _, cancelB := hub.Subscribe("ws_b", "")
	defer cancelB()

	hub.Publish(ctx, "ws_a", ThumbnailReady("asset_1", "thumb_1"))

	got := recv(t, chA)
	if got.Type != TypeThumbnailReady {
		t.Fatalf("workspace A did not receive its own event: %+v", got)
	}
	assertNoEvent(t, chB)
}

func TestWorkspaceSegregationOnReplayPath(t *testing.T) {
	hub := NewEventHub().(*EventHubImpl)
	ctx := context.Background()

	// Publish a few events to ws_a to populate its buffer and obtain a real ID.
	_, replayA, cancelA := hub.Subscribe("ws_a", "")
	cancelA()
	hub.Publish(ctx, "ws_a", ThumbnailReady("asset_1", "thumb_1"))
	hub.Publish(ctx, "ws_a", ThumbnailReady("asset_1", "thumb_2"))
	_ = replayA

	// Grab workspace A's latest event ID to use as a Last-Event-ID on workspace B.
	hub.mu.Lock()
	wsA := hub.ws["ws_a"]
	lastIDFromA := wsA.buffer[len(wsA.buffer)-1].ID
	hub.mu.Unlock()

	// Now publish independent events to workspace B and connect with A's ID.
	hub.Publish(ctx, "ws_b", ThumbnailReady("asset_9", "thumb_9"))
	hub.Publish(ctx, "ws_b", ThumbnailReady("asset_9", "thumb_10"))

	_, replayB, cancelB := hub.Subscribe("ws_b", lastIDFromA)
	defer cancelB()

	for _, ev := range replayB {
		if ev.Type == TypeResync {
			continue
		}
		if ev.WorkspaceID != "" && ev.WorkspaceID != "ws_b" {
			t.Fatalf("replay leaked event from another workspace: %+v", ev)
		}
		// Confirm no payload from ws_a's asset_1 events leaked.
		if ev.Payload != nil {
			var p ThumbnailReadyPayload
			_ = json.Unmarshal(ev.Payload, &p)
			if p.AssetID == "asset_1" {
				t.Fatalf("replay leaked workspace A event: %+v", ev)
			}
		}
	}
}

func TestReplayDeliversAllMissedEventsInOrder(t *testing.T) {
	hub := NewEventHub().(*EventHubImpl)
	ctx := context.Background()

	// Establish a baseline subscriber to learn the "connected" ID, then disconnect.
	_, _, cancel := hub.Subscribe("ws_1", "")
	cancel()

	hub.Publish(ctx, "ws_1", ThumbnailReady("asset_1", "thumb_1"))
	hub.Publish(ctx, "ws_1", VariantReady("asset_1", "variant_1"))
	hub.Publish(ctx, "ws_1", StackMergeDone("asset_1", "job_1"))

	_, replay, cancel2 := hub.Subscribe("ws_1", "0")
	defer cancel2()

	if len(replay) != 3 {
		t.Fatalf("expected 3 replayed events, got %d: %+v", len(replay), replay)
	}
	wantTypes := []string{TypeThumbnailReady, TypeVariantReady, TypeStackMergeDone}
	for i, ev := range replay {
		if ev.Type != wantTypes[i] {
			t.Fatalf("replay[%d].Type = %q, want %q", i, ev.Type, wantTypes[i])
		}
	}

	// Reconnecting with the last delivered ID should replay nothing further.
	_, replay2, cancel3 := hub.Subscribe("ws_1", replay[len(replay)-1].ID)
	defer cancel3()
	if len(replay2) != 0 {
		t.Fatalf("expected no replay after catching up, got %+v", replay2)
	}
}

func TestReplayBufferOverrunTriggersResync(t *testing.T) {
	hub := NewEventHub().(*EventHubImpl)
	ctx := context.Background()

	_, _, cancel := hub.Subscribe("ws_1", "")
	cancel()

	for range replayBufSize + 10 {
		hub.Publish(ctx, "ws_1", ThumbnailReady("asset_1", "thumb"))
	}

	// ID "1" has long since been evicted from the 256-entry ring buffer.
	_, replay, cancel2 := hub.Subscribe("ws_1", "1")
	defer cancel2()

	if len(replay) != 1 || replay[0].Type != TypeResync {
		t.Fatalf("expected a single resync event, got %+v", replay)
	}
}

func TestReplayStaleLastEventIDAcrossRestartTriggersResync(t *testing.T) {
	hub := NewEventHub().(*EventHubImpl)
	ctx := context.Background()

	_, _, cancel := hub.Subscribe("ws_1", "")
	cancel()

	// Simulate a fresh process: only a few events published, so nextID is
	// small, but the reconnecting client remembers a much larger ID from a
	// previous incarnation of the process.
	hub.Publish(ctx, "ws_1", ThumbnailReady("asset_1", "thumb_1"))
	hub.Publish(ctx, "ws_1", ThumbnailReady("asset_1", "thumb_2"))

	_, replay, cancel2 := hub.Subscribe("ws_1", "50")
	defer cancel2()

	if len(replay) != 1 || replay[0].Type != TypeResync {
		t.Fatalf("expected a single resync event for stale last-event-id, got %+v", replay)
	}
}

func TestSlowConsumerDropsAndConnectionStaysAlive(t *testing.T) {
	hub := NewEventHub().(*EventHubImpl)
	ctx := context.Background()

	ch, _, cancel := hub.Subscribe("ws_1", "")
	defer cancel()

	// Fill the subscriber's buffer (capacity subBufferSize) without reading,
	// then publish more to force drops.
	for range subBufferSize + 5 {
		hub.Publish(ctx, "ws_1", ThumbnailReady("asset_1", "thumb"))
	}

	if hub.DropCount() == 0 {
		t.Fatal("expected drop counter to increment for a slow consumer")
	}

	// Connection must still be alive: draining the channel yields buffered events.
	got := recv(t, ch)
	if got.Type != TypeThumbnailReady {
		t.Fatalf("expected connection to remain usable after drops, got %+v", got)
	}
}
