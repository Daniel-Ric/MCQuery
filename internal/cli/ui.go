package cli

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
)

const (
	colorReset    = "\033[0m"
	colorDim      = "\033[2m"
	colorAccent   = "\033[36m"
	colorBlue     = "\033[34m"
	colorGreen    = "\033[32m"
	colorRed      = "\033[31m"
	colorWarn     = "\033[33m"
	colorBold     = "\033[1m"
	colorSelected = "\033[97;44m"
)

var errAborted = errors.New("aborted")

var (
	activeFrameLines int
	frameReady       bool
	immersiveUI      bool
)

func beginConsoleSession() func() {
	restoreOutputMode := enableVirtualTerminalOutput()
	noAlternateScreen := strings.TrimSpace(os.Getenv("MCQUERY_NO_ALT_SCREEN"))
	if noAlternateScreen == "1" || strings.EqualFold(noAlternateScreen, "true") || strings.EqualFold(strings.TrimSpace(os.Getenv("TERM")), "dumb") {
		return restoreOutputMode
	}

	immersiveUI = true
	activeFrameLines = 0
	frameReady = false
	fmt.Print("\033[?1049h\033[?25l\033[2J\033[H")

	return func() {
		if !immersiveUI {
			return
		}
		fmt.Print("\033[?25h\033[0m\033[2J\033[H\033[?1049l")
		immersiveUI = false
		activeFrameLines = 0
		frameReady = false
		restoreOutputMode()
	}
}

func setCursorVisible(visible bool) {
	if !immersiveUI {
		return
	}
	if visible {
		fmt.Print("\033[?25h")
		return
	}
	fmt.Print("\033[?25l")
}

func moveCursorTo(row, column int) {
	if row < 1 {
		row = 1
	}
	if column < 1 {
		column = 1
	}
	fmt.Printf("\033[%d;%dH", row, column)
}

func supportsColor() bool {
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	if os.Getenv("FORCE_COLOR") != "" || os.Getenv("CLICOLOR_FORCE") != "" {
		return true
	}
	term := strings.TrimSpace(os.Getenv("TERM"))
	if term != "" && term != "dumb" {
		return true
	}
	if os.Getenv("TERMINAL_EMULATOR") == "JetBrains-JediTerm" || os.Getenv("IDEA_INITIAL_DIRECTORY") != "" {
		return true
	}
	if os.Getenv("WT_SESSION") != "" || os.Getenv("ConEmuANSI") == "ON" || os.Getenv("ANSICON") != "" {
		return true
	}
	_, _, interactive := readTerminalSize()
	return interactive
}

func style(text, color string) string {
	if !supportsColor() || color == "" {
		return text
	}
	return color + text + colorReset
}

func colorize(text string, colors ...string) string {
	return style(text, strings.Join(colors, ""))
}

func promptInput(label, hint, errMsg string) (string, error) {
	body := make([]string, 0, 4)
	if errMsg != "" {
		body = append(body, formatStatus("Input error", errMsg, "warn"))
	}
	if hint != "" {
		body = append(body, formatKeyValue("Hint", hint))
	}
	body = append(body, "")
	prompt := colorize("mcquery", colorAccent, colorBold) + style(" › ", colorDim)
	body = append(body, prompt)
	lines := renderFrame(label, body)
	if immersiveUI {
		moveCursorTo(lines-1, 3+printableWidth(prompt))
	} else {
		moveCursorUp(1)
		moveCursorColumn(3 + printableWidth(prompt))
	}
	setCursorVisible(true)
	defer setCursorVisible(false)
	reader := bufio.NewReader(os.Stdin)
	value, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(value), nil
}

func selectOption(title string, options []string) (int, error) {
	return selectOptionWithInitial(title, options, 0)
}

