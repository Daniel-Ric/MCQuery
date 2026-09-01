package cli

import (
	"encoding/csv"
	"strings"
	"testing"

	"UWP-TCP-Con/internal/ping"
)

func TestCSVExportKeepsExtendedHeaderAndRowsAligned(t *testing.T) {
	var output strings.Builder
	writer := csv.NewWriter(&output)
	records := []exportRecord{{
		Mode:         "direct",
		Edition:      "java",
		Host:         "example.test",
		Port:         25565,
		Success:      true,
		PlayerSample: []ping.JavaPlayer{{Name: "Alex", ID: "uuid"}},
		Mods:         []ping.JavaMod{{ID: "example", Version: "1.0"}},
	}}

	if err := writeCSVExport(writer, records); err != nil {
		t.Fatalf("writeCSVExport returned an error: %v", err)
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		t.Fatalf("CSV writer returned an error: %v", err)
	}

	rows, err := csv.NewReader(strings.NewReader(output.String())).ReadAll()
	if err != nil {
		t.Fatalf("read exported CSV: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("CSV row count = %d, want 2", len(rows))
	}
	if len(rows[0]) != len(rows[1]) {
		t.Fatalf("CSV header has %d columns but record has %d", len(rows[0]), len(rows[1]))
	}
	header := strings.Join(rows[0], ",")
	if !strings.Contains(header, "player_sample") || !strings.Contains(header, "server_guid") || !strings.Contains(header, "mods") {
		t.Fatalf("extended columns are missing from %q", header)
	}
}
