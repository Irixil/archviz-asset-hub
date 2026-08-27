package events

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"damask/server/internal/auth"
	"damask/server/internal/telemetry"

	"github.com/gofiber/fiber/v3"
)

const (
	sseEventTicker = 10 * time.Second
	subBufferSize  = 8 // fixed channel buffer size for all subscribers
	replayBufSize  = 256
	// dropLogEvery logs the 1st drop then every Nth after, so a stuck slow
	// consumer doesn't flood logs at warn level.
	dropLogEvery = 50
)

type EventHub interface {
	Subscribe(workspaceID, lastEventID string) (<-chan Event, []Event, func())
	Publish(ctx context.Context, workspaceID string, ev Event)
	EventHandler(c fiber.Ctx) error
	// DropCount returns the total number of events dropped across all
	// subscribers due to slow consumers, since process start.
	DropCount() uint64
}

type subscriber struct {
	ch    chan Event
	drops uint64
}

type workspaceState struct {
	subs   map[uint64]*subscriber
	buffer []Event // ordered ascending by ID, capped at replayBufSize
	nextID uint64
}

type EventHubImpl struct {
	mu         sync.Mutex
	ws         map[string]*workspaceState
	subCounter uint64
	totalDrops atomic.Uint64
}

// NewEventHub returns a workspace-scoped SSE broadcast bus.
func NewEventHub() EventHub {
	return &EventHubImpl{
		ws: make(map[string]*workspaceState),
	}
}

func (h *EventHubImpl) workspace(workspaceID string) *workspaceState {
	ws, ok := h.ws[workspaceID]
	if !ok {
		ws = &workspaceState{subs: make(map[uint64]*subscriber)}
		h.ws[workspaceID] = ws
	}
	return ws
}

// Subscribe registers a new subscriber for workspaceID. It returns the live
// event channel, a replay slice of events missed since lastEventID (empty if
// lastEventID is empty, unknown, or current), and a cancel func that must be
// called when the connection closes.
//
// If lastEventID has fallen out of the replay buffer, the replay slice's
// final event is a resync event and no earlier events are replayed.
func (h *EventHubImpl) Subscribe(workspaceID, lastEventID string) (<-chan Event, []Event, func()) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.subCounter++
	id := h.subCounter
	sub := &subscriber{ch: make(chan Event, subBufferSize)}

	ws := h.workspace(workspaceID)
	ws.subs[id] = sub

	replay := ws.replaySince(lastEventID)

	return sub.ch, replay, func() {
		h.mu.Lock()
		defer h.mu.Unlock()
		if wsState, ok := h.ws[workspaceID]; ok {
			delete(wsState.subs, id)
			close(sub.ch)
			if len(wsState.subs) == 0 {
				delete(h.ws, workspaceID)
			}
		}
	}
}

// replaySince returns events with ID greater than lastEventID. If
// lastEventID is empty it returns nil (fresh connection, no replay). If
// lastEventID is non-empty but no longer covered by the buffer, it returns a
// single resync event.
func (ws *workspaceState) replaySince(lastEventID string) []Event {
	if lastEventID == "" {
		return nil
	}
	last, err := strconv.ParseUint(lastEventID, 10, 64)
	if err != nil {
		return []Event{Resync("invalid Last-Event-ID")}
	}
	if len(ws.buffer) == 0 {
		if last == ws.nextID {
			return nil // caller is fully caught up, no gap
		}
		return []Event{Resync("no buffered events")}
	}
	if last == ws.nextID {
		return nil // caller is fully caught up, no gap
	}
	if last > ws.nextID {
		// Client claims an ID this incarnation never issued (e.g. a process
		// restart reset nextID). It can't have seen that event; force resync
		// rather than silently returning zero replayed events.
		return []Event{Resync("stale last-event-id")}
	}
	oldest, err := strconv.ParseUint(ws.buffer[0].ID, 10, 64)
	if err != nil || last+1 < oldest {
		return []Event{Resync("buffer overrun")}
	}
	out := make([]Event, 0, len(ws.buffer))
	for _, ev := range ws.buffer {
		evID, parseErr := strconv.ParseUint(ev.ID, 10, 64)
		if parseErr == nil && evID > last {
			out = append(out, ev)
		}
	}
	return out
}

