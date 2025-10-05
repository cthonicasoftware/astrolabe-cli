package upload

import "context"

type Client struct{}

func NewClient(_ any) *Client { return &Client{} }

func (c *Client) Enqueue(runID string, paths []string) error {
	_ = runID
	_ = paths
	return nil
}

func (c *Client) Run(ctx context.Context) error {
	<-ctx.Done()
	return nil
}
