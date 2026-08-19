package repository

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/example/feature-release-control-plane/internal/audit"
	"github.com/example/feature-release-control-plane/internal/configdomain"
)

func TestMemoryConcurrentReadsAndWrites(t *testing.T) {
	m := NewMemory()
	scope := configdomain.Scope{TenantID: "tenant1", Application: "billing", Environment: "staging", Namespace: "flags"}
	_ = m.Create(context.Background(), configdomain.Config{ID: "cfg-1", Scope: scope, Key: "checkout", Version: 1})
	_ = m.PutVersion(context.Background(), configdomain.ConfigVersion{ConfigID: "cfg-1", Number: 1, State: configdomain.Published, Value: configdomain.ConfigValue{Kind: configdomain.Boolean, Raw: []byte("true")}})

	start := make(chan struct{})
	var wg sync.WaitGroup
	worker := func() {
		defer wg.Done()
		<-start
		for i := 0; i < 200; i++ {
			switch i % 4 {
			case 0:
				_, _ = m.Published(context.Background(), scope)
			case 1:
				_, _, _ = m.List(context.Background(), scope, 1, 10)
			case 2:
				_, _ = m.ListVersions(context.Background(), "cfg-1")
			default:
				_, _ = m.ListAuditAll(context.Background())
			}
		}
	}
	writer := func() {
		defer wg.Done()
		<-start
		for i := 0; i < 200; i++ {
			if i%2 == 0 {
				_ = m.PutVersion(context.Background(), configdomain.ConfigVersion{ConfigID: "cfg-1", Number: int64(i + 2), State: configdomain.Draft, Value: configdomain.ConfigValue{Kind: configdomain.Boolean, Raw: []byte("true")}})
			} else {
				_ = m.Append(context.Background(), audit.Record{ID: fmt.Sprintf("aud-%d", i), At: time.Now()})
			}
		}
	}
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go worker()
	}
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go writer()
	}
	close(start)
	wg.Wait()
}

func TestMemoryConcurrentAuditAppend(t *testing.T) {
	m := NewMemory()
	start := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		<-start
		for i := 0; i < 200; i++ {
			_, _ = m.ListAuditAll(context.Background())
		}
	}()
	go func() {
		defer wg.Done()
		<-start
		for i := 0; i < 200; i++ {
			_ = m.Append(context.Background(), audit.Record{ID: fmt.Sprintf("aud-%d", i), At: time.Now()})
		}
	}()
	close(start)
	wg.Wait()
}

func TestMemoryConcurrentVersionReads(t *testing.T) {
	m := NewMemory()
	scope := configdomain.Scope{TenantID: "tenant1", Application: "billing", Environment: "staging", Namespace: "flags"}
	_ = m.Create(context.Background(), configdomain.Config{ID: "cfg-1", Scope: scope, Key: "checkout", Version: 1})
	start := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		<-start
		for i := 0; i < 200; i++ {
			_, _ = m.ListVersions(context.Background(), "cfg-1")
		}
	}()
	go func() {
		defer wg.Done()
		<-start
		for i := 0; i < 200; i++ {
			_ = m.PutVersion(context.Background(), configdomain.ConfigVersion{ConfigID: "cfg-1", Number: int64(i + 1), State: configdomain.Draft, Value: configdomain.ConfigValue{Kind: configdomain.Boolean, Raw: []byte("true")}})
		}
	}()
	close(start)
	wg.Wait()
}

func TestPublishedDoesNotAliasInternalRules(t *testing.T) {
	m := NewMemory()
	scope := configdomain.Scope{TenantID: "tenant1", Application: "billing", Environment: "staging", Namespace: "flags"}
	_ = m.Create(context.Background(), configdomain.Config{ID: "cfg-1", Scope: scope, Key: "checkout", Version: 1})
	_ = m.PutVersion(context.Background(), configdomain.ConfigVersion{
		ConfigID: "cfg-1", Number: 1, State: configdomain.Published,
		Value: configdomain.ConfigValue{Kind: configdomain.Boolean, Raw: []byte("true")},
		Rules: []configdomain.TargetRule{{ID: "r1", Priority: 10, Percentage: 10, HashAttribute: "device_id", Enabled: true}},
	})
	got, _ := m.Published(context.Background(), scope)
	if len(got) != 1 || len(got[0].Rules) != 1 {
		t.Fatalf("expected one published version with one rule, got %+v", got)
	}
	got[0].Rules[0].Enabled = false
	again, _ := m.Published(context.Background(), scope)
	if again[0].Rules[0].Enabled != true {
		t.Fatalf("published version was mutated through the returned slice")
	}
}

func TestListVersionsDoesNotAliasInternalRules(t *testing.T) {
	m := NewMemory()
	scope := configdomain.Scope{TenantID: "tenant1", Application: "billing", Environment: "staging", Namespace: "flags"}
	_ = m.Create(context.Background(), configdomain.Config{ID: "cfg-1", Scope: scope, Key: "checkout", Version: 1})
	_ = m.PutVersion(context.Background(), configdomain.ConfigVersion{
		ConfigID: "cfg-1", Number: 1, State: configdomain.Published,
		Value: configdomain.ConfigValue{Kind: configdomain.Boolean, Raw: []byte("true")},
		Rules: []configdomain.TargetRule{{ID: "r1", Priority: 10, Percentage: 10, HashAttribute: "device_id", Enabled: true}},
	})
	got, _ := m.ListVersions(context.Background(), "cfg-1")
	if len(got) != 1 || len(got[0].Rules) != 1 {
		t.Fatalf("expected one version with one rule, got %+v", got)
	}
	got[0].Rules[0].Enabled = false
	again, _ := m.ListVersions(context.Background(), "cfg-1")
	if again[0].Rules[0].Enabled != true {
		t.Fatalf("version was mutated through the returned slice")
	}
}

func TestListAuditAllReturnsCopy(t *testing.T) {
	m := NewMemory()
	_ = m.Append(context.Background(), audit.Record{ID: "a", Metadata: []byte(`{"x":1}`), At: time.Now()})
	all, _ := m.ListAuditAll(context.Background())
	if len(all) != 1 {
		t.Fatalf("expected one audit record, got %d", len(all))
	}
	all[0].Metadata[0] = 'X'
	again, _ := m.ListAuditAll(context.Background())
	if again[0].Metadata[0] == 'X' {
		t.Fatalf("audit record was mutated through the returned slice")
	}
}
