package cli

import (
	"strings"
	"testing"
)

func TestSupportsColorDetectsJetBrainsTerminal(t *testing.T) {
	clearColorEnv(t)
	t.Setenv("TERMINAL_EMULATOR", "JetBrains-JediTerm")

	if !supportsColor() {
		t.Fatal("expected JetBrains terminal to support color")
	}
}

func TestSupportsColorCanBeForced(t *testing.T) {
	clearColorEnv(t)
	t.Setenv("FORCE_COLOR", "1")

	if !supportsColor() {
		t.Fatal("expected FORCE_COLOR to enable color")
	}
}

func TestSupportsColorNoColorWins(t *testing.T) {
	clearColorEnv(t)
	t.Setenv("NO_COLOR", "1")
	t.Setenv("FORCE_COLOR", "1")
	t.Setenv("TERMINAL_EMULATOR", "JetBrains-JediTerm")

	if supportsColor() {
		t.Fatal("expected NO_COLOR to disable color")
	}
}

func TestSupportsColorRejectsDumbTerminal(t *testing.T) {
	clearColorEnv(t)
	t.Setenv("TERM", "dumb")

	if supportsColor() {
		t.Fatal("expected dumb terminal without overrides to disable color")
	}
}

func TestPrintableWidthIgnoresANSISequences(t *testing.T) {
	value := "\033[36mMCQuery\033[0m > "

	if got, want := printableWidth(value), len("MCQuery > "); got != want {
		t.Fatalf("printableWidth() = %d, want %d", got, want)
	}
}

func TestFrameLinesUseTheConfiguredViewportWidth(t *testing.T) {
	clearColorEnv(t)
	t.Setenv("NO_COLOR", "1")
	t.Setenv("COLUMNS", "100")
	t.Setenv("LINES", "30")

	lines := append(buildHeaderLines("Operations console"), frameContentLine("Ready"))
	lines = append(lines, "╰"+strings.Repeat("─", frameWidth()-2)+"╯")
	for index, line := range lines {
		if got, want := printableWidth(line), frameWidth(); got != want {
			t.Fatalf("line %d width = %d, want %d (%q)", index, got, want, line)
		}
	}
}

func TestHeaderUsesConcreteProductSlogan(t *testing.T) {
	clearColorEnv(t)
	t.Setenv("NO_COLOR", "1")

	header := strings.Join(buildHeaderLines("Operations console"), "\n")
	if !strings.Contains(header, "Check Minecraft servers.") {
		t.Fatalf("header is missing the product slogan: %q", header)
	}
	for _, removed := range []string{"Get clear answers", "intelligence console"} {
		if strings.Contains(header, removed) {
			t.Fatalf("header still contains removed slogan text %q: %q", removed, header)
		}
	}
}

func TestSelectedMenuRowFillsItsPanel(t *testing.T) {
	clearColorEnv(t)
	t.Setenv("FORCE_COLOR", "1")

	row := formatMenuOptionWidth(0, "Direct query: Ping one server", true, 12, 42)
	if got, want := printableWidth(row), 42; got != want {
		t.Fatalf("selected row width = %d, want %d", got, want)
	}
	if !strings.Contains(row, colorSelected) {
		t.Fatalf("selected row is missing its highlight: %q", row)
	}
	if !strings.Contains(colorSelected, "97;44") {
		t.Fatalf("selected row must use high-contrast bright text on a dark background: %q", colorSelected)
	}
}

func TestMainMenuContextExplainsSelectedCapability(t *testing.T) {
	clearColorEnv(t)
	lines := menuContextLines("Operations console", "Direct query: Ping one server", 0, 8, 36)
	joined := strings.Join(lines, "\n")

	for _, expected := range []string{"Direct query", "Bedrock UDP", "latency", "Enter to open"} {
		if !strings.Contains(joined, expected) {
			t.Fatalf("menu context is missing %q in %q", expected, joined)
		}
	}
}

func TestDisplayTruncationPreservesVisibleWidth(t *testing.T) {
	clearColorEnv(t)
	t.Setenv("FORCE_COLOR", "1")
	value := colorize("A deliberately long highlighted value", colorAccent, colorBold)

	got := truncateDisplayText(value, 14)
	if width := printableWidth(got); width != 14 {
		t.Fatalf("truncated width = %d, want 14 (%q)", width, got)
	}
}

func clearColorEnv(t *testing.T) {
	t.Helper()

	for _, name := range []string{
		"ANSICON",
		"CLICOLOR_FORCE",
		"ConEmuANSI",
		"FORCE_COLOR",
		"IDEA_INITIAL_DIRECTORY",
		"NO_COLOR",
		"TERM",
		"TERMINAL_EMULATOR",
		"WT_SESSION",
	} {
		t.Setenv(name, "")
	}
}
