package storage

import (
    "github.com/LostinTimeandspaceYT/qa_cli_agent/internal/core"
)

type Store interface {
    Start(run *core.Run) error
    Write([]core.Record) error
    Flush() error
    Artifacts() []string
}

type FS struct {
    artifacts []string
}

func NewFS(path string) *FS {
    return &FS{artifacts: []string{}}
}

func (f *FS) Start(run *core.Run) error { return nil }
func (f *FS) Write(_ []core.Record) error { return nil }
func (f *FS) Flush() error { return nil }
func (f *FS) Artifacts() []string { return f.artifacts }
