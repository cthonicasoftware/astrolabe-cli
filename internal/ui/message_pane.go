package ui

import (
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	"github.com/charmbracelet/lipgloss"
)

type messagePane struct {
	viewport viewport.Model
	lines    []string
	maxLines int
}

func newMessagePane(width, height, maxLines int) *messagePane {
	vp := viewport.New(width, height)
	return &messagePane{viewport: vp, maxLines: maxLines}
}

func (p *messagePane) SetWidth(width int) {
	if width < 0 {
		width = 0
	}
	if p.viewport.Width == width {
		return
	}
	p.viewport.Width = width
	p.sync()
}

func (p *messagePane) SetHeight(height int) {
	if height < 0 {
		height = 0
	}
	p.viewport.Height = height
}

func (p *messagePane) SetSize(width, height int) {
	p.SetWidth(width)
	p.SetHeight(height)
}

func (p *messagePane) Append(line string) {
	p.lines = append(p.lines, line)
	p.enforceLimit()
	p.sync()
}

func (p *messagePane) SetLines(lines []string) {
	p.lines = append([]string(nil), lines...)
	p.enforceLimit()
	p.sync()
}

func (p *messagePane) View() string {
	return p.viewport.View()
}

func (p *messagePane) Sync() {
	p.sync()
}

func (p *messagePane) Width() int {
	return p.viewport.Width
}

func (p *messagePane) Height() int {
	return p.viewport.Height
}

func (p *messagePane) enforceLimit() {
	if p.maxLines <= 0 {
		return
	}
	if len(p.lines) > p.maxLines {
		p.lines = p.lines[len(p.lines)-p.maxLines:]
	}
}

func (p *messagePane) sync() {
	content := strings.Join(p.lines, "\n")
	if p.viewport.Width > 0 {
		content = lipgloss.NewStyle().Width(p.viewport.Width).Render(content)
	}
	p.viewport.SetContent(content)
	p.viewport.GotoBottom()
}
