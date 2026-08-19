package application

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/example/feature-release-control-plane/internal/configdomain"
	"github.com/example/feature-release-control-plane/internal/releasedomain"
	"github.com/example/feature-release-control-plane/internal/repository"
)

func TestServiceConcurrentEmitAndSubscribe(t *testing.T) {
	store := repository.NewMemory()
	releases := releasedomain.NewMemoryRepository()
	svc := New(store, store, store, releases, "0123456789abcdef")

	start := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		<-start
		for i := 0; i < 300; i++ {
			ch, cancel := svc.Subscribe()
			<-ch
			cancel()
		}
	}()
	go func() {
		defer wg.Done()
		<-start
		for i := 0; i < 300; i++ {
			_, _ = svc.CreateConfig(context.Background(), "actor", CreateConfig{
				Scope: configdomain.Scope{TenantID: "tenant1", Application: "billing", Environment: "staging", Namespace: "flags"},
				Key:   fmt.Sprintf("checkout-%d", i),
			})
		}
	}()
	close(start)
	wg.Wait()
}
