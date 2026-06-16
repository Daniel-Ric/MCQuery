package cli

import "testing"

func TestParsePortListExpandsRanges(t *testing.T) {
	got, err := parsePortList("19132-19135")
	if err != nil {
		t.Fatalf("parsePortList returned error: %v", err)
	}
	want := []int{19132, 19133, 19134, 19135}
	assertPorts(t, got, want)
}

func TestParsePortListMixesPortsAndRanges(t *testing.T) {
	got, err := parsePortList("19132,19134-19136,19134")
	if err != nil {
		t.Fatalf("parsePortList returned error: %v", err)
	}
	want := []int{19132, 19134, 19135, 19136}
	assertPorts(t, got, want)
}

func TestParsePortListRejectsReversedRange(t *testing.T) {
	if _, err := parsePortList("19140-19132"); err == nil {
		t.Fatal("expected reversed range error")
	}
}

func assertPorts(t *testing.T, got, want []int) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("ports length = %d, want %d (%v)", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("ports[%d] = %d, want %d (%v)", i, got[i], want[i], got)
		}
	}
}
