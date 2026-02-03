package cli

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"
)

const (
	colorReset  = "\033[0m"
	colorDim    = "\033[2m"
	colorAccent = "\033[36m"
	colorWarn   = "\033[33m"
	colorBold   = "\033[1m"
)

var errAborted = errors.New("abgebrochen")

func supportsColor() bool {
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	term := strings.TrimSpace(os.Getenv("TERM"))
	return term != "" && term != "dumb"
}

func style(text, color string) string {
	if !supportsColor() || color == "" {
		return text
	}
	return color + text + colorReset
}

func promptInput(label, hint, errMsg string) (string, error) {
	clearScreen()
	fmt.Println(style(label, colorAccent))
	if hint != "" {
		fmt.Println(style(hint, colorDim))
	}
	if errMsg != "" {
		fmt.Println(style(errMsg, colorWarn))
	}
	fmt.Println()
	fmt.Print(style("> ", colorAccent))
	reader := bufio.NewReader(os.Stdin)
	value, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(value), nil
}

func selectOption(title string, options []string) (int, error) {
	if len(options) == 0 {
		return 0, errors.New("keine Auswahloptionen verfügbar")
	}
	fd := int(os.Stdin.Fd())
	state, err := makeRaw(fd)
	if err != nil {
		return 0, err
	}
	defer restore(fd, state)

	selected := 0
	lines := renderMenuBlock(title, options, selected, "", true)

	reader := bufio.NewReader(os.Stdin)
	for {
		b, err := reader.ReadByte()
		if err != nil {
			return 0, err
		}

		switch b {
		case 3:
			return 0, errAborted
		case 'w', 'k':
			if selected > 0 {
				selected--
			}
			lines = updateMenu(title, options, selected, "", lines)
		case 's', 'j':
			if selected < len(options)-1 {
				selected++
			}
			lines = updateMenu(title, options, selected, "", lines)
		case 13, 10:
			clearScreen()
			return selected, nil
		case 27:
			seq, err := readEscapeSequence(reader)
			if err != nil {
				return 0, err
			}
			if seq == "[A" || seq == "OA" {
				if selected > 0 {
					selected--
				}
			}
			if seq == "[B" || seq == "OB" {
				if selected < len(options)-1 {
					selected++
				}
			}
			lines = updateMenu(title, options, selected, "", lines)
		case 0, 224:
			code, err := reader.ReadByte()
			if err != nil {
				return 0, err
			}
			if code == 72 && selected > 0 {
				selected--
			}
			if code == 80 && selected < len(options)-1 {
				selected++
			}
			lines = updateMenu(title, options, selected, "", lines)
		}
	}
}

func readEscapeSequence(reader *bufio.Reader) (string, error) {
	b1, err := reader.ReadByte()
	if err != nil {
		return "", err
	}
	b2, err := reader.ReadByte()
	if err != nil {
		return "", err
	}
	if b1 == '[' && b2 >= '0' && b2 <= '9' {
		b3, err := reader.ReadByte()
		if err != nil {
			return "", err
		}
		return string([]byte{b1, b2, b3}), nil
	}
	return string([]byte{b1, b2}), nil
}

func renderMenuBlock(title string, options []string, selected int, hint string, clear bool) int {
	if clear {
		clearScreen()
	}
	lines := 0
	lines += printLine(style(title, colorAccent))
	lines += printLine("")
	for i, option := range options {
		if i == selected {
			lines += printLine(fmt.Sprintf("%s %s", style(">", colorAccent+colorBold), style(option, colorBold)))
			continue
		}
		lines += printLine(fmt.Sprintf("  %s", option))
	}
	lines += printLine("")
	if hint == "" {
		hint = "Pfeiltasten (oder W/S), Enter zum Bestätigen"
	}
	lines += printLine(style(hint, colorDim))
	return lines
}

func updateMenu(title string, options []string, selected int, hint string, lines int) int {
	if lines > 0 {
		moveCursorUp(lines)
	}
	return renderMenuBlock(title, options, selected, hint, false)
}

func printLine(value string) int {
	clearLine()
	fmt.Println(value)
	return 1
}

func clearLine() {
	fmt.Print("\r\033[K")
}

func moveCursorUp(lines int) {
	if lines <= 0 {
		return
	}
	fmt.Printf("\033[%dA", lines)
}

func clearScreen() {
	fmt.Print("\033[H\033[2J")
}

func renderTextPage(title, content string) {
	lines := strings.Split(content, "\n")
	renderPage(title, lines)
}

func renderPage(title string, lines []string) {
	clearScreen()
	fmt.Println(style(title, colorAccent))
	fmt.Println()
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			fmt.Println()
			continue
		}
		fmt.Println(line)
	}
}

func renderSpinnerPage(title, message, frame string) {
	renderPage(title, []string{fmt.Sprintf("%s %s", style(message, colorDim), style(frame, colorAccent))})
}

func withSpinner(title, message string, tick time.Duration, action func() (string, error)) (string, error) {
	resultCh := make(chan struct {
		result string
		err    error
	}, 1)
	go func() {
		result, err := action()
		resultCh <- struct {
			result string
			err    error
		}{result: result, err: err}
	}()

	clearScreen()
	fmt.Println(style(title, colorAccent))
	fmt.Println()

	frames := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	ticker := time.NewTicker(tick)
	defer ticker.Stop()

	frame := 0
	lastLen := 0
	for {
		select {
		case res := <-resultCh:
			if lastLen > 0 {
				fmt.Print("\r", strings.Repeat(" ", lastLen), "\r")
			}
			fmt.Println()
			return res.result, res.err
		case <-ticker.C:
			line := fmt.Sprintf("%s %s", style(message, colorDim), style(frames[frame], colorAccent))
			if lastLen > 0 && len(line) < lastLen {
				line += strings.Repeat(" ", lastLen-len(line))
			}
			fmt.Print("\r", line)
			lastLen = len(line)
			frame = (frame + 1) % len(frames)
		}
	}
}
