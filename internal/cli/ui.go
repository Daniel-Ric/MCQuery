package cli

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strings"
)

func promptInput(label string) (string, error) {
	reader := bufio.NewReader(os.Stdin)
	fmt.Printf("%s: ", label)
	value, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(value), nil
}

func selectOption(title string, options []string) (int, error) {
	fd := int(os.Stdin.Fd())
	state, err := makeRaw(fd)
	if err != nil {
		return 0, err
	}
	defer restore(fd, state)

	selected := 0
	renderMenu(title, options, selected)

	reader := bufio.NewReader(os.Stdin)
	for {
		b, err := reader.ReadByte()
		if err != nil {
			return 0, err
		}

		switch b {
		case 3:
			return 0, errors.New("abgebrochen")
		case 13, 10:
			clearScreen()
			return selected, nil
		case 27:
			seq, err := readEscapeSequence(reader)
			if err != nil {
				return 0, err
			}
			if seq == "[A" {
				if selected > 0 {
					selected--
				}
			}
			if seq == "[B" {
				if selected < len(options)-1 {
					selected++
				}
			}
			renderMenu(title, options, selected)
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
	return string([]byte{b1, b2}), nil
}

func renderMenu(title string, options []string, selected int) {
	clearScreen()
	fmt.Println(title)
	fmt.Println()
	for i, option := range options {
		if i == selected {
			fmt.Printf("> %s\n", option)
			continue
		}
		fmt.Printf("  %s\n", option)
	}
	fmt.Println()
	fmt.Println("Pfeiltasten zum Navigieren, Enter zum Bestätigen")
}

func clearScreen() {
	fmt.Print("\033[H\033[2J")
}
