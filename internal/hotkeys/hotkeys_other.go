//go:build !windows

package hotkeys

import "context"

type Manager struct{}

func Start(ctx context.Context) *Manager {
	return &Manager{}
}

func (m *Manager) Stop() {}
