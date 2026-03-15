package cli

import (
	"fmt"
	"math"
	"runtime"
	"strings"
	"sync"
	"time"

	"UWP-TCP-Con/internal/ping"
)

type lookupProgressView struct {
	mu            sync.Mutex
	edition       ping.Edition
	startedAt     time.Time
	progress      ping.LookupProgress
	total         int
	concurrency   int
	rateLimit     int
	timeout       time.Duration
	retryCount    int
	retryDelay    time.Duration
	lastObserved  time.Time
	lastCompleted int
	smoothedRate  float64
	initialRate   float64
}

func newLookupProgressView(settings Settings, config LookupConfig) *lookupProgressView {
	total := countLookupSubdomains(config.Subdomains) * countLookupEndings(config.Endings)
	concurrency := resolveLookupConcurrency(settings.LookupConcurrency, total)
	view := &lookupProgressView{
		edition:     config.Edition,
		startedAt:   time.Now(),
		total:       total,
		concurrency: concurrency,
		rateLimit:   settings.LookupRateLimit,
		timeout:     settings.RequestTimeout(),
		retryCount:  settings.RetryCount,
		retryDelay:  settings.RetryDelay(),
	}
	view.initialRate = estimateLookupInitialRate(config.Edition, concurrency, settings.LookupRateLimit, view.timeout, view.retryCount, view.retryDelay)
	return view
}

func (v *lookupProgressView) Observe(progress ping.LookupProgress) {
	v.mu.Lock()
	defer v.mu.Unlock()

	if progress.Total > 0 {
		v.total = progress.Total
		if v.concurrency > progress.Total {
			v.concurrency = progress.Total
		}
	}

	now := time.Now()
	if progress.Completed > v.lastCompleted {
		if !v.lastObserved.IsZero() {
			deltaCompleted := progress.Completed - v.lastCompleted
			deltaTime := now.Sub(v.lastObserved)
			if deltaCompleted > 0 && deltaTime > 0 {
				instantRate := float64(deltaCompleted) / deltaTime.Seconds()
				weight := 0.24
				if progress.Completed < maxInt(6, v.concurrency/2) {
					weight = 0.4
				}
				if v.smoothedRate == 0 {
					v.smoothedRate = instantRate
				} else {
					v.smoothedRate = (v.smoothedRate * (1 - weight)) + (instantRate * weight)
				}
			}
		}
		if v.lastObserved.IsZero() {
			elapsed := now.Sub(v.startedAt)
			if elapsed > 0 {
				v.smoothedRate = float64(progress.Completed) / elapsed.Seconds()
			}
		}
		v.lastObserved = now
		v.lastCompleted = progress.Completed
	}

	v.progress = progress
}

func (v *lookupProgressView) Render(frame int) string {
	v.mu.Lock()
	progress := v.progress
	total := v.total
	concurrency := v.concurrency
	rateLimit := v.rateLimit
	timeout := v.timeout
	retryCount := v.retryCount
	retryDelay := v.retryDelay
	lastObserved := v.lastObserved
	smoothedRate := v.smoothedRate
	initialRate := v.initialRate
	startedAt := v.startedAt
	edition := v.edition
	v.mu.Unlock()

	if total <= 0 {
		total = maxInt(progress.Total, 1)
	}
	if progress.Total <= 0 {
		progress.Total = total
	}
	if progress.Completed > progress.Total {
		progress.Completed = progress.Total
	}

	elapsed := time.Since(startedAt)
	averageRate := calculateLookupObservedRate(progress.Completed, elapsed)
	projectedRate := blendLookupRate(progress.Completed, progress.Total, initialRate, smoothedRate, averageRate)
	projectedRate = dampLookupRate(projectedRate, lastObserved, edition, timeout, retryCount, retryDelay)

	remaining := maxInt(progress.Total-progress.Completed, 0)
	etaText := "calibrating"
	if remaining == 0 && progress.Total > 0 {
		etaText = "done"
	} else if projectedRate > 0 {
		etaText = formatLookupDuration(time.Duration(float64(time.Second) * (float64(remaining) / projectedRate)))
	} else if initialRate > 0 {
		etaText = "~" + formatLookupDuration(time.Duration(float64(time.Second)*(float64(remaining)/initialRate)))
	}

	windowRate := initialRate
	if averageRate > 0 {
		windowRate = averageRate
	}
	if smoothedRate > 0 {
		windowRate = smoothedRate
	}

	subdomain := progress.Subdomain
	if subdomain == "" {
		subdomain = "(none)"
	}

	ending := progress.Ending
	if ending == "" {
		ending = "-"
	}

	host := progress.Host
	if host == "" {
		host = "Preparing next candidate"
	}

	percent := calculateLookupCompletion(progress.Completed, progress.Total)
	estimatedTimeLine := fmt.Sprintf("Estimated time: %s • Elapsed: %s • Window: %s", etaText, formatLookupDuration(elapsed), formatLookupRate(windowRate))
	if etaText != "calibrating" && etaText != "done" {
		estimatedTimeLine = fmt.Sprintf("Estimated time: %s left • Elapsed: %s • Window: %s", etaText, formatLookupDuration(elapsed), formatLookupRate(windowRate))
	}

	lines := []string{
		fmt.Sprintf("%s %s/%s (%.1f%%)", buildLookupProgressBar(progress.Completed, progress.Total, frame, 28), formatLookupNumber(progress.Completed), formatLookupNumber(progress.Total), percent),
		estimatedTimeLine,
		fmt.Sprintf("Pipeline: %d workers • %s remaining • Confidence: %s", concurrency, formatLookupNumber(remaining), lookupConfidence(progress.Completed, progress.Total)),
		fmt.Sprintf("Stage: %s • Rate cap: %s", lookupStage(progress.Completed, progress.Total), formatLookupRateCap(rateLimit)),
		fmt.Sprintf("Subdomain: %s", subdomain),
		fmt.Sprintf("Ending: %s", ending),
		fmt.Sprintf("Host: %s", host),
	}
	return strings.Join(lines, "\n")
}

