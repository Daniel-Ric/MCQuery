package cli

import (
	"errors"
	"os"
	"strings"
	"testing"
)

func TestFriendlyErrorMessageFileNotFound(t *testing.T) {
	if got, want := friendlyErrorMessage(os.ErrNotExist), "File or path was not found"; got != want {
		t.Fatalf("friendlyErrorMessage() = %q, want %q", got, want)
	}
}

func TestFormatErrorPageIncludesRawDetailWhenUseful(t *testing.T) {
	page := formatErrorPage(errors.New("dial tcp: i/o timeout"))

	if !strings.Contains(page, "Network request timed out") {
		t.Fatalf("expected friendly timeout message in %q", page)
	}
	if !strings.Contains(page, "Raw error: dial tcp: i/o timeout") {
		t.Fatalf("expected raw detail in %q", page)
	}
}

func TestAppendWarningText(t *testing.T) {
	got := appendWarningText("Server\nStatus: online", "Result export failed", errors.New("disk full"))

	if !strings.Contains(got, "[WARN] Result export failed: disk full") {
		t.Fatalf("expected warning line in %q", got)
	}
}

func TestNilResultFallbacksDoNotPanic(t *testing.T) {
	if got, want := formatResultSummary(nil, resultFormatOptions{}), "Server\nStatus: unavailable"; got != want {
		t.Fatalf("formatResultSummary(nil) = %q, want %q", got, want)
	}
	if got, want := compactResultStatus(nil), "no response"; got != want {
		t.Fatalf("compactResultStatus(nil) = %q, want %q", got, want)
	}
}
