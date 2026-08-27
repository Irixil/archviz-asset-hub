package mockservice

import (
	"context"

	"damask/server/internal/service"
)

// MockIngressService is a no-op implementation of service.IngressService.
type MockIngressService struct {
	ListSourcesFn    func(ctx context.Context, workspaceID string) ([]*service.IngressSourceDTO, error)
	GetSourceFn      func(ctx context.Context, workspaceID, id string) (*service.IngressSourceDTO, error)
	CreateSourceFn   func(ctx context.Context, workspaceID, userID string, p service.CreateIngressSourceParams) (*service.IngressSourceDTO, error)
	UpdateSourceFn   func(ctx context.Context, workspaceID, id string, p service.UpdateIngressSourceParams) (*service.IngressSourceDTO, error)
	DeleteSourceFn   func(ctx context.Context, workspaceID, id string) error
	TestSourceFn     func(ctx context.Context, workspaceID, id string) error
	TriggerPollFn    func(ctx context.Context, workspaceID, id string) (string, error)
	ListRulesFn      func(ctx context.Context, workspaceID, sourceID string) ([]*service.IngressRuleDTO, error)
	CreateRuleFn     func(ctx context.Context, workspaceID, sourceID string, p service.CreateIngressRuleParams) (*service.IngressRuleDTO, error)
	UpdateRuleFn     func(ctx context.Context, workspaceID, sourceID, ruleID string, p service.UpdateIngressRuleParams) (*service.IngressRuleDTO, error)
	DeleteRuleFn     func(ctx context.Context, workspaceID, sourceID, ruleID string) error
	ReorderRulesFn   func(ctx context.Context, workspaceID, sourceID string, entries []service.ReorderRuleEntry) ([]*service.IngressRuleDTO, error)
	ListLogFn        func(ctx context.Context, workspaceID, statusFilter string, limit, offset int64) ([]*service.IngressLogEntryDTO, error)
	ListSourceLogFn  func(ctx context.Context, workspaceID, sourceID string, limit, offset int64) ([]*service.IngressLogEntryDTO, error)
	DeleteLogEntryFn func(ctx context.Context, workspaceID, entryID string) error
	RetryLogEntryFn  func(ctx context.Context, workspaceID, entryID string) (string, error)
}

func NewIngressService() *MockIngressService { return &MockIngressService{} }

func (m *MockIngressService) ListSources(ctx context.Context, workspaceID string) ([]*service.IngressSourceDTO, error) {
	if m.ListSourcesFn != nil {
		return m.ListSourcesFn(ctx, workspaceID)
	}
	return nil, nil
}

func (m *MockIngressService) GetSource(ctx context.Context, workspaceID, id string) (*service.IngressSourceDTO, error) {
	if m.GetSourceFn != nil {
		return m.GetSourceFn(ctx, workspaceID, id)
	}
	return nil, nil //nolint:nilnil // mock
}

func (m *MockIngressService) CreateSource(
	ctx context.Context,
	workspaceID, userID string,
	p service.CreateIngressSourceParams,
) (*service.IngressSourceDTO, error) {
	if m.CreateSourceFn != nil {
		return m.CreateSourceFn(ctx, workspaceID, userID, p)
	}
	return nil, nil //nolint:nilnil // mock
}

func (m *MockIngressService) UpdateSource(
	ctx context.Context,
	workspaceID, id string,
	p service.UpdateIngressSourceParams,
) (*service.IngressSourceDTO, error) {
	if m.UpdateSourceFn != nil {
		return m.UpdateSourceFn(ctx, workspaceID, id, p)
	}
	return nil, nil //nolint:nilnil // mock
}

func (m *MockIngressService) DeleteSource(ctx context.Context, workspaceID, id string) error {
	if m.DeleteSourceFn != nil {
		return m.DeleteSourceFn(ctx, workspaceID, id)
	}
	return nil
}

func (m *MockIngressService) TestSource(ctx context.Context, workspaceID, id string) error {
	if m.TestSourceFn != nil {
		return m.TestSourceFn(ctx, workspaceID, id)
	}
	return nil
}

func (m *MockIngressService) TriggerPoll(ctx context.Context, workspaceID, id string) (string, error) {
	if m.TriggerPollFn != nil {
		return m.TriggerPollFn(ctx, workspaceID, id)
	}
	return "", nil
}

func (m *MockIngressService) ListRules(
	ctx context.Context,
	workspaceID, sourceID string,
) ([]*service.IngressRuleDTO, error) {
	if m.ListRulesFn != nil {
		return m.ListRulesFn(ctx, workspaceID, sourceID)
	}
	return nil, nil
}

func (m *MockIngressService) CreateRule(
	ctx context.Context,
	workspaceID, sourceID string,
	p service.CreateIngressRuleParams,
) (*service.IngressRuleDTO, error) {
	if m.CreateRuleFn != nil {
		return m.CreateRuleFn(ctx, workspaceID, sourceID, p)
	}
	return nil, nil //nolint:nilnil // mock
}

func (m *MockIngressService) UpdateRule(
	ctx context.Context,
	workspaceID, sourceID, ruleID string,
	p service.UpdateIngressRuleParams,
) (*service.IngressRuleDTO, error) {
	if m.UpdateRuleFn != nil {
		return m.UpdateRuleFn(ctx, workspaceID, sourceID, ruleID, p)
	}
	return nil, nil //nolint:nilnil // mock
}

func (m *MockIngressService) DeleteRule(ctx context.Context, workspaceID, sourceID, ruleID string) error {
	if m.DeleteRuleFn != nil {
		return m.DeleteRuleFn(ctx, workspaceID, sourceID, ruleID)
	}
	return nil
}

func (m *MockIngressService) ReorderRules(
	ctx context.Context,
	workspaceID, sourceID string,
	entries []service.ReorderRuleEntry,
) ([]*service.IngressRuleDTO, error) {
	if m.ReorderRulesFn != nil {
		return m.ReorderRulesFn(ctx, workspaceID, sourceID, entries)
	}
	return nil, nil
}

func (m *MockIngressService) ListLog(
	ctx context.Context,
	workspaceID, statusFilter string,
	limit, offset int64,
) ([]*service.IngressLogEntryDTO, error) {
	if m.ListLogFn != nil {
		return m.ListLogFn(ctx, workspaceID, statusFilter, limit, offset)
	}
	return nil, nil
}

func (m *MockIngressService) ListSourceLog(
	ctx context.Context,
	workspaceID, sourceID string,
	limit, offset int64,
) ([]*service.IngressLogEntryDTO, error) {
	if m.ListSourceLogFn != nil {
		return m.ListSourceLogFn(ctx, workspaceID, sourceID, limit, offset)
	}
	return nil, nil
}

func (m *MockIngressService) DeleteLogEntry(ctx context.Context, workspaceID, entryID string) error {
	if m.DeleteLogEntryFn != nil {
		return m.DeleteLogEntryFn(ctx, workspaceID, entryID)
	}
	return nil
}

func (m *MockIngressService) RetryLogEntry(ctx context.Context, workspaceID, entryID string) (string, error) {
	if m.RetryLogEntryFn != nil {
		return m.RetryLogEntryFn(ctx, workspaceID, entryID)
	}
	return "", nil
}
