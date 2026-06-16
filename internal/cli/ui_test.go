package cli

import "testing"

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
