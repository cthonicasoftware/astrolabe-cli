package root

import (
	"github.com/cthonicasoftware/astrolabe-cli/internal/config"
	"github.com/cthonicasoftware/astrolabe-cli/internal/sources"
	"github.com/cthonicasoftware/astrolabe-cli/internal/tui"
)

// sendPrefsAdapter implements tui.SendPrefsPort on top of config's
// send_prefs.json store.
type sendPrefsAdapter struct{}

func (sendPrefsAdapter) Load() (tui.SendPrefs, error) {
	p, err := config.LoadSendPrefs()
	return tui.SendPrefs{
		LineEnding: sources.ParseLineEnding(p.LineEnding),
		Echo:       p.Echo,
	}, err
}

func (sendPrefsAdapter) Save(p tui.SendPrefs) error {
	return config.SaveSendPrefs(config.SendPrefs{
		LineEnding: string(p.LineEnding),
		Echo:       p.Echo,
	})
}