func selectOptionWithInitial(title string, options []string, initial int) (int, error) {
	if len(options) == 0 {
		return 0, errors.New("no options available")
	}
	fd := int(os.Stdin.Fd())
	state, err := makeRaw(fd)
	if err != nil {
		return 0, err
	}
	defer restore(fd, state)

	selected := clampInt(initial, 0, len(options)-1)
	lines := renderMenuBlock(title, options, selected, "", true)

	reader := bufio.NewReader(os.Stdin)
	for {
		b, err := reader.ReadByte()
		if err != nil {
			return 0, err
		}

		if b >= '1' && b <= '9' {
			index := int(b - '1')
			if index < len(options) {
				return index, nil
			}
		}
		if b == '0' && len(options) >= 10 {
			return 9, nil
		}

		switch b {
		case 3, 'q', 'Q':
			return 0, errAborted
		case '?':
			renderKeyboardHelp()
			if _, err := reader.ReadByte(); err != nil {
				return 0, err
			}
			lines = updateMenu(title, options, selected, "", lines)
		case 9:
			selected = (selected + 1) % len(options)
			lines = updateMenu(title, options, selected, "", lines)
		case 'w', 'W', 'k', 'K':
			if selected > 0 {
				selected--
			}
			lines = updateMenu(title, options, selected, "", lines)
		case 's', 'S', 'j', 'J':
			if selected < len(options)-1 {
				selected++
			}
			lines = updateMenu(title, options, selected, "", lines)
		case 13, 10:
			return selected, nil
		case 27:
			seq, err := readEscapeSequence(reader)
			if err != nil {
				return 0, err
			}
			if seq == "OP" {
				renderKeyboardHelp()
				if _, err := reader.ReadByte(); err != nil {
					return 0, err
				}
			}
			if seq == "[A" || seq == "OA" || seq == "[Z" {
				if selected > 0 {
					selected--
				}
			}
			if seq == "[B" || seq == "OB" {
				if selected < len(options)-1 {
					selected++
				}
			}
			if seq == "[H" || seq == "OH" {
				selected = 0
			}
			if seq == "[F" || seq == "OF" {
				selected = len(options) - 1
			}
			if seq == "[5~" {
				selected = maxInt(0, selected-maxInt(1, terminalHeight()/2))
			}
			if seq == "[6~" {
				selected = minInt(len(options)-1, selected+maxInt(1, terminalHeight()/2))
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
	_ = clear
	start, end := visibleMenuRange(len(options), selected)
	labelWidth := menuLabelWidth(options[start:end])
	body := make([]string, 0, len(options)+8)

	if contentWidth() >= 72 {
		leftWidth := clampInt(contentWidth()*56/100, 34, 60)
		rightWidth := maxInt(18, contentWidth()-leftWidth-3)
		left := []string{
			colorize("ACTIONS", colorAccent, colorBold),
			formatKeyValue("Selection", fmt.Sprintf("%d/%d", selected+1, len(options))),
			"",
		}
		if start > 0 {
			left = append(left, style(fmt.Sprintf("  ↑ %d more", start), colorDim))
		}
		for i := start; i < end; i++ {
			left = append(left, formatMenuOptionWidth(i, options[i], i == selected, labelWidth, leftWidth))
		}
		if end < len(options) {
			left = append(left, style(fmt.Sprintf("  ↓ %d more", len(options)-end), colorDim))
		}

		right := menuContextLines(title, options[selected], selected, len(options), rightWidth)
		body = append(body, joinMenuColumns(left, right, leftWidth, rightWidth)...)
	} else {
		body = append(body, formatKeyValue("Selection", fmt.Sprintf("%d/%d", selected+1, len(options))))
		body = append(body, "")
		if start > 0 {
			body = append(body, style(fmt.Sprintf("  ↑ %d more", start), colorDim))
		}
		for i := start; i < end; i++ {
			body = append(body, formatMenuOptionWidth(i, options[i], i == selected, labelWidth, contentWidth()))
		}
		if end < len(options) {
			body = append(body, style(fmt.Sprintf("  ↓ %d more", len(options)-end), colorDim))
		}
	}
	body = append(body, "")
	if hint == "" {
		hint = "↑↓/W-S move   1-9 jump   Enter open   ? help   Q back"
	}
	body = append(body, formatHint(hint))
	return renderFrame(title, body)
}

func updateMenu(title string, options []string, selected int, hint string, lines int) int {
	_ = lines
	return renderMenuBlock(title, options, selected, hint, true)
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
	clearCurrentFrame()
	activeFrameLines = 0
	frameReady = false
}

func renderTextPage(title, content string) {
	body := formatPageBody(strings.Split(content, "\n"))
	capacity := pageContentCapacity()
	if len(body) > capacity {
		body = append(append([]string(nil), body[:capacity]...), "", formatHint("More content available • press Enter to open the pager"))
	}
	renderFrame(title, body)
}

func renderTextPageAndWait(title, content string) error {
	body := formatPageBody(strings.Split(content, "\n"))
	if !immersiveUI {
		renderFrame(title, body)
		return waitForEnter()
	}
	return browseTextPage(title, body)
}

func renderPage(title string, lines []string) {
	renderFrame(title, formatPageBody(lines))
}

func formatPageBody(lines []string) []string {
	body := make([]string, 0, len(lines))
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			body = append(body, "")
			continue
		}
		for _, wrapped := range wrapDisplayLine(line, contentWidth()) {
			body = append(body, formatPageLine(wrapped))
		}
	}
	return body
}

func pageContentCapacity() int {
	return maxInt(3, terminalHeight()-8)
}

func browseTextPage(title string, body []string) error {
	fd := int(os.Stdin.Fd())
	state, err := makeRaw(fd)
	if err != nil {
		renderFrame(title, body)
		return waitForEnter()
	}
	defer restore(fd, state)
	setCursorVisible(false)

	reader := bufio.NewReader(os.Stdin)
	offset := 0
	for {
		capacity := pageContentCapacity()
		maxOffset := maxInt(0, len(body)-capacity)
		offset = clampInt(offset, 0, maxOffset)
		end := minInt(len(body), offset+capacity)
		visible := append([]string(nil), body[offset:end]...)
		visible = append(visible, "")
		if len(body) > capacity {
			page := offset/capacity + 1
			pages := (len(body) + capacity - 1) / capacity
			visible = append(visible, formatKeyValue("View", fmt.Sprintf("page %d/%d • lines %d-%d of %d", page, pages, offset+1, end, len(body))))
			visible = append(visible, formatHint("↑↓/J-K scroll   PgUp/PgDn page   Home/End jump   Enter/Q close"))
		} else {
			visible = append(visible, formatHint("Enter/Q close   ? shortcuts"))
		}
		renderFrame(title, visible)

		key, err := reader.ReadByte()
		if err != nil {
			return err
		}
		switch key {
		case 3:
			return errAborted
		case 10, 13, 'q', 'Q':
			return nil
		case 'j', 'J', 's', 'S':
			offset = minInt(maxOffset, offset+1)
		case 'k', 'K', 'w', 'W':
			offset = maxInt(0, offset-1)
		case ' ', 'f', 'F':
			offset = minInt(maxOffset, offset+capacity)
		case 'b', 'B':
			offset = maxInt(0, offset-capacity)
		case 'g':
			offset = 0
		case 'G':
			offset = maxOffset
		case '?':
			renderKeyboardHelp()
			if _, err := reader.ReadByte(); err != nil {
				return err
			}
		case 27:
			sequence, err := readEscapeSequence(reader)
			if err != nil {
				return err
			}
			switch sequence {
			case "[A", "OA":
				offset = maxInt(0, offset-1)
			case "[B", "OB":
				offset = minInt(maxOffset, offset+1)
			case "[5~":
				offset = maxInt(0, offset-capacity)
			case "[6~":
				offset = minInt(maxOffset, offset+capacity)
			case "[H", "OH":
				offset = 0
			case "[F", "OF":
				offset = maxOffset
			case "OP":
				renderKeyboardHelp()
				if _, err := reader.ReadByte(); err != nil {
					return err
				}
			}
		}
	}
}

func renderKeyboardHelp() {
	body := []string{
		colorize("NAVIGATION", colorAccent, colorBold),
		formatKeyValue("↑ / ↓", "move through choices or scroll results"),
		formatKeyValue("W-S / J-K", "keyboard alternatives for navigation"),
		formatKeyValue("1-9", "open a visible menu action directly"),
		formatKeyValue("Home / End", "jump to the first or last entry"),
		formatKeyValue("PgUp / PgDn", "move by a complete result page"),
		"",
		colorize("ACTIONS", colorAccent, colorBold),
		formatKeyValue("Enter", "open, confirm or close a result"),
		formatKeyValue("Tab", "advance to the next menu action"),
		formatKeyValue("Q / Ctrl+C", "go back or close the console"),
		formatKeyValue("? / F1", "show this shortcut reference"),
		"",
		formatStatus("Tip", "P or Space pauses long scans; Q cancels them", "success"),
		"",
		formatHint("Press any key to return"),
	}
	renderFrame("Keyboard shortcuts", body)
}

func renderSpinnerPage(title, message, frame string) {
	renderLiveFrame(title, []string{message}, frame, true)
}

func renderHeader(title string) {
	renderFrame(title, nil)
}

func renderHeaderLines(title string) int {
	return renderFrame(title, nil)
}

func renderFrame(title string, body []string) int {
	lines := make([]string, 0, len(body)+5)
	lines = append(lines, buildHeaderLines(title)...)
	for _, line := range body {
		lines = append(lines, frameContentLine(line))
	}
	lines = append(lines, style("╰"+strings.Repeat("─", frameWidth()-2)+"╯", colorDim))
	return drawFrame(lines)
}

func drawFrame(lines []string) int {
	if len(lines) == 0 {
		lines = []string{""}
	}
	lines = fitFrameToViewport(lines)
	if immersiveUI {
		fmt.Print("\033[H")
		frameReady = true
	} else if frameReady {
		moveCursorUp(activeFrameLines - 1)
	} else {
		frameReady = true
	}

	renderLines := maxInt(activeFrameLines, len(lines))
	if renderLines <= 0 {
		renderLines = 1
	}
	for i := 0; i < renderLines; i++ {
		clearLine()
		if i < len(lines) {
			fmt.Print(lines[i])
		}
		if i < renderLines-1 {
			fmt.Print("\n")
		}
	}

	targetLine := maxInt(len(lines), 1)
	if immersiveUI {
		moveCursorTo(targetLine, 1)
	} else if renderLines > targetLine {
		moveCursorUp(renderLines - targetLine)
	}
	col := 1
	if len(lines) > 0 {
		col = printableWidth(lines[len(lines)-1]) + 1
	}
	moveCursorColumn(clampInt(col, 1, terminalWidth()))
	activeFrameLines = targetLine
	return len(lines)
}

func clearCurrentFrame() {
	if !frameReady || activeFrameLines <= 0 {
		return
	}
	if immersiveUI {
		fmt.Print("\033[2J\033[H")
		activeFrameLines = 0
		frameReady = false
		return
	}
	moveCursorUp(activeFrameLines - 1)
	for i := 0; i < activeFrameLines; i++ {
		clearLine()
		if i < activeFrameLines-1 {
			fmt.Print("\n")
		}
	}
	moveCursorUp(activeFrameLines - 1)
}

func moveCursorColumn(col int) {
	fmt.Print("\r")
	if col > 1 {
		fmt.Printf("\033[%dC", col-1)
	}
}

func fitFrameToViewport(lines []string) []string {
	height := terminalHeight()
	if height <= 1 || len(lines) <= height {
		return lines
	}
	if height < 4 {
		return append([]string(nil), lines[:height]...)
	}
	hidden := len(lines) - height + 2
	clipped := append([]string(nil), lines[:height-2]...)
	clipped = append(clipped, frameContentLine(style(fmt.Sprintf("… %d more lines below", hidden), colorDim)))
	clipped = append(clipped, lines[len(lines)-1])
	return clipped
}

func printableWidth(value string) int {
	width := 0
	escapeState := 0
	for _, r := range value {
		switch escapeState {
		case 1:
			if r == '[' {
				escapeState = 2
			} else {
				escapeState = 0
			}
			continue
		case 2:
			if r >= '@' && r <= '~' {
				escapeState = 0
			}
			continue
		}
		if r == '\033' {
			escapeState = 1
			continue
		}
		width++
	}
	return width
}

func buildHeaderLines(title string) []string {
	width := frameWidth()
	inner := contentWidth()
	brand := colorize("MCQUERY", colorAccent, colorBold) + " " + style("v"+appVersion, colorDim)
	badge := colorize("BEDROCK + JAVA", colorGreen, colorBold)
	subtitle := style("Check Minecraft servers.", colorDim)
	return []string{
		style("╭"+strings.Repeat("─", width-2)+"╮", colorDim),
		frameContentLine(alignFrameSides(brand, badge, inner)),
		frameContentLine(subtitle),
		frameTitleDivider(title),
	}
}

func frameWidth() int {
	return clampInt(terminalWidth()-2, 36, 120)
}

func frameContentLine(value string) string {
	inner := contentWidth()
	return style("│", colorDim) + " " + padDisplay(value, inner) + " " + style("│", colorDim)
}

func frameTitleDivider(title string) string {
	width := frameWidth()
	title = strings.ToUpper(strings.TrimSpace(title))
	if title == "" {
		return style("├"+strings.Repeat("─", width-2)+"┤", colorDim)
	}
	title = truncateText(title, maxInt(1, width-8))
	dashes := maxInt(0, width-len([]rune(title))-5)
	return style("├─ ", colorDim) + colorize(title, colorAccent, colorBold) + style(" "+strings.Repeat("─", dashes)+"┤", colorDim)
}

func alignFrameSides(left, right string, width int) string {
	gap := width - printableWidth(left) - printableWidth(right)
	if gap < 2 {
		return truncateDisplayText(left, width)
	}
	return left + strings.Repeat(" ", gap) + right
}

func terminalSize() (int, int) {
	if width, height, ok := readTerminalSize(); ok {
		return clampInt(width, 36, 160), clampInt(height, 12, 80)
	}
	width := envInt("COLUMNS", 72)
	height := envInt("LINES", 24)
	return clampInt(width, 36, 160), clampInt(height, 12, 80)
}

func terminalWidth() int {
	width, _ := terminalSize()
	return width
}

func terminalHeight() int {
	_, height := terminalSize()
	return height
}

func contentWidth() int {
	return maxInt(32, frameWidth()-4)
}

func envInt(name string, fallback int) int {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	return value
}

func visibleMenuRange(total, selected int) (int, int) {
	if total <= 0 {
		return 0, 0
	}
	// Reserve space for the branded frame, selection metadata, scroll markers,
	// and the persistent shortcut footer.
	headerAndFooter := 12
	available := terminalHeight() - headerAndFooter
	if available < 4 {
		available = 4
	}
	if available > total {
		available = total
	}
	start := selected - available/2
	if start < 0 {
		start = 0
	}
	if start+available > total {
		start = total - available
	}
	if start < 0 {
		start = 0
	}
	return start, start + available
}

func formatMenuOption(index int, option string, selected bool, labelWidth int) string {
	return formatMenuOptionWidth(index, option, selected, labelWidth, maxInt(12, contentWidth()-2))
}

func formatMenuOptionWidth(index int, option string, selected bool, labelWidth int, width int) string {
	width = maxInt(12, width)
	if selected && supportsColor() {
		plain := fmt.Sprintf(" › %d  %s", index+1, strings.TrimSpace(option))
		plain = padRight(truncateText(plain, width), width)
		return style(plain, colorSelected)
	}
	prefix := " "
	if selected {
		prefix = "›"
	}
	key := style(fmt.Sprintf("%2d", index+1), colorDim)
	text := formatOptionText(option, selected, labelWidth, maxInt(8, width-7))
	return fmt.Sprintf("%s %s  %s", prefix, key, text)
}

func joinMenuColumns(left, right []string, leftWidth, rightWidth int) []string {
	rows := maxInt(len(left), len(right))
	result := make([]string, 0, rows)
	divider := style(" │ ", colorDim)
	for row := 0; row < rows; row++ {
		leftLine := ""
		if row < len(left) {
			leftLine = left[row]
		}
		rightLine := ""
		if row < len(right) {
			rightLine = right[row]
		}
		result = append(result, padDisplay(leftLine, leftWidth)+divider+truncateDisplayText(rightLine, rightWidth))
	}
	return result
}

func menuContextLines(title, option string, selected, total, width int) []string {
	label, detail, hasDetail := splitOptionLabel(option)
	if !hasDetail {
		detail = menuFallbackDetail(label)
	}
	lines := []string{
		colorize("OVERVIEW", colorAccent, colorBold),
		style(fmt.Sprintf("%s  •  %d of %d", title, selected+1, total), colorDim),
		"",
		colorize(label, colorBold),
	}
	for _, line := range wrapDisplayLine(detail, width) {
		lines = append(lines, line)
	}
	lines = append(lines, "")
	for _, line := range menuCapabilityLines(label) {
		for index, wrapped := range wrapDisplayLine("• "+line, width) {
			if index == 0 {
				lines = append(lines, style("• ", colorAccent)+strings.TrimPrefix(wrapped, "• "))
			} else {
				lines = append(lines, "  "+wrapped)
			}
		}
	}
	lines = append(lines, "", formatStatus("Ready", "Enter to open", "success"))
	return lines
}

func menuFallbackDetail(label string) string {
	lower := strings.ToLower(strings.TrimSpace(label))
	switch {
	case lower == "back":
		return "Return to the previous workspace without changing anything."
	case lower == "exit":
		return "Close MCQuery and restore the terminal exactly as it was."
	case strings.Contains(lower, "enabled"):
		return "Use the enabled setting for future operations."
	case strings.Contains(lower, "disabled"):
		return "Keep this feature disabled for future operations."
	default:
		return "Open this option and continue with the guided workflow."
	}
}

func menuCapabilityLines(label string) []string {
	lower := strings.ToLower(label)
	switch {
	case strings.Contains(lower, "direct query"):
		return []string{"Bedrock UDP and Java TCP", "SRV discovery, latency, players and MOTD", "Optional exports and Bedrock join links"}
	case strings.Contains(lower, "favorites"):
		return []string{"Reusable server profiles", "Fast repeat checks", "Local configuration storage"}
	case strings.Contains(lower, "batch"):
		return []string{"Mixed-edition target lists", "Pause and cancel controls", "Consolidated result export"}
	case strings.Contains(lower, "port scan"):
		return []string{"Edition-aware common ports", "Custom ranges", "Concurrent probing"}
	case strings.Contains(lower, "domain lookup"):
		return []string{"Subdomain and TLD combinations", "Live rate and ETA telemetry", "Sorting and result filters"}
	case strings.Contains(lower, "settings"):
		return []string{"Timeouts, retries and IP mode", "Concurrency and rate limits", "Output paths and presets"}
	case strings.Contains(lower, "update"):
		return []string{"Release and tag comparison", "Five-second network timeout", "No automatic installation"}
	default:
		return []string{"Keyboard-first navigation", "Changes remain local until confirmed"}
	}
}

func formatOptionText(option string, selected bool, labelWidth int, width int) string {
	label, detail, hasDetail := splitOptionLabel(option)
	if hasDetail {
		if labelWidth <= 0 {
			labelWidth = minInt(maxInt(len([]rune(label)), 12), 28)
		}
		label = truncateText(label, labelWidth)
		detailWidth := maxInt(8, width-labelWidth-2)
		detail = truncateText(detail, detailWidth)
		paddedLabel := padRight(label, labelWidth)
		if selected {
			return fmt.Sprintf("%s  %s", colorize(paddedLabel, colorAccent, colorBold), formatOptionDetail(detail, true))
		}
		return fmt.Sprintf("%s  %s", style(paddedLabel, colorDim), formatOptionDetail(detail, false))
	}
	if selected {
		return colorize(truncateText(option, width), colorAccent, colorBold)
	}
	lower := strings.ToLower(strings.TrimSpace(label))
	switch {
	case lower == "back" || lower == "exit":
		return style(truncateText(label, width), colorDim)
	case strings.Contains(lower, "reset") || strings.Contains(lower, "delete") || strings.Contains(lower, "clear"):
		return style(truncateText(label, width), colorWarn)
	}
	return truncateText(label, width)
}

func splitOptionLabel(option string) (string, string, bool) {
	option = strings.TrimSpace(option)
	if parts := strings.SplitN(option, ":", 2); len(parts) == 2 {
		label := strings.TrimSpace(parts[0])
		detail := strings.TrimSpace(parts[1])
		if label != "" && detail != "" {
			return label, detail, true
		}
	}
	return option, "", false
}

func menuLabelWidth(options []string) int {
	width := 0
	for _, option := range options {
		label, _, ok := splitOptionLabel(option)
		if !ok {
			continue
		}
		if length := len([]rune(label)); length > width {
			width = length
		}
	}
	if width == 0 {
		return 0
	}
	return clampInt(width, 12, 28)
}

func padRight(value string, width int) string {
	padding := width - len([]rune(value))
	if padding <= 0 {
		return value
	}
	return value + strings.Repeat(" ", padding)
}

func padDisplay(value string, width int) string {
	value = truncateDisplayText(value, width)
	padding := width - printableWidth(value)
	if padding <= 0 {
		return value
	}
	return value + strings.Repeat(" ", padding)
}

func truncateDisplayText(value string, width int) string {
	if width <= 0 {
		return ""
	}
	if printableWidth(value) <= width {
		return value
	}
	target := maxInt(1, width-1)
	var builder strings.Builder
	visible := 0
	escapeState := 0
	for _, r := range value {
		if escapeState != 0 {
			builder.WriteRune(r)
			if escapeState == 1 && r == '[' {
				escapeState = 2
			} else if escapeState == 1 || (escapeState == 2 && r >= '@' && r <= '~') {
				escapeState = 0
			}
			continue
		}
		if r == '\033' {
			builder.WriteRune(r)
			escapeState = 1
			continue
		}
		if visible >= target {
			break
		}
		builder.WriteRune(r)
		visible++
	}
	if supportsColor() {
		builder.WriteString(colorReset)
	}
	if width > 1 {
		builder.WriteRune('…')
	}
	return builder.String()
}

func formatOptionDetail(value string, selected bool) string {
	trimmed := strings.TrimSpace(value)
	lower := strings.ToLower(trimmed)
	switch lower {
	case "true", "enabled", "on":
		return colorize(trimmed, colorGreen, colorBold)
	case "false", "disabled", "off":
		return style(trimmed, colorDim)
	case "auto":
		return style(trimmed, colorAccent)
	default:
		if selected {
			return style(trimmed, colorBold)
		}
		return style(trimmed, colorDim)
	}
}

func formatValue(value string) string {
	return " " + formatOptionDetail(value, false)
}

func formatHint(text string) string {
	return style(text, colorDim)
}

func formatKeyValue(label, value string) string {
	return fmt.Sprintf("%s %s", style(label+":", colorDim), value)
}

func formatStatus(label, value, level string) string {
	color := colorAccent
	switch level {
	case "success":
		color = colorGreen
	case "warn":
		color = colorWarn
	case "error":
		color = colorRed
	}
	return fmt.Sprintf("%s %s", colorize(label+":", color, colorBold), value)
}

func formatPageLine(line string) string {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" || strings.Contains(line, "\033[") {
		return line
	}
	switch {
	case isSectionTitle(trimmed):
		return colorize(trimmed, colorAccent, colorBold)
	case trimmed == resultEntryDivider:
		return colorize(trimmed, colorAccent, colorBold)
	case strings.HasPrefix(trimmed, "[OK]"):
		return colorize("[OK]", colorGreen, colorBold) + strings.TrimPrefix(trimmed, "[OK]")
	case strings.HasPrefix(trimmed, "[ERR]"):
		return colorize("[ERR]", colorRed, colorBold) + strings.TrimPrefix(trimmed, "[ERR]")
	case strings.HasPrefix(trimmed, "[WARN]"):
		return colorize("[WARN]", colorWarn, colorBold) + strings.TrimPrefix(trimmed, "[WARN]")
	case strings.HasPrefix(trimmed, "Saved result:") || strings.HasPrefix(trimmed, "Server icon saved:"):
		return formatStatus(strings.SplitN(trimmed, ":", 2)[0], strings.TrimSpace(strings.SplitN(trimmed, ":", 2)[1]), "success")
	case strings.HasPrefix(trimmed, "Add link") || strings.HasPrefix(trimmed, "Join link") || strings.HasPrefix(trimmed, "URL:"):
		return formatColonLine(trimmed, colorBlue)
	case strings.HasPrefix(trimmed, "Status:"):
		value := strings.TrimSpace(strings.TrimPrefix(trimmed, "Status:"))
		level := "success"
		if strings.Contains(strings.ToLower(value), "canceled") || strings.Contains(strings.ToLower(value), "failed") {
			level = "error"
		}
		if strings.Contains(strings.ToLower(value), "available") {
			level = "warn"
		}
		return formatStatus("Status", value, level)
	case strings.HasPrefix(trimmed, "- "):
		return style("- ", colorAccent) + formatBulletContent(strings.TrimSpace(strings.TrimPrefix(trimmed, "- ")))
	case strings.Contains(trimmed, ":"):
		return formatColonLine(trimmed, "")
	default:
		return line
	}
}

func isSectionTitle(value string) bool {
	switch value {
	case "Summary", "Details", "Server", "World", "Players", "Network", "Identity", "Security", "Modding", "Performance", "Debug", "Skipped", "Results", "Matches", "Update", "Links":
		return true
	default:
		return strings.HasPrefix(value, "Match ")
	}
}

func formatBulletContent(value string) string {
	if strings.Contains(value, ":") {
		return formatColonLine(value, "")
	}
	return value
}

func formatColonLine(value, valueColor string) string {
	parts := strings.SplitN(value, ":", 2)
	label := strings.TrimSpace(parts[0])
	content := strings.TrimSpace(parts[1])
	if valueColor == "" {
		return fmt.Sprintf("%s %s", style(label+":", colorDim), content)
	}
	return fmt.Sprintf("%s %s", style(label+":", colorDim), style(content, valueColor))
}

func truncateText(value string, width int) string {
	if width <= 0 {
		return ""
	}
	runes := []rune(value)
	if len(runes) <= width {
		return value
	}
	if width <= 3 {
		return string(runes[:width])
	}
	return string(runes[:width-3]) + "..."
}

func wrapDisplayLine(line string, width int) []string {
	if width <= 0 || strings.Contains(line, "\033[") {
		return []string{line}
	}
	runes := []rune(line)
	if len(runes) <= width {
		return []string{line}
	}
	indent := leadingWhitespace(line)
	wrapped := make([]string, 0, (len(runes)/width)+1)
	remaining := strings.TrimRight(line, "\r\n")
	first := true
	for len([]rune(remaining)) > width {
		cut := findWrapCut(remaining, width)
		part := strings.TrimRight(remaining[:cut], " ")
		if !first && indent != "" {
			part = indent + strings.TrimLeft(part, " ")
		}
		wrapped = append(wrapped, part)
		remaining = strings.TrimLeft(remaining[cut:], " ")
		first = false
		if indent != "" {
			width = maxInt(16, contentWidth()-len([]rune(indent)))
		}
	}
	if remaining != "" {
		if !first && indent != "" {
			remaining = indent + remaining
		}
		wrapped = append(wrapped, remaining)
	}
	if len(wrapped) == 0 {
		return []string{line}
	}
	return wrapped
}

func leadingWhitespace(value string) string {
	var builder strings.Builder
	for _, r := range value {
		if r != ' ' && r != '\t' {
			break
		}
		builder.WriteRune(r)
	}
	return builder.String()
}

func findWrapCut(value string, width int) int {
	runes := []rune(value)
	if len(runes) <= width {
		return len(value)
	}
	cutRunes := width
	for i := width; i > width/2; i-- {
		if runes[i-1] == ' ' || runes[i-1] == '\t' {
			cutRunes = i
			break
		}
	}
	return len(string(runes[:cutRunes]))
}

func progressBarWidth() int {
	return clampInt(terminalWidth()-42, 12, 56)
}

func clampInt(value, minValue, maxValue int) int {
	if value < minValue {
		return minValue
	}
	if value > maxValue {
		return maxValue
	}
	return value
}

func waitForEnter() error {
	fmt.Println()
	fmt.Print(formatHint("Press Enter to continue"))
	activeFrameLines += 2
	reader := bufio.NewReader(os.Stdin)
	_, err := reader.ReadString('\n')
	return err
}

func withSpinner(title string, message func(frame int) string, tick time.Duration, action func() (string, error)) (string, error) {
	resultCh := make(chan struct {
		result string
		err    error
	}, 1)
	go func() {
		defer func() {
			if recovered := recover(); recovered != nil {
				resultCh <- struct {
					result string
					err    error
				}{err: internalPanicError(recovered)}
			}
		}()
		result, err := action()
		resultCh <- struct {
			result string
			err    error
		}{result: result, err: err}
	}()

	frames := []string{"◐", "◓", "◑", "◒"}
	ticker := time.NewTicker(tick)
	defer ticker.Stop()

	frame := 0
	renderLiveFrame(title, strings.Split(message(frame), "\n"), frames[frame], true)
	frame = (frame + 1) % len(frames)
	for {
		select {
		case res := <-resultCh:
			return res.result, res.err
		case <-ticker.C:
			parts := strings.Split(message(frame), "\n")
			if len(parts) == 0 {
				parts = []string{""}
			}
			renderLiveFrame(title, parts, frames[frame], true)
			frame = (frame + 1) % len(frames)
		}
	}
}

type spinnerControl struct {
	ctx       context.Context
	cancel    context.CancelFunc
	paused    atomic.Bool
	cancelled atomic.Bool
}

func (c *spinnerControl) Context() context.Context {
	if c == nil {
		return context.Background()
	}
	return c.ctx
}

func (c *spinnerControl) IsPaused() bool {
	return c != nil && c.paused.Load()
}

func (c *spinnerControl) IsCancelled() bool {
	return c != nil && c.cancelled.Load()
}

func (c *spinnerControl) togglePause() {
	if c == nil || c.IsCancelled() {
		return
	}
	c.paused.Store(!c.paused.Load())
}

func (c *spinnerControl) Cancel() {
	if c == nil {
		return
	}
	c.cancelled.Store(true)
	c.paused.Store(false)
	c.cancel()
}

func withControlledSpinner(title string, message func(frame int, control *spinnerControl) string, tick time.Duration, action func(control *spinnerControl) (string, error)) (string, error) {
	ctx, cancel := context.WithCancel(context.Background())
	control := &spinnerControl{ctx: ctx, cancel: cancel}
	defer cancel()

	resultCh := make(chan struct {
		result string
		err    error
	}, 1)
	go func() {
		defer func() {
			if recovered := recover(); recovered != nil {
				resultCh <- struct {
					result string
					err    error
				}{err: internalPanicError(recovered)}
			}
		}()
		result, err := action(control)
		resultCh <- struct {
			result string
			err    error
		}{result: result, err: err}
	}()

	fd := int(os.Stdin.Fd())
	inputEnabled := terminalSupportsKeyPolling()
	var state *terminalState
	if inputEnabled {
		rawState, err := makeRaw(fd)
		if err == nil {
			state = rawState
			defer restore(fd, state)
		} else {
			inputEnabled = false
		}
	}

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt)
	defer signal.Stop(signals)

	frames := []string{"◐", "◓", "◑", "◒"}
	ticker := time.NewTicker(tick)
	defer ticker.Stop()

	frame := 0
	renderControlledSpinnerFrame(title, message, frame, frames[frame], control, inputEnabled)
	frame = (frame + 1) % len(frames)
	for {
		select {
		case res := <-resultCh:
			return res.result, res.err
		case <-signals:
			control.Cancel()
		case <-ticker.C:
			if inputEnabled {
				pollSpinnerControls(fd, control)
			}
			renderControlledSpinnerFrame(title, message, frame, frames[frame], control, inputEnabled)
			frame = (frame + 1) % len(frames)
		}
	}
}