func buildLookupProgressBar(completed, total, frame, width int) string {
	if total <= 0 || width <= 0 {
		return "[----------------------------]"
	}
	if completed > total {
		completed = total
	}
	filled := (completed * width) / total
	if filled > width {
		filled = width
	}
	bar := make([]rune, width)
	for i := 0; i < width; i++ {
		bar[i] = '░'
	}
	for i := 0; i < filled; i++ {
		bar[i] = '█'
	}
	if completed < total && filled < width {
		animation := []rune{'▏', '▎', '▍', '▌', '▋', '▊', '▉'}
		bar[filled] = animation[frame%len(animation)]
	}
	return fmt.Sprintf("[%s]", string(bar))
}

func resolveLookupConcurrency(configured, total int) int {
	if total <= 0 {
		return 0
	}
	if configured <= 0 {
		configured = runtime.NumCPU() * 8
		if configured < 32 {
			configured = 32
		}
	}
	if configured > total {
		configured = total
	}
	return configured
}

func estimateLookupInitialRate(edition ping.Edition, concurrency, rateLimit int, timeout time.Duration, retryCount int, retryDelay time.Duration) float64 {
	if concurrency <= 0 {
		return 0
	}
	span := expectedLookupProbeSpan(edition, timeout, retryCount, retryDelay)
	if span <= 0 {
		return 0
	}
	rate := float64(concurrency) / span.Seconds()
	if rateLimit > 0 && rate > float64(rateLimit) {
		rate = float64(rateLimit)
	}
	return rate
}

func expectedLookupProbeSpan(edition ping.Edition, timeout time.Duration, retryCount int, retryDelay time.Duration) time.Duration {
	base := 550 * time.Millisecond
	if edition == ping.EditionJava {
		base = 750 * time.Millisecond
	}
	if timeout > 0 {
		projected := time.Duration(float64(timeout) * 0.55)
		minimum := base / 2
		if projected < minimum {
			projected = minimum
		}
		if projected > timeout {
			projected = timeout
		}
		base = projected
	}
	span := base * time.Duration(retryCount+1)
	if retryCount > 0 {
		span += retryDelay * time.Duration(retryCount)
	}
	if span < 350*time.Millisecond {
		span = 350 * time.Millisecond
	}
	return span
}

func blendLookupRate(completed, total int, initialRate, smoothedRate, averageRate float64) float64 {
	observedRate := 0.0
	switch {
	case smoothedRate > 0 && averageRate > 0:
		observedRate = (smoothedRate * 0.64) + (averageRate * 0.36)
	case smoothedRate > 0:
		observedRate = smoothedRate
	case averageRate > 0:
		observedRate = averageRate
	}

	if observedRate <= 0 {
		return initialRate
	}
	if initialRate <= 0 || total <= 0 {
		return observedRate
	}

	warmupSamples := maxInt(10, minInt(64, total/8))
	warmupWeight := clamp(float64(completed)/float64(warmupSamples), 0, 1)
	return (initialRate * (1 - warmupWeight)) + (observedRate * warmupWeight)
}

