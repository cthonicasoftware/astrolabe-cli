package root

import (
	"github.com/cthonicasoftware/astrolabe-cli/internal/config"
	"github.com/cthonicasoftware/astrolabe-cli/internal/tui"
)

type metadataAdapter struct {
	repo config.MetadataRepository
}

func newMetadataAdapter(repo config.MetadataRepository) *metadataAdapter {
	return &metadataAdapter{repo: repo}
}

func (a *metadataAdapter) Load() (tui.MetadataValues, error) {
	meta, err := a.repo.Load()
	return tui.MetadataValues{
		Operator:   meta.Operator,
		Location:   meta.Location,
		Device:     meta.Device,
		Test:       meta.Test,
		Tags:       meta.Tags,
		Attributes: meta.Attributes,
	}, err
}

func (a *metadataAdapter) Save(values tui.MetadataValues) (string, error) {
	return a.repo.Save(config.Metadata{
		Operator:   values.Operator,
		Location:   values.Location,
		Device:     values.Device,
		Test:       values.Test,
		Tags:       values.Tags,
		Attributes: values.Attributes,
	})
}

func (a *metadataAdapter) Path() (string, error) {
	return a.repo.Path()
}
