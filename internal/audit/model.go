package audit

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"time"
)

// Query describes the dimensions supported by the audit read path. Empty
// fields are wildcards; time bounds are inclusive at the start and exclusive
// at the end so callers can safely walk adjacent windows.
type Query struct {
	Scope    string
	Actor    string
	Action   string
	Resource string
	From     *time.Time
	To       *time.Time
}

func (q Query) Match(r Record) bool {
	if q.Scope != "" && r.Scope != q.Scope {
		return false
	}
	if q.Actor != "" && r.Actor != q.Actor {
		return false
	}
	if q.Action != "" && r.Action != q.Action {
		return false
	}
	if q.Resource != "" && r.Resource != q.Resource {
		return false
	}
	if q.From != nil && r.At.Before(*q.From) {
		return false
	}
	if q.To != nil && !r.At.Before(*q.To) {
		return false
	}
	return true
}

type Record struct {
	ID           string          `json:"id"`
	Scope        string          `json:"scope"`
	Actor        string          `json:"actor"`
	Action       string          `json:"action"`
	Resource     string          `json:"resource"`
	Metadata     json.RawMessage `json:"metadata"`
	At           time.Time       `json:"at"`
	PreviousHash string          `json:"previous_hash"`
	Hash         string          `json:"hash"`
}

func (r *Record) Seal(previous string) {
	r.PreviousHash = previous
	b, _ := json.Marshal(struct {
		Scope, Actor, Action, Resource, Previous string
		Metadata                                 json.RawMessage
		At                                       time.Time
	}{r.Scope, r.Actor, r.Action, r.Resource, previous, r.Metadata, r.At})
	sum := sha256.Sum256(b)
	r.Hash = hex.EncodeToString(sum[:])
}
func Verify(records []Record) bool {
	previous := ""
	for _, record := range records {
		candidate := record
		candidate.Seal(previous)
		if candidate.Hash != record.Hash {
			return false
		}
		previous = record.Hash
	}
	return true
}
