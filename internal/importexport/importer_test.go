package importexport

import "testing"

func TestSortedConfigsDoesNotMutateOriginal(t *testing.T) {
	d := Document{Configs: []ConfigDraft{{Key: "beta"}, {Key: "alpha"}}}
	firstBefore := d.Configs[0].Key
	sorted := d.SortedConfigs()
	if d.Configs[0].Key != firstBefore {
		t.Fatalf("SortedConfigs mutated the original slice: %q -> %q", firstBefore, d.Configs[0].Key)
	}
	if len(sorted) != 2 || sorted[0].Key != "alpha" || sorted[1].Key != "beta" {
		t.Fatalf("unexpected sorted order: %+v", sorted)
	}
}
