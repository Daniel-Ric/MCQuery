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
	colorReset  = "\033[0m"
	colorDim    = "\033[2m"
	colorAccent = "\033[36m"
	colorBlue   = "\033[34m"
	colorGreen  = "\033[32m"
	colorRed    = "\033[31m"
	colorWarn   = "\033[33m"
	colorBold   = "\033[1m"
)

var errAborted = errors.New("aborted")

var (
	activeFrameLines int
	frameReady       bool
)

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
	return os.Getenv("WT_SESSION") != "" || os.Getenv("ConEmuANSI") == "ON" || os.Getenv("ANSICON") != ""
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
	body = append(body, colorize("mcquery", colorAccent, colorBold)+style(" > ", colorDim))
	renderFrame(label, body)
	reader := bufio.NewReader(os.Stdin)
	value, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	activeFrameLines++
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
	_ = clear
	body := make([]string, 0, len(options)+6)
	body = append(body, formatKeyValue("Selection", fmt.Sprintf("%d/%d", selected+1, len(options))))
	body = append(body, "")
	start, end := visibleMenuRange(len(options), selected)
	labelWidth := menuLabelWidth(options[start:end])
	if start > 0 {
		body = append(body, style(fmt.Sprintf("  %d more above", start), colorDim))
	}
	for i := start; i < end; i++ {
		option := options[i]
		body = append(body, formatMenuOption(i, option, i == selected, labelWidth))
	}
	if end < len(options) {
		body = append(body, style(fmt.Sprintf("  %d more below", len(options)-end), colorDim))
	}
	body = append(body, "")
	if hint == "" {
		hint = "Arrows/W-S move | 1-9 jump | Enter select | Q back"
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
	lines := strings.Split(content, "\n")
	renderPage(title, lines)
}

func renderTextPageAndWait(title, content string) error {
	renderTextPage(title, content)
	return waitForEnter()
}

func renderPage(title string, lines []string) {
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
	renderFrame(title, body)
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
	lines := make([]string, 0, len(body)+4)
	lines = append(lines, buildHeaderLines(title)...)
	lines = append(lines, "")
	lines = append(lines, body...)
	return drawFrame(lines)
}

func drawFrame(lines []string) int {
	if len(lines) == 0 {
		lines = []string{""}
	}
	lines = fitFrameToViewport(lines)
	if frameReady {
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
	if renderLines > targetLine {
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
	hidden := len(lines) - height + 1
	clipped := append([]string(nil), lines[:height]...)
	clipped[height-1] = style(fmt.Sprintf("... %d more lines below", hidden), colorDim)
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
	width := contentWidth()
	brand := colorize("MCQuery", colorAccent, colorBold)
	version := style("v"+appVersion, colorDim)
	plainTitle := strings.TrimSpace(title)
	if strings.TrimSpace(title) != "" && title != "MCQuery" {
		plainTitle = truncateText(plainTitle, maxInt(12, width-18))
		brand = fmt.Sprintf("%s %s  %s", brand, version, colorize(plainTitle, colorBold))
	} else {
		brand = fmt.Sprintf("%s %s", brand, version)
	}
	subtitle := truncateText("Minecraft server query for Bedrock and Java", width)
	return []string{
		brand,
		style(subtitle, colorDim),
		style(strings.Repeat("-", width), colorDim),
	}
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
	return clampInt(terminalWidth()-2, 34, 112)
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
	headerAndFooter := 8
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
	prefix := " "
	if selected {
		prefix = colorize(">", colorAccent, colorBold)
	}
	key := style(fmt.Sprintf("%2d", index+1), colorDim)
	text := formatOptionText(option, selected, labelWidth, maxInt(12, contentWidth()-8))
	return fmt.Sprintf("%s %s  %s", prefix, key, text)
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
	case "Summary", "Details", "Server", "Players", "Performance", "Debug", "Skipped", "Results", "Matches", "Update", "Links":
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

	frames := []string{"|", "/", "-", "\\"}
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

	frames := []string{"|", "/", "-", "\\"}
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
	body := make([]string, 0, len(parts))
	for i, part := range parts {
		line := formatLiveLine(fitLiveLine(part))
		if i == len(parts)-1 && spin {
			line = fmt.Sprintf("%s %s", line, style(frame, colorAccent))
		}
		body = append(body, line)
	}
	return renderFrame(title, body)
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
		return fmt.Sprintf("[%s]", style(strings.Repeat("-", width), colorDim))
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
	builder.WriteString(style("[", colorDim))
	if filled > 0 {
		builder.WriteString(style(strings.Repeat("#", filled), colorGreen))
	}
	if completed < total && empty > 0 {
		animation := []string{"|", "/", "-", "\\"}
		builder.WriteString(style(animation[frame%len(animation)], colorAccent))
		empty--
	}
	if empty > 0 {
		builder.WriteString(style(strings.Repeat("-", empty), colorDim))
	}
	builder.WriteString(style("]", colorDim))
	return builder.String()
}
