package notify

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestDeliverReturnsErrorOnServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := NewWebhookClient(time.Second)
	err := c.Deliver(context.Background(), Subscription{ID: "s", URL: srv.URL}, Envelope{EventID: "e"})
	if err == nil {
		t.Fatal("expected an error for a 5xx response")
	}
}

func TestDeliverAcceptsNoContent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	c := NewWebhookClient(time.Second)
	err := c.Deliver(context.Background(), Subscription{ID: "s", URL: srv.URL}, Envelope{EventID: "e"})
	if err != nil {
		t.Fatalf("204 No Content should be accepted, got %v", err)
	}
}
