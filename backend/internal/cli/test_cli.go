package cli

import "testing"

func TestRegionsList_NotEmpty(t *testing.T) {
	regions := RegionsList()
	if len(regions) == 0 {
		t.Fatal("RegionsList returned empty slice; expected at least one region")
	}
}
