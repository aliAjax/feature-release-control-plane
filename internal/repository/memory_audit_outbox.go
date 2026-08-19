package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/example/feature-release-control-plane/internal/audit"
	"github.com/example/feature-release-control-plane/internal/platform/httpx"
)

type AuditRepository interface {
	Append(context.Context, audit.Record) error
	ListAudit(context.Context, string, int, int) ([]audit.Record, int, error)
	ListAuditFiltered(context.Context, audit.Query, int, int) ([]audit.Record, int, error)
	ListAuditAll(context.Context) ([]audit.Record, error)
}
type OutboxEvent struct {
	ID          string
	Topic       string
	Key         string
	Payload     []byte
	CreatedAt   time.Time
	DeliveredAt *time.Time
	Attempts    int
}
type Outbox interface {
	Add(context.Context, OutboxEvent) error
	Pending(context.Context, int) ([]OutboxEvent, error)
	MarkDelivered(context.Context, string) error
}

func (m *Memory) Append(_ context.Context, r audit.Record) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	previous := ""
	if len(m.audits) > 0 {
		previous = m.audits[len(m.audits)-1].Hash
	}
	r.Seal(previous)
	m.audits = append(m.audits, r)
	return nil
}
func (m *Memory) ListAudit(_ context.Context, scope string, page, size int) ([]audit.Record, int, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	all := []audit.Record{}
	for _, r := range m.audits {
		if scope == "" || r.Scope == scope {
			all = append(all, r)
		}
	}
	items, total, err := pageOf(all, page, size)
	return items, total, err
}

func (m *Memory) ListAuditFiltered(_ context.Context, query audit.Query, page, size int) ([]audit.Record, int, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	all := make([]audit.Record, 0, len(m.audits))
	for _, record := range m.audits {
		if query.Match(record) {
			all = append(all, record)
		}
	}
	// Audit records are appended in chronological order. Preserve that order
	// because it is also the order used by hash-chain verification.
	return pageOf(all, page, size)
}

func (m *Memory) ListAuditAll(_ context.Context) ([]audit.Record, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	all := make([]audit.Record, len(m.audits))
	for i, r := range m.audits {
		r.Metadata = append(json.RawMessage(nil), r.Metadata...)
		all[i] = r
	}
	return all, nil
}
func (m *Memory) Add(_ context.Context, e OutboxEvent) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.outbox[e.ID]; ok {
		return nil
	}
	m.outbox[e.ID] = e
	return nil
}
func (m *Memory) Pending(_ context.Context, limit int) ([]OutboxEvent, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	all := []OutboxEvent{}
	for _, e := range m.outbox {
		if e.DeliveredAt == nil {
			all = append(all, e)
		}
	}
	sort.Slice(all, func(i, j int) bool { return all[i].CreatedAt.Before(all[j].CreatedAt) })
	if len(all) > limit {
		all = all[:limit]
	}
	return all, nil
}
func (m *Memory) MarkDelivered(_ context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	e, ok := m.outbox[id]
	if !ok {
		return fmt.Errorf("%w: outbox event", httpx.ErrNotFound)
	}
	now := time.Now().UTC()
	e.DeliveredAt = &now
	e.Attempts++
	m.outbox[id] = e
	return nil
}