func renderControlledSpinnerFrame(title string, message func(frame int, control *spinnerControl) string, frame int, spinner string, control *spinnerControl, inputEnabled bool) {
	parts := strings.Split(message(frame, control), "\n")
	if len(parts) == 0 {
		parts = []string{""}
	}
	if inputEnabled {
		parts = append(parts, "Controls: P pause/resume, Q abort")
	} else {
		parts = append(parts, "Controls: Ctrl+C abort")
	}
	renderLiveFrame(title, parts, spinner, !control.IsPaused() && !control.IsCancelled())
}

func renderLiveFrame(title string, parts []string, frame string, spin bool) int {
	if len(parts) == 0 {
		parts = []string{""}
	}
	body := make([]string, 0, len(parts)+2)
	liveColor := colorGreen
	if !spin {
		liveColor = colorWarn
	}
	body = append(body, colorize("● LIVE OPERATION", liveColor, colorBold), "")
	for i, part := range parts {
		line := formatLiveLine(fitLiveLine(part))
		if i == len(parts)-1 && spin {
			line = fmt.Sprintf("%s %s", line, style(frame, colorAccent))
		}
		body = append(body, line)
	}
	return renderFrame(title+" / live", body)
}

func pollSpinnerControls(fd int, control *spinnerControl) {
	for i := 0; i < 16; i++ {
		b, ok, err := readPendingByte(fd)
		if err != nil || !ok {
			return
		}
		switch b {
		case 3, 27, 'q', 'Q':
			control.Cancel()
		case 'p', 'P', ' ':
			control.togglePause()
		}
	}
}

