package importexport

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/example/feature-release-control-plane/internal/configdomain"
	"github.com/example/feature-release-control-plane/internal/platform/httpx"
)

// ConfigDraft is the import representation of a configuration before it is
// assigned a server-side id. Import accepts many drafts at once and validates
// them as a unit so a partially invalid document cannot be half-applied.
type ConfigDraft struct {
	Key         string                    `json:"key"`
	Description string                    `json:"description"`
	Value       configdomain.ConfigValue  `json:"value"`
	Rules       []configdomain.TargetRule `json:"rules,omitempty"`
}

type Document struct {
	Scope   configdomain.Scope `json:"scope"`
	Configs []ConfigDraft      `json:"configs"`
}

// Parse decodes a strict JSON document. Unknown fields and trailing values are
// rejected so operators get a clear error instead of silently dropped input.
func Parse(data []byte) (Document, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	var doc Document
	if err := dec.Decode(&doc); err != nil {
		return Document{}, fmt.Errorf("%w: %v", httpx.ErrBadRequest, err)
	}
	if dec.More() {
		return Document{}, fmt.Errorf("%w: document must contain one JSON value", httpx.ErrBadRequest)
	}
	return doc, nil
}

// Validate checks the document as a whole: scope names, config keys, value
// kinds and rule windows. Duplicate keys within one document are rejected
// because import is idempotent only when the input itself is unambiguous.
func (d Document) Validate() error {
	if err := d.Scope.Validate(); err != nil {
		return err
	}
	if len(d.Configs) == 0 {
		return fmt.Errorf("%w: document must contain at least one config", httpx.ErrBadRequest)
	}
	seen := make(map[string]bool, len(d.Configs))
	for _, c := range d.Configs {
		if err := c.Validate(); err != nil {
			return err
		}
		if seen[c.Key] {
			return fmt.Errorf("%w: duplicate config key %s", httpx.ErrBadRequest, c.Key)
		}
		seen[c.Key] = true
	}
	return nil
}

func (c ConfigDraft) Validate() error {
	if !validKey(c.Key) {
		return fmt.Errorf("%w: invalid config key %q", httpx.ErrBadRequest, c.Key)
	}
	if err := c.Value.Validate(); err != nil {
		return err
	}
	for _, r := range c.Rules {
		if err := r.Validate(); err != nil {
			return err
		}
	}
	return nil
}

func validKey(s string) bool {
	if len(s) < 2 || len(s) > 64 {
		return false
	}
	for i := 0; i < len(s); i++ {
		b := s[i]
		if (b >= 'a' && b <= 'z') || (b >= '0' && b <= '9') || b == '_' || b == '-' {
			continue
		}
		return false
	}
	return true
}

// SortedConfigs returns drafts ordered by key. Callers use it to guarantee a
// deterministic import order for audit and diff purposes.
func (d Document) SortedConfigs() []ConfigDraft {
	sort.Slice(d.Configs, func(i, j int) bool { return d.Configs[i].Key < d.Configs[j].Key })
	return d.Configs
}
