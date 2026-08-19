package notify

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func TestDispatcherDrainsAllEvents(t *testing.T) {
	var mu sync.Mutex
	var paths []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		paths = append(paths, r.URL.Path)
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := NewWebhookClient(time.Second)
	d := NewDispatcher(client, RetryPolicy{MaxAttempts: 1, InitialBackoff: time.Millisecond}, 4)
	d.Start(context.Background())
	for i := 0; i < 8; i++ {
		evt := DeliveryEvent{
			Subscription: Subscription{ID: fmt.Sprintf("sub-%d", i), URL: srv.URL},
			Envelope:     Envelope{EventID: fmt.Sprintf("evt-%d", i)},
		}
		if err := d.Enqueue(evt); err != nil {
			t.Fatalf("enqueue %d: %v", i, err)
		}
	}
	d.Close()
	d.Wait()
	mu.Lock()
	count := len(paths)
	mu.Unlock()
	if count != 8 {
		t.Fatalf("expected 8 deliveries, got %d", count)
	}
}

func TestEnqueueAfterCloseReturnsError(t *testing.T) {
	d := NewDispatcher(NewWebhookClient(time.Second), RetryPolicy{MaxAttempts: 1}, 2)
	d.Start(context.Background())
	d.Close()
	d.Wait()

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("Enqueue after Close must return an error, not panic: %v", r)
		}
	}()
	err := d.Enqueue(DeliveryEvent{})
	if err == nil {
		t.Fatal("expected an error when enqueueing after Close")
	}
}

func TestDispatcherConcurrentEnqueueClose(t *testing.T) {
	d := NewDispatcher(NewWebhookClient(time.Second), RetryPolicy{MaxAttempts: 1}, 4)
	d.Start(context.Background())
	start := make(chan struct{})
	var wg sync.WaitGroup
	panicked := make(chan any, 1)
	wg.Add(2)
	go func() {
		defer wg.Done()
		<-start
		defer func() {
			if r := recover(); r != nil {
				select {
				case panicked <- r:
				default:
				}
			}
		}()
		for i := 0; i < 1000; i++ {
			_ = d.Enqueue(DeliveryEvent{Subscription: Subscription{ID: "s", URL: "http://127.0.0.1:1"}, Envelope: Envelope{EventID: "e"}})
		}
	}()
	go func() {
		defer wg.Done()
		<-start
		d.Close()
	}()
	close(start)
	wg.Wait()
	select {
	case r := <-panicked:
		t.Fatalf("concurrent Enqueue after Close panicked: %v", r)
	default:
	}
}
