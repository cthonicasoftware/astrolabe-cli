package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
)

type captureDetailsSection struct {
	runIDInput      *textinput.Model
	runIDFocused    bool
	firmwareInput   *textinput.Model
	firmwareFocused bool
	formatField     selectField
	formatFocused   bool
	uploadModeField selectField
	uploadFocused   bool
	autoUploadView  string
	err             error
}

func newCaptureDetailsSection(
	runIDInput *textinput.Model,
	runIDFocused bool,
	firmwareInput *textinput.Model,
	firmwareFocused bool,
	formatField selectField,
	formatFocused bool,
	uploadModeField selectField,
	uploadFocused bool,
	autoUploadView string,
	err error,
) captureDetailsSection {
	return captureDetailsSection{
		runIDInput:      runIDInput,
		runIDFocused:    runIDFocused,
		firmwareInput:   firmwareInput,
		firmwareFocused: firmwareFocused,
		formatField:     formatField,
		formatFocused:   formatFocused,
		uploadModeField: uploadModeField,
		uploadFocused:   uploadFocused,
		autoUploadView:  autoUploadView,
		err:             err,
	}
}

func (s captureDetailsSection) View() string {
	parts := []string{labelStyle.Render("Capture Details"), ""}

	applyInputFocusStyles(s.runIDInput, s.runIDFocused)
	parts = append(parts, labelStyle.Render("Run ID"))
	parts = append(parts, s.runIDInput.View())
	parts = append(parts, "")

	applyInputFocusStyles(s.firmwareInput, s.firmwareFocused)
	parts = append(parts, labelStyle.Render("Firmware Hash"))
	parts = append(parts, s.firmwareInput.View())
	parts = append(parts, "")

	parts = append(parts, s.formatField.view(s.formatFocused))
	parts = append(parts, s.uploadModeField.view(s.uploadFocused))
	parts = append(parts, s.autoUploadView)

	if s.err != nil {
		parts = append(parts, "")
		parts = append(parts, errorStyle.Render(fmt.Sprintf("Error: %v", s.err)))
	}

	return strings.Join(parts, "\n")
}
