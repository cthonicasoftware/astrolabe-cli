package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

type stubModel struct{}

func (stubModel) Init() tea.Cmd                             { return nil }
func (m stubModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) { return m, nil }
func (stubModel) View() string                              { return "" }

func TestRouterCtrlCTriggersCleanup(t *testing.T) {
	cleaned := false
	r := newRouter(RouterConfig{
		InitialScreen: ScreenWelcome,
		Factories: map[ScreenID]ScreenFactory{
			ScreenWelcome: func(ctx ScreenContext) (tea.Model, func(), error) {
				return stubModel{}, func() { cleaned = true }, nil
			},
		},
	})

	model, cmd := r.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if model != r {
		t.Fatalf("Update() returned unexpected model")
	}
	if cmd == nil {
		t.Fatalf("Update() cmd = nil, want quit command")
	}
	if !cleaned {
		t.Fatalf("cleanup was not called before quit")
	}
}
