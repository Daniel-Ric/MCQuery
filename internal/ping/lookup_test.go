package ping

import (
	"context"
	"testing"
	"time"
)

func TestEnqueueLookupCandidatesWaitsForLimiter(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	candidates := make(chan lookupCandidate, 2)
	limiter := make(chan time.Time)

	go enqueueLookupCandidates(ctx, candidates, []string{""}, []string{"com", "net"}, []int{19132}, "example", limiter, nil)

	select {
	case candidate := <-candidates:
		t.Fatalf("candidate emitted before limiter tick: %+v", candidate)
	case <-time.After(20 * time.Millisecond):
	}

	limiter <- time.Now()

	select {
	case candidate, ok := <-candidates:
		if !ok {
			t.Fatal("candidate channel closed before first candidate")
		}
		if candidate.host != "example.com" {
			t.Fatalf("unexpected first host: %s", candidate.host)
		}
		if candidate.port != 19132 {
			t.Fatalf("unexpected first port: %d", candidate.port)
		}
		if candidate.attempt != 1 {
			t.Fatalf("unexpected first attempt: %d", candidate.attempt)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("first candidate was not released after limiter tick")
	}

	select {
	case candidate := <-candidates:
		t.Fatalf("second candidate emitted before second limiter tick: %+v", candidate)
	case <-time.After(20 * time.Millisecond):
	}

	limiter <- time.Now()

	select {
	case candidate, ok := <-candidates:
		if !ok {
			t.Fatal("candidate channel closed before second candidate")
		}
		if candidate.host != "example.net" {
			t.Fatalf("unexpected second host: %s", candidate.host)
		}
		if candidate.port != 19132 {
			t.Fatalf("unexpected second port: %d", candidate.port)
		}
		if candidate.attempt != 2 {
			t.Fatalf("unexpected second attempt: %d", candidate.attempt)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("second candidate was not released after limiter tick")
	}
}

func TestEnqueueLookupCandidatesIncludesPorts(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	candidates := make(chan lookupCandidate, 4)
	go enqueueLookupCandidates(ctx, candidates, []string{""}, []string{"com"}, []int{19132, 19133}, "example", nil, nil)

	first, ok := <-candidates
	if !ok {
		t.Fatal("candidate channel closed before first candidate")
	}
	second, ok := <-candidates
	if !ok {
		t.Fatal("candidate channel closed before second candidate")
	}
	if first.host != "example.com" || second.host != "example.com" {
		t.Fatalf("unexpected hosts: %s, %s", first.host, second.host)
	}
	if first.port != 19132 || second.port != 19133 {
		t.Fatalf("unexpected ports: %d, %d", first.port, second.port)
	}
	if first.attempt != 1 || second.attempt != 2 {
		t.Fatalf("unexpected attempts: %d, %d", first.attempt, second.attempt)
	}
}

func TestDefaultLookupConcurrency(t *testing.T) {
	target := AutoLookupConcurrencyTarget()
	if target < lookupAutoWorkersMin {
		t.Fatalf("auto target below minimum: %d", target)
	}

	if got := DefaultLookupConcurrency(target + 10); got != target {
		t.Fatalf("expected uncapped auto target %d, got %d", target, got)
	}
	if got := DefaultLookupConcurrency(7); got != 7 {
		t.Fatalf("expected total cap, got %d", got)
	}
	if got := DefaultLookupConcurrency(0); got != 0 {
		t.Fatalf("expected no workers for empty lookup, got %d", got)
	}
}
