package ui

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
)

type portInputSection struct {
	label       string
	portInput   *textinput.Model
	portFocused bool
	showBaud    bool
	baudInput   *textinput.Model
	baudFocused bool
}

func newPortInputSection(label string, portInput *textinput.Model, portFocused bool, showBaud bool, baudInput *textinput.Model, baudFocused bool) portInputSection {
	return portInputSection{
		label:       label,
		portInput:   portInput,
		portFocused: portFocused,
		showBaud:    showBaud,
		baudInput:   baudInput,
		baudFocused: baudFocused,
	}
}

func (s portInputSection) View() string {
	var parts []string

	parts = append(parts, labelStyle.Render(s.label))
	applyInputFocusStyles(s.portInput, s.portFocused)
	parts = append(parts, s.portInput.View())

	if s.showBaud {
		parts = append(parts, "")
		parts = append(parts, labelStyle.Render("Baud Rate"))
		applyInputFocusStyles(s.baudInput, s.baudFocused)
		parts = append(parts, s.baudInput.View())
	}

	return strings.Join(parts, "\n")
}
