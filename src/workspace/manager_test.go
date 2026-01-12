package workspace_test

import (
	"context"
	"testing"

	"github.com/AzielCF/az-wap/workspace"
	"github.com/AzielCF/az-wap/workspace/domain"
)

// Mock Adapter
type mockAdapter struct {
	id        string
	onMessage func(workspace.IncomingMessage)
}

func (m *mockAdapter) ID() string                                                         { return m.id }
func (m *mockAdapter) Type() domain.ChannelType                                           { return "mock" }
func (m *mockAdapter) Status() domain.ChannelStatus                                       { return domain.ChannelStatusConnected }
func (m *mockAdapter) Start(ctx context.Context, config domain.ChannelConfig) error       { return nil }
func (m *mockAdapter) Stop(ctx context.Context) error                                     { return nil }
func (m *mockAdapter) SendMessage(ctx context.Context, chatID, text string) error         { return nil }
func (m *mockAdapter) SendPresence(ctx context.Context, chatID string, typing bool) error { return nil }
func (m *mockAdapter) OnMessage(handler func(workspace.IncomingMessage)) {
	m.onMessage = handler
}

func TestManager_RegisterAdapter(t *testing.T) {
	mgr := workspace.NewManager(nil, nil) // Nil repo and engine

	adapter := &mockAdapter{id: "ch1"}
	mgr.RegisterAdapter(adapter)

	if _, ok := mgr.GetAdapter("ch1"); !ok {
		t.Error("adapter not registered")
	}

	mgr.UnregisterAdapter("ch1")
	if _, ok := mgr.GetAdapter("ch1"); ok {
		t.Error("adapter not unregistered")
	}
}
