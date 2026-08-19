package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Subscription is a subscriber endpoint that receives release-change
// notifications. Every delivery carries the same signed envelope so receivers
// can verify provenance independently of the transport.
type Subscription struct {
	ID          string            `json:"id"`
	URL         string            `json:"url"`
	Topics      []string          `json:"topics"`
	Headers     map[string]string `json:"headers,omitempty"`
	LastErrorAt *time.Time        `json:"last_error_at,omitempty"`
}

// Envelope is the wire payload delivered to a subscriber.
type Envelope struct {
	EventID   string          `json:"event_id"`
	Topic     string          `json:"topic"`
	Resource  string          `json:"resource"`
	Version   int64           `json:"version"`
	Occurred  time.Time       `json:"occurred_at"`
	Payload   json.RawMessage `json:"payload"`
	Signature string          `json:"signature"`
}

// WebhookClient performs a single HTTP delivery with a bounded timeout. It is
// kept separate from the retry policy so the two concerns stay independent
// concerns independently.
type WebhookClient struct {
	client  *http.Client
	baseURL string
}

func NewWebhookClient(timeout time.Duration) *WebhookClient {
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	return &WebhookClient{client: &http.Client{Timeout: timeout}}
}

// Deliver POSTs the envelope to a subscription URL. A non-2xx response is an
// error so callers can decide whether to retry or quarantine the subscription.
func (c *WebhookClient) Deliver(ctx context.Context, sub Subscription, env Envelope) error {
	body, err := json.Marshal(env)
	if err != nil {
		return fmt.Errorf("marshal envelope: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, sub.URL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range sub.Headers {
		req.Header.Set(k, v)
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("post webhook: %w", err)
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	return nil
}