func renderLiveLines(parts []string, frame string, lastLines int, spin bool) int {
	renderLines := len(parts)
	if lastLines > renderLines {
		renderLines = lastLines
	}
	if lastLines > 1 {
		moveCursorUp(lastLines - 1)
	}
	for i := 0; i < renderLines; i++ {
		clearLine()
		if i < len(parts) {
			line := formatLiveLine(fitLiveLine(parts[i]))
			if i == len(parts)-1 && spin {
				line = fmt.Sprintf("%s %s", line, style(frame, colorAccent))
			}
			fmt.Print(line)
		}
		if i < renderLines-1 {
			fmt.Print("\n")
		}
	}
	return len(parts)
}

func clearSpinnerLines(lines int) {
	if lines > 1 {
		moveCursorUp(lines - 1)
	}
	for i := 0; i < lines; i++ {
		clearLine()
		if i < lines-1 {
			fmt.Print("\n")
		}
	}
	fmt.Println()
}

func fitLiveLine(line string) string {
	if strings.Contains(line, "\033[") {
		return line
	}
	return truncateText(line, contentWidth())
}

func formatLiveLine(line string) string {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" || strings.Contains(line, "\033[") {
		return line
	}
	switch {
	case strings.HasPrefix(trimmed, "Controls:"):
		return style(trimmed, colorDim)
	case strings.HasPrefix(trimmed, "Status:"):
		value := strings.TrimSpace(strings.TrimPrefix(trimmed, "Status:"))
		level := "success"
		if value == "paused" || value == "calibrating" {
			level = "warn"
		}
		if value == "aborting" || value == "canceled" {
			level = "error"
		}
		return formatStatus("Status", value, level)
	case strings.Contains(trimmed, ":"):
		return formatColonLine(trimmed, "")
	default:
		return style(trimmed, colorDim)
	}
}

func renderProgressBar(completed, total, frame, width int) string {
	if width <= 0 {
		width = progressBarWidth()
	}
	width = clampInt(width, 8, maxInt(8, terminalWidth()-24))
	if total <= 0 {
		return style("╺", colorDim) + style(strings.Repeat("─", width), colorDim) + style("╸", colorDim)
	}
	if completed < 0 {
		completed = 0
	}
	if completed > total {
		completed = total
	}
	filled := (completed * width) / total
	if filled > width {
		filled = width
	}
	empty := width - filled
	var builder strings.Builder
	builder.WriteString(style("╺", colorDim))
	if filled > 0 {
		builder.WriteString(style(strings.Repeat("━", filled), colorGreen))
	}
	if completed < total && empty > 0 {
		animation := []string{"◆", "◇"}
		builder.WriteString(style(animation[frame%len(animation)], colorAccent))
		empty--
	}
	if empty > 0 {
		builder.WriteString(style(strings.Repeat("─", empty), colorDim))
	}
	builder.WriteString(style("╸", colorDim))
	return builder.String()
}
