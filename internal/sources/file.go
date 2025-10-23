package sources

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"

	"github.com/LostinTimeandspaceYT/qa_cli_agent/internal/core"
)

// FileConfig captures file ingestion parameters.
type FileConfig struct {
	Path      string
	ChunkSize int  // bytes per chunk (0 = line-by-line)
	SkipLines int  // skip N header lines
	Follow    bool // tail -f mode (future enhancement)
}

// DefaultFileConfig returns standard file ingestion defaults.
func DefaultFileConfig() FileConfig {
	return FileConfig{
		ChunkSize: 0, // line-by-line by default
		SkipLines: 0,
		Follow:    false,
	}
}

// File represents a file-based data source for ingesting existing data files.
type File struct {
	FileConfig

	mu     sync.Mutex
	file   *os.File
	ch     chan []byte
	cancel context.CancelFunc
}

// NewFileWithConfig constructs a File source from an explicit configuration.
func NewFileWithConfig(cfg FileConfig) (*File, error) {
	defaults := DefaultFileConfig()

	if cfg.Path == "" {
		return nil, fmt.Errorf("file path is required")
	}

	if cfg.ChunkSize == 0 {
		cfg.ChunkSize = defaults.ChunkSize
	}

	// Expand path (handle ~, relative paths)
	absPath, err := filepath.Abs(cfg.Path)
	if err != nil {
		return nil, fmt.Errorf("invalid file path: %w", err)
	}

	// Verify file exists and is readable
	info, err := os.Stat(absPath)
	if err != nil {
		return nil, fmt.Errorf("cannot access file: %w", err)
	}

	if info.IsDir() {
		return nil, fmt.Errorf("path is a directory, not a file: %s", absPath)
	}

	cfg.Path = absPath

	return &File{
		FileConfig: cfg,
		ch:         make(chan []byte, 16),
	}, nil
}

// NewFile creates a file source with just a path (backward compatibility).
func NewFile(path string) (*File, error) {
	return NewFileWithConfig(FileConfig{Path: path})
}

// Open starts reading the file and streaming data through the channel.
func (f *File) Open(ctx context.Context) error {
	file, err := os.Open(f.Path)
	if err != nil {
		return fmt.Errorf("failed to open file %s: %w", f.Path, err)
	}

	f.mu.Lock()
	f.file = file
	f.mu.Unlock()

	// Create cancellable context for read loop
	readCtx, cancel := context.WithCancel(ctx)
	f.cancel = cancel

	// Start reading loop
	go f.readLoop(readCtx)

	return nil
}

// readLoop streams file content through the channel.
func (f *File) readLoop(ctx context.Context) {
	defer close(f.ch)

	f.mu.Lock()
	file := f.file
	f.mu.Unlock()

	if file == nil {
		return
	}

	reader := bufio.NewReader(file)

	// Skip header lines if configured
	for i := 0; i < f.SkipLines; i++ {
		_, err := reader.ReadBytes('\n')
		if err != nil {
			if err == io.EOF {
				return
			}
			return
		}
	}

	if f.ChunkSize > 0 {
		// Chunk mode: read fixed-size chunks
		f.readChunked(ctx, reader)
	} else {
		// Line mode: read line by line
		f.readLines(ctx, reader)
	}
}

// readLines reads file line by line and sends each line as a frame.
func (f *File) readLines(ctx context.Context, reader *bufio.Reader) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			line, err := reader.ReadBytes('\n')
			if err != nil {
				if err == io.EOF {
					// Send final line if it doesn't end with newline
					if len(line) > 0 {
						select {
						case f.ch <- line:
						case <-ctx.Done():
						}
					}
					return
				}
				// Other error - stop reading
				return
			}

			if len(line) > 0 {
				select {
				case f.ch <- line:
				case <-ctx.Done():
					return
				}
			}
		}
	}
}

// readChunked reads file in fixed-size chunks.
func (f *File) readChunked(ctx context.Context, reader *bufio.Reader) {
	buf := make([]byte, f.ChunkSize)

	for {
		select {
		case <-ctx.Done():
			return
		default:
			n, err := reader.Read(buf)
			if err != nil {
				if err == io.EOF {
					// Send final chunk if any data was read
					if n > 0 {
						chunk := make([]byte, n)
						copy(chunk, buf[:n])
						select {
						case f.ch <- chunk:
						case <-ctx.Done():
						}
					}
					return
				}
				// Other error - stop reading
				return
			}

			if n > 0 {
				chunk := make([]byte, n)
				copy(chunk, buf[:n])

				select {
				case f.ch <- chunk:
				case <-ctx.Done():
					return
				}
			}
		}
	}
}

// Frames returns the channel for receiving file data.
func (f *File) Frames() <-chan []byte {
	return f.ch
}

// Meta returns source metadata for the file.
func (f *File) Meta() core.SourceMeta {
	return core.SourceMeta{
		Kind: "file",
		Path: f.Path,
	}
}

// Close closes the file and stops the read loop.
func (f *File) Close() error {
	// Cancel the read loop
	if f.cancel != nil {
		f.cancel()
	}

	// Close the file
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.file != nil {
		return f.file.Close()
	}
	return nil
}
