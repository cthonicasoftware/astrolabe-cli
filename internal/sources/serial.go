package sources

import (
    "context"
    "github.com/LostinTimeandspaceYT/qa_cli_agent/internal/core"
)

type Serial struct {
    Port string
    Baud int
    ch   chan []byte
}

func NewSerial(port string, baud int) *Serial {
    return &Serial{Port: port, Baud: baud, ch: make(chan []byte)}
}

func (s *Serial) Open(ctx context.Context) error {
    // TODO: implement real serial reading; for now just close channel.
    go func() {
        close(s.ch)
    }()
    return nil
}

func (s *Serial) Frames() <-chan []byte { return s.ch }

func (s *Serial) Meta() core.SourceMeta {
    return core.SourceMeta{Kind: "serial", Port: s.Port, Baud: s.Baud}
}

func (s *Serial) Close() error { return nil }
