package cli

import (
	"testing"

	"UWP-TCP-Con/internal/ping"
)

func TestResolveLookupConcurrencyUsesPingDefault(t *testing.T) {
	total := ping.AutoLookupConcurrencyTarget() + 10

	got := resolveLookupConcurrency(0, total)
	if got != ping.DefaultLookupConcurrency(total) {
		t.Fatalf("expected ping default concurrency, got %d", got)
	}
}

func TestResolveLookupConcurrencyCapsManualValue(t *testing.T) {
	got := resolveLookupConcurrency(512, 12)
	if got != 12 {
		t.Fatalf("expected concurrency capped to total, got %d", got)
	}
}
