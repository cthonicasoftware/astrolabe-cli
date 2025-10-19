package upload

import (
	"context"
	"errors"
	"sync"
)

// UploadClient defines the interface for uploading runs.
// This allows for easy mocking in tests.
type UploadClient interface {
	UploadRun(ctx context.Context, runID string) error
}

// Ensure Client implements UploadClient
var _ UploadClient = (*Client)(nil)

// MockUploadClient is a mock implementation of UploadClient for testing.
// It allows developers to simulate upload success, failure, and delays.
type MockUploadClient struct {
	mu sync.Mutex

	// UploadFunc is called when UploadRun is invoked
	// If nil, returns success by default
	UploadFunc func(ctx context.Context, runID string) error

	// Calls tracks all calls to UploadRun
	Calls []MockUploadCall

	// Results maps runID to predetermined results
	// Useful for testing specific scenarios
	Results map[string]error
}

// MockUploadCall records a single call to UploadRun
type MockUploadCall struct {
	RunID string
	Error error
}

// NewMockUploadClient creates a new mock upload client.
func NewMockUploadClient() *MockUploadClient {
	return &MockUploadClient{
		Calls:   make([]MockUploadCall, 0),
		Results: make(map[string]error),
	}
}

// UploadRun implements UploadClient.
func (m *MockUploadClient) UploadRun(ctx context.Context, runID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check for context cancellation
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	var err error

	// Use custom function if provided
	if m.UploadFunc != nil {
		err = m.UploadFunc(ctx, runID)
	} else if result, exists := m.Results[runID]; exists {
		// Use predetermined result
		err = result
	}
	// Otherwise, return success (nil)

	// Record the call
	m.Calls = append(m.Calls, MockUploadCall{
		RunID: runID,
		Error: err,
	})

	return err
}

// CallCount returns the number of times UploadRun was called.
func (m *MockUploadClient) CallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.Calls)
}

// CalledWith returns true if UploadRun was called with the given runID.
func (m *MockUploadClient) CalledWith(runID string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, call := range m.Calls {
		if call.RunID == runID {
			return true
		}
	}
	return false
}

// Reset clears all recorded calls.
func (m *MockUploadClient) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Calls = make([]MockUploadCall, 0)
}

// SetResult configures a predetermined result for a specific runID.
func (m *MockUploadClient) SetResult(runID string, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Results[runID] = err
}

// SetError is a convenience method to set an error result.
func (m *MockUploadClient) SetError(runID string, errMsg string) {
	m.SetResult(runID, errors.New(errMsg))
}

// SetSuccess is a convenience method to set a success result.
func (m *MockUploadClient) SetSuccess(runID string) {
	m.SetResult(runID, nil)
}
