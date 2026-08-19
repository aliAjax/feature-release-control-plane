package application

import (
	"testing"
	"time"

	"github.com/example/feature-release-control-plane/internal/audit"
)

func TestSortAuditByTimeKeepsInputOrder(t *testing.T) {
	now := time.Now()
	records := []audit.Record{
		{ID: "b", At: now.Add(time.Second)},
		{ID: "a", At: now},
	}
	firstBefore := records[0].ID
	_ = SortAuditByTime(records)
	if records[0].ID != firstBefore {
		t.Fatalf("SortAuditByTime mutated the caller's slice: %q -> %q", firstBefore, records[0].ID)
	}
}
