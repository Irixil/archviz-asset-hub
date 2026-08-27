package service_test

import (
	"context"
	"maps"
	"sync"

	"damask/server/internal/audit"
)

// spyWriter captures audit events for use in tests.
type spyWriter struct {
	mu      sync.Mutex
	asset   []audit.AssetEvent
	project []audit.ProjectEvent
}

func newSpy() *spyWriter { return &spyWriter{} }

func (s *spyWriter) WriteAsset(_ context.Context, e audit.AssetEvent) {
	s.mu.Lock()
	s.asset = append(s.asset, e)
	s.mu.Unlock()
}

func (s *spyWriter) WriteAssetAsync(e audit.AssetEvent) {
	s.WriteAsset(context.Background(), e)
}

func (s *spyWriter) WriteProject(_ context.Context, e audit.ProjectEvent) {
	s.mu.Lock()
	s.project = append(s.project, e)
	s.mu.Unlock()
}

// lastAsset returns the most recently captured AssetEvent, or the zero value.
func (s *spyWriter) lastAsset() audit.AssetEvent {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.asset) == 0 {
		return audit.AssetEvent{}
	}
	return s.asset[len(s.asset)-1]
}

// lastProject returns the most recently captured ProjectEvent, or the zero value.
func (s *spyWriter) lastProject() audit.ProjectEvent {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.project) == 0 {
		return audit.ProjectEvent{}
	}
	return s.project[len(s.project)-1]
}

// assetCount returns how many asset events have been captured.
func (s *spyWriter) assetCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.asset)
}

type triggerCall struct {
	eventType string
	data      map[string]any
}

type triggerSpy struct {
	mu    sync.Mutex
	calls []triggerCall
	err   error
}

func (s *triggerSpy) Dispatch(_ context.Context, eventType string, data map[string]any) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	cp := make(map[string]any, len(data))
	maps.Copy(cp, data)
	s.calls = append(s.calls, triggerCall{eventType: eventType, data: cp})
	return s.err
}

func (s *triggerSpy) count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.calls)
}

func (s *triggerSpy) last() triggerCall {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.calls) == 0 {
		return triggerCall{}
	}
	return s.calls[len(s.calls)-1]
}
