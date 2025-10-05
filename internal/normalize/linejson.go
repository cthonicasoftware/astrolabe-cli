package normalize

import (
    "strings"
    "time"

    "github.com/LostinTimeandspaceYT/qa_cli_agent/internal/core"
)

type Normalizer interface {
    Init(run *core.Run) error
    Ingest(raw []byte) ([]core.Record, error)
    Close() error
}

type LineJSON struct{}

func NewLineJSON() *LineJSON { return &LineJSON{} }

func (l *LineJSON) Init(*core.Run) error { return nil }

func (l *LineJSON) Ingest(b []byte) ([]core.Record, error) {
    s := strings.TrimSpace(string(b))
    rec := core.Record{
        TS:   time.Now().UTC(),
        Seq:  0,
        Type: "sample",
        Payload: map[string]any{"line": s},
    }
    return []core.Record{rec}, nil
}

func (l *LineJSON) Close() error { return nil }
