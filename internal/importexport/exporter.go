package importexport

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/example/feature-release-control-plane/internal/configdomain"
)

// ExportedConfig is the public shape of an exported configuration version.
type ExportedConfig struct {
	Key          string                    `json:"key"`
	Version      int64                     `json:"version"`
	State        configdomain.VersionState `json:"state"`
	Value        configdomain.ConfigValue  `json:"value"`
	Rules        []configdomain.TargetRule `json:"rules,omitempty"`
	Dependencies []configdomain.Dependency `json:"dependencies,omitempty"`
}

type Export struct {
	Scope   configdomain.Scope `json:"scope"`
	Configs []ExportedConfig   `json:"configs"`
}

// BuildExport assembles an export document from a scope and its resolved
// versions. It sorts deterministically by key and then by version number so two
// exports of the same state are byte-for-byte identical.
func BuildExport(scope configdomain.Scope, versions map[string][]configdomain.ConfigVersion) Export {
	out := Export{Scope: scope}
	for key, list := range versions {
		for _, v := range list {
			out.Configs = append(out.Configs, ExportedConfig{
				Key: key, Version: v.Number, State: v.State, Value: v.Value,
				Rules: v.Rules, Dependencies: v.Dependencies,
			})
		}
	}
	sort.Slice(out.Configs, func(i, j int) bool {
		if out.Configs[i].Key != out.Configs[j].Key {
			return out.Configs[i].Key < out.Configs[j].Key
		}
		return out.Configs[i].Version < out.Configs[j].Version
	})
	return out
}

// Marshal renders the export with stable key ordering and indentation so it can
// be diffed and stored in version control.
func Marshal(export Export) ([]byte, error) {
	b, err := json.MarshalIndent(export, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal export: %w", err)
	}
	return b, nil
}
