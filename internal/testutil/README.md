# Test Utilities

This package provides mock implementations and test helpers for the Astrolabe CLI/TUI project.

## MockSerial

`MockSerial` is a configurable mock serial device for testing serial port interactions without requiring actual hardware.

### Features

- **Pre-configured data streams**: Emit a sequence of byte frames
- **Configurable delays**: Simulate real-world timing between frames
- **Error injection**: Test error handling by configuring read errors
- **Context cancellation**: Properly handles context cancellation
- **Reusable**: Reset and reuse for multiple test cases
- **Dynamic frames**: Add frames during test execution

### Basic Usage

```go
import "github.com/cthonicasoftware/astrolabe-cli/internal/testutil"

// Create a mock with test data
mock := testutil.NewMockSerial(
    testutil.WithFrames([][]byte{
        []byte("Frame 1\n"),
        []byte("Frame 2\n"),
    }),
    testutil.WithFrameDelay(10*time.Millisecond),
)

ctx := context.Background()
mock.Open(ctx)
defer mock.Close()

// Read frames
for frame := range mock.Frames() {
    // Process frame
}
```

### Configuration Options

#### WithFrames
Set the sequence of byte frames to emit:
```go
frames := [][]byte{
    []byte("TEMP:23.5C\n"),
    []byte("TEMP:23.7C\n"),
}
mock := testutil.NewMockSerial(testutil.WithFrames(frames))
```

#### WithFrameDelay
Control the timing between frame emissions:
```go
// Emit frames every 100ms
mock := testutil.NewMockSerial(
    testutil.WithFrameDelay(100*time.Millisecond),
)
```

#### WithConfig
Configure serial port parameters:
```go
cfg := sources.Config{
    Port:        "/dev/ttyUSB0",
    Baud:        9600,
    Parity:      "E",
    DataBits:    7,
    StopBits:    "2",
    FlowControl: "hardware",
}
mock := testutil.NewMockSerial(testutil.WithConfig(cfg))
```

#### WithReadError
Simulate read errors after frames are exhausted:
```go
mock := testutil.NewMockSerial(
    testutil.WithFrames(frames),
    testutil.WithReadError(errors.New("connection lost")),
)
```

### Testing Patterns

#### Simulating a Real Device
```go
// Simulate a sensor emitting readings
sensorData := [][]byte{
    []byte("TEMP:23.5C\n"),
    []byte("TEMP:23.7C\n"),
    []byte("TEMP:23.6C\n"),
}

mock := testutil.NewMockSerial(
    testutil.WithFrames(sensorData),
    testutil.WithFrameDelay(100*time.Millisecond),
)

ctx := context.Background()
mock.Open(ctx)
defer mock.Close()

for frame := range mock.Frames() {
    // Process sensor reading
}
```

#### Testing Context Cancellation
```go
mock := testutil.NewMockSerial(
    testutil.WithFrames(manyFrames),
    testutil.WithFrameDelay(10*time.Millisecond),
)

ctx, cancel := context.WithCancel(context.Background())
mock.Open(ctx)
defer mock.Close()

// Read some frames
for i := 0; i < 5; i++ {
    <-mock.Frames()
}

// Cancel and verify cleanup
cancel()
_, ok := <-mock.Frames()
// ok should be false (channel closed)
```

#### Testing Error Handling
```go
expectedError := errors.New("hardware failure")
mock := testutil.NewMockSerial(
    testutil.WithFrames([][]byte{[]byte("Last frame\n")}),
    testutil.WithReadError(expectedError),
)

mock.Open(context.Background())
defer mock.Close()

// Read until error
for frame := range mock.Frames() {
    // Process frame
}
// Channel closes after error
```

#### Reusing Mocks
```go
mock := testutil.NewMockSerial(testutil.WithFrames(frames))

// First test run
mock.Open(ctx)
// ... read frames ...
mock.Close()

// Reset and reuse
mock.Reset()

// Second test run
mock.Open(ctx)
// ... read frames again ...
mock.Close()
```

### API Reference

#### Constructor
```go
func NewMockSerial(opts ...MockSerialOption) *MockSerial
```

#### Methods
- `Open(ctx context.Context) error` - Open the mock serial port
- `Close() error` - Close the mock serial port
- `Frames() <-chan []byte` - Get the frame output channel
- `Meta() core.SourceMeta` - Get source metadata
- `Reset()` - Reset the mock for reuse
- `AddFrame(frame []byte)` - Dynamically add a frame
- `FramesRemaining() int` - Get count of unread frames

### Interface Compatibility

`MockSerial` implements the same interface as `sources.Serial`, making it a drop-in replacement for testing:

```go
type SerialDevice interface {
    Open(ctx context.Context) error
    Close() error
    Frames() <-chan []byte
    Meta() core.SourceMeta
}
```

### Examples

See `examples_test.go` for complete working examples demonstrating various usage patterns.

### Running Tests

```bash
# Run all testutil tests
go test ./internal/testutil/...

# Run with verbose output
go test -v ./internal/testutil/...

# Run specific test
go test -v ./internal/testutil/ -run TestMockSerial_BasicOperation
```

## Future Test Utilities

Planned additions to this package:

- `MockTCPSource` - Mock TCP socket source
- `MockFileSource` - Mock file source with configurable content
- `NewTempRunDir()` - Create temporary run directories for testing
- `MockHTTPServer()` - Simulate QA backend API
- `AssertManifest()` - Validate manifest structure

## Contributing

When adding new test utilities:

1. Follow the existing patterns (builder options, context support)
2. Provide comprehensive tests
3. Include usage examples
4. Update this README with documentation
5. Ensure interface compatibility with real implementations

