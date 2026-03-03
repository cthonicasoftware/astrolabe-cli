package testutil_test

import (
	"fmt"

	"github.com/cthonicasoftware/astrolabe-cli/internal/sources"
	"github.com/cthonicasoftware/astrolabe-cli/internal/testutil"
)

// ExampleNewMockSerial demonstrates creating a mock serial device.
func ExampleNewMockSerial() {
	// Create a mock serial device with pre-configured data
	mock := testutil.NewMockSerial(
		testutil.WithFrames([][]byte{
			[]byte("Hello\n"),
			[]byte("World\n"),
		}),
	)

	fmt.Printf("Mock created with %d frames\n", mock.FramesRemaining())

	// Output:
	// Mock created with 2 frames
}

// ExampleWithConfig demonstrates configuring serial parameters.
func ExampleWithConfig() {
	// Configure mock with specific serial settings
	cfg := sources.Config{
		Port:        "/dev/ttyUSB0",
		Baud:        9600,
		Parity:      "E",
		DataBits:    7,
		StopBits:    "2",
		FlowControl: "hardware",
	}

	mock := testutil.NewMockSerial(testutil.WithConfig(cfg))

	meta := mock.Meta()
	fmt.Printf("Port: %s, Baud: %d\n", meta.Port, meta.Baud)

	// Output:
	// Port: /dev/ttyUSB0, Baud: 9600
}

// ExampleMockSerial_AddFrame demonstrates adding frames dynamically.
func ExampleMockSerial_AddFrame() {
	mock := testutil.NewMockSerial(
		testutil.WithFrames([][]byte{[]byte("Initial\n")}),
	)

	fmt.Printf("Initial frames: %d\n", mock.FramesRemaining())

	// Add more frames
	mock.AddFrame([]byte("Second\n"))
	mock.AddFrame([]byte("Third\n"))

	fmt.Printf("After adding: %d\n", mock.FramesRemaining())

	// Output:
	// Initial frames: 1
	// After adding: 3
}

// ExampleMockSerial_Meta demonstrates getting source metadata.
func ExampleMockSerial_Meta() {
	mock := testutil.NewMockSerial(
		testutil.WithConfig(sources.Config{
			Port: "/dev/ttyUSB0",
			Baud: 115200,
		}),
	)

	meta := mock.Meta()
	fmt.Printf("Kind: %s, Port: %s, Baud: %d\n", meta.Kind, meta.Port, meta.Baud)

	// Output:
	// Kind: serial, Port: /dev/ttyUSB0, Baud: 115200
}

