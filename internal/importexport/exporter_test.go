package importexport

import (
	"testing"

	"github.com/example/feature-release-control-plane/internal/configdomain"
)

func TestBuildExportIncludesAllVersions(t *testing.T) {
	versions := map[string][]configdomain.ConfigVersion{
		"checkout": {
			{Number: 1, State: configdomain.Published},
			{Number: 2, State: configdomain.Scheduled},
		},
	}
	exp := BuildExport(configdomain.Scope{}, versions)
	if len(exp.Configs) != 2 {
		t.Fatalf("expected both versions in export, got %d", len(exp.Configs))
	}
	if exp.Configs[0].Version != 1 || exp.Configs[1].Version != 2 {
		t.Fatalf("expected versions sorted ascending, got %+v", exp.Configs)
	}
}