func dampLookupRate(rate float64, lastObserved time.Time, edition ping.Edition, timeout time.Duration, retryCount int, retryDelay time.Duration) float64 {
	if rate <= 0 || lastObserved.IsZero() {
		return rate
	}
	quietFor := time.Since(lastObserved)
	grace := expectedLookupProbeSpan(edition, timeout, retryCount, retryDelay)
	if grace < 1200*time.Millisecond {
		grace = 1200 * time.Millisecond
	}
	if quietFor <= grace {
		return rate
	}
	damp := 1 / math.Sqrt(float64(quietFor)/float64(grace))
	if damp < 0.45 {
		damp = 0.45
	}
	return rate * damp
}

func calculateLookupObservedRate(completed int, elapsed time.Duration) float64 {
	if completed <= 0 || elapsed <= 0 {
		return 0
	}
	return float64(completed) / elapsed.Seconds()
}

func calculateLookupAverageRate(completed int, startedAt time.Time) float64 {
	return calculateLookupObservedRate(completed, time.Since(startedAt))
}

func calculateLookupCompletion(completed, total int) float64 {
	if total <= 0 {
		return 0
	}
	return (float64(completed) / float64(total)) * 100
}

func formatLookupDuration(value time.Duration) string {
	if value < 0 {
		value = 0
	}
	rounded := value.Round(time.Second)
	if rounded < time.Minute {
		return fmt.Sprintf("%ds", int(rounded.Seconds()))
	}
	if rounded < time.Hour {
		minutes := int(rounded / time.Minute)
		seconds := int((rounded % time.Minute) / time.Second)
		return fmt.Sprintf("%02d:%02d", minutes, seconds)
	}
	if rounded < 24*time.Hour {
		hours := int(rounded / time.Hour)
		minutes := int((rounded % time.Hour) / time.Minute)
		seconds := int((rounded % time.Minute) / time.Second)
		return fmt.Sprintf("%d:%02d:%02d", hours, minutes, seconds)
	}
	days := int(rounded / (24 * time.Hour))
	hours := int((rounded % (24 * time.Hour)) / time.Hour)
	return fmt.Sprintf("%dd %02dh", days, hours)
}

func formatLookupRate(value float64) string {
	if value <= 0 {
		return "0 req/s"
	}
	if value >= 100 {
		return fmt.Sprintf("%.0f req/s", value)
	}
	if value >= 10 {
		return fmt.Sprintf("%.1f req/s", value)
	}
	return fmt.Sprintf("%.2f req/s", value)
}

func formatLookupRateCap(rateLimit int) string {
	if rateLimit <= 0 {
		return "uncapped"
	}
	return fmt.Sprintf("%d req/s", rateLimit)
}

func formatLookupNumber(value int) string {
	raw := fmt.Sprintf("%d", value)
	if value < 1000 && value > -1000 {
		return raw
	}
	sign := ""
	if value < 0 {
		sign = "-"
		raw = raw[1:]
	}
	parts := make([]string, 0, (len(raw)+2)/3)
	for len(raw) > 3 {
		parts = append([]string{raw[len(raw)-3:]}, parts...)
		raw = raw[:len(raw)-3]
	}
	parts = append([]string{raw}, parts...)
	return sign + strings.Join(parts, ",")
}

func lookupStage(completed, total int) string {
	if total <= 0 || completed <= 0 {
		return "queueing"
	}
	ratio := float64(completed) / float64(total)
	switch {
	case ratio < 0.05:
		return "calibrating"
	case ratio < 0.25:
		return "surface scan"
	case ratio < 0.6:
		return "broad sweep"
	case ratio < 0.9:
		return "deep sweep"
	case ratio < 1:
		return "final sweep"
	default:
		return "complete"
	}
}

func lookupConfidence(completed, total int) string {
	if total <= 0 || completed <= 0 {
		return "low"
	}
	ratio := float64(completed) / float64(total)
	switch {
	case completed < 8 || ratio < 0.03:
		return "low"
	case completed < 24 || ratio < 0.12:
		return "medium"
	case ratio < 0.5:
		return "high"
	default:
		return "very high"
	}
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func clamp(value, minValue, maxValue float64) float64 {
	if value < minValue {
		return minValue
	}
	if value > maxValue {
		return maxValue
	}
	return value
}