// Publish sends an event to all subscribers of workspaceID.
// Non-blocking: slow clients are skipped and counted as drops.
func (h *EventHubImpl) Publish(ctx context.Context, workspaceID string, ev Event) {
	var err error
	_, span := telemetry.StartSpan(ctx, "service.events.publish")
	defer func() { telemetry.EndSpan(span, err) }()

	h.mu.Lock()
	ws := h.workspace(workspaceID)
	ws.nextID++
	ev.WorkspaceID = workspaceID
	ev.ID = strconv.FormatUint(ws.nextID, 10)

	ws.buffer = append(ws.buffer, ev)
	if len(ws.buffer) > replayBufSize {
		ws.buffer = ws.buffer[len(ws.buffer)-replayBufSize:]
	}

	slog.DebugContext(ctx, "publishing event", "workspace_id", workspaceID, "event_type", ev.Type, "event_id", ev.ID)

	for _, sub := range ws.subs {
		select {
		case sub.ch <- ev:
		default: // subscriber too slow; drop
			h.recordDrop(ctx, workspaceID, ev, sub)
		}
	}
	h.mu.Unlock()
}

func (h *EventHubImpl) recordDrop(ctx context.Context, workspaceID string, ev Event, sub *subscriber) {
	sub.drops++
	total := h.totalDrops.Add(1)
	if sub.drops == 1 || sub.drops%dropLogEvery == 0 {
		slog.WarnContext(ctx, "sse subscriber dropped event",
			"workspace_id", workspaceID,
			"event_type", ev.Type,
			"subscriber_drops", sub.drops,
			"total_drops", total,
		)
	}
}

// DropCount returns the total number of events dropped across all
// subscribers due to slow consumers, since process start.
func (h *EventHubImpl) DropCount() uint64 {
	return h.totalDrops.Load()
}

func (h *EventHubImpl) EventHandler(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	lastEventID := c.Get("Last-Event-ID")

	c.Set("Content-Type", "text/event-stream")
	c.Set("Cache-Control", "no-cache")
	c.Set("Connection", "keep-alive")
	c.Set("X-Accel-Buffering", "no")

	ch, replay, cancel := h.Subscribe(claims.WorkspaceID, lastEventID)

	return c.SendStreamWriter(func(w *bufio.Writer) {
		defer cancel()

		_, _ = fmt.Fprintf(w, ": connected\n\n")
		if err := w.Flush(); err != nil {
			return
		}

		for _, ev := range replay {
			if !writeEvent(w, ev) {
				return
			}
		}
		if err := w.Flush(); err != nil {
			return
		}

		ticker := time.NewTicker(sseEventTicker)
		defer ticker.Stop()

		for {
			select {
			case ev, ok := <-ch:
				if !ok {
					return
				}
				if !writeEvent(w, ev) {
					return
				}
				if err := w.Flush(); err != nil {
					return
				}
			case <-ticker.C:
				_, _ = fmt.Fprintf(w, ": heartbeat\n\n")
				if err := w.Flush(); err != nil {
					return
				}
			}
		}
	})
}

// writeEvent writes a single SSE frame, including the id: line used by
// EventSource to track Last-Event-ID for reconnects. Returns false if the
// write failed and the stream should close.
func writeEvent(w *bufio.Writer, ev Event) bool {
	data, err := json.Marshal(ev)
	if err != nil {
		return true // skip malformed event, keep connection alive
	}
	_, werr := fmt.Fprintf(w, "id: %s\ndata: %s\n\n", ev.ID, data)
	return werr == nil
}
