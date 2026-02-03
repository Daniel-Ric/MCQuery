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
	renderMenu(title, options, selected, "")

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
			renderMenu(title, options, selected, "")
		case 's', 'j':
			if selected < len(options)-1 {
				selected++
			}
			renderMenu(title, options, selected, "")
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
			renderMenu(title, options, selected, "")
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
			renderMenu(title, options, selected, "")
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

func renderMenu(title string, options []string, selected int, hint string) {
	clearScreen()
	fmt.Println(style(title, colorAccent))
	fmt.Println()
	for i, option := range options {
		if i == selected {
			fmt.Printf("%s %s\n", style(">", colorAccent+colorBold), style(option, colorBold))
			continue
		}
		fmt.Printf("  %s\n", option)
	}
	fmt.Println()
	if hint == "" {
		hint = "Pfeiltasten (oder W/S), Enter zum Bestätigen"
	}
	fmt.Println(style(hint, colorDim))
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

	frames := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	ticker := time.NewTicker(tick)
	defer ticker.Stop()

	frame := 0
	for {
		select {
		case res := <-resultCh:
			return res.result, res.err
		case <-ticker.C:
			renderSpinnerPage(title, message, frames[frame])
			frame = (frame + 1) % len(frames)
		}
	}
}
