package root

import (
	"github.com/cthonicasoftware/astrolabe-cli/internal/config"
	"github.com/cthonicasoftware/astrolabe-cli/internal/sources"
	"github.com/cthonicasoftware/astrolabe-cli/internal/tui"
)

// liveViewPrefsAdapter implements tui.LiveViewPrefsPort on top of the
// live_view section of config's settings.json.
type liveViewPrefsAdapter struct{}

func (liveViewPrefsAdapter) Load() (tui.LiveViewPrefs, error) {
	s, err := config.LoadSettings()
	return tui.LiveViewPrefs{
		LineEnding: sources.ParseLineEnding(s.LiveView.LineEnding),
		Echo:       s.LiveView.Echo,
		Timestamps: s.LiveView.Timestamps,
		// Files saved before reconnect existed have no key: default on.
		Reconnect: s.LiveView.Reconnect == nil || *s.LiveView.Reconnect,
		Hex:       s.LiveView.Hex,
	}, err
}

func (liveViewPrefsAdapter) Save(p tui.LiveViewPrefs) error {
	reconnect := p.Reconnect
	return config.UpdateSettings(func(s *config.Settings) {
		s.LiveView = config.LiveViewPrefs{
			LineEnding: string(p.LineEnding),
			Echo:       p.Echo,
			Timestamps: p.Timestamps,
			Reconnect:  &reconnect,
			Hex:        p.Hex,
		}
	})
}
