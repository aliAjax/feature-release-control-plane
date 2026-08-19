package notify

import (
	"context"
	"fmt"
	"sync"
)

// DeliveryEvent pairs a subscription with the envelope that must be delivered
// to it. Producers enqueue events and workers drain them so a slow subscriber
// cannot block the release control plane's hot path.
type DeliveryEvent struct {
	Subscription Subscription
	Envelope     Envelope
}

// DeliveryResult records the outcome of one delivery attempt. It is buffered
// independently from the queue so result collection never backpressures the
// workers while they are still draining.
type DeliveryResult struct {
	SubscriptionID string
	EventID        string
	Err            error
}

// Dispatcher fans a single event stream out to subscriber endpoints through a
// fixed worker pool. All workers share the same queue and the same cancellation
// context; a worker exits either when ctx is done or when the queue is closed
// and drained.
type Dispatcher struct {
	client  *WebhookClient
	retry   RetryPolicy
	workers int

	queue   chan DeliveryEvent
	results chan DeliveryResult

	wg    sync.WaitGroup
	mu    sync.Mutex
	close sync.Once
	done  bool
}

func NewDispatcher(client *WebhookClient, retry RetryPolicy, workers int) *Dispatcher {
	if workers < 1 {
		workers = 1
	}
	return &Dispatcher{
		client:  client,
		retry:   retry,
		workers: workers,
		queue:   make(chan DeliveryEvent, 64),
		results: make(chan DeliveryResult, 64),
	}
}

// Start spawns the configured number of workers. The WaitGroup counter is
// incremented before any goroutine is launched so a concurrent Wait can never
// observe a zero counter and return early.
func (d *Dispatcher) Start(ctx context.Context) {
	d.wg.Add(d.workers)
	for i := 0; i < d.workers; i++ {
		go d.worker(ctx)
	}
}

func (d *Dispatcher) worker(ctx context.Context) {
	defer d.wg.Done()
	for {
		select {
		case <-ctx.Done():
			return
		case evt, ok := <-d.queue:
			if !ok {
				return
			}
			err := d.retry.Do(ctx, func(c context.Context) error {
				return d.client.Deliver(c, evt.Subscription, evt.Envelope)
			})
			d.results <- DeliveryResult{SubscriptionID: evt.Subscription.ID, EventID: evt.Envelope.EventID, Err: err}
		}
	}
}

// Enqueue appends an event to the shared queue. Enqueue fails after Close so
// producers cannot race with the shutdown sequence and write to a closed
// channel.
func (d *Dispatcher) Enqueue(evt DeliveryEvent) error {
	d.mu.Lock()
	if d.done {
		d.mu.Unlock()
		return fmt.Errorf("dispatcher is closed")
	}
	d.mu.Unlock()
	d.queue <- evt
	return nil
}

// Close marks the queue as closed. Workers already draining will finish the
// buffered events and exit; no new events are accepted afterwards.
func (d *Dispatcher) Close() {
	d.close.Do(func() {
		d.mu.Lock()
		d.done = true
		d.mu.Unlock()
		close(d.queue)
	})
}

// Wait blocks until all workers have exited. Callers should invoke Close first
// unless they rely on ctx cancellation to stop the workers.
func (d *Dispatcher) Wait() {
	d.wg.Wait()
}

// Results returns the result channel so callers can count delivered and failed
// events without blocking worker progress.
func (d *Dispatcher) Results() <-chan DeliveryResult {
	return d.results
}
