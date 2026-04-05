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

	go enqueueLookupCandidates(ctx, candidates, []string{""}, []string{"com", "net"}, "example", limiter)

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
		if candidate.attempt != 2 {
			t.Fatalf("unexpected second attempt: %d", candidate.attempt)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("second candidate was not released after limiter tick")
	}
}
