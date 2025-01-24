package github_release

import (
	"testing"
	"time"
)

func TestBackoffMultiple(t *testing.T) {
	b := &Backoff{}
	for i := 0; i < 19; i++ {
		p, ok := b.Pause()
		t.Logf("iteration %d pausing for %s", i, p)
		if !ok {
			t.Fatalf("hit the pause timeout after %d pauses", i)
		}
	}
}

func TestBackoffTimeout(t *testing.T) {
	var elapsed time.Duration
	b := &Backoff{}
	for i := 0; i < 40; i++ {
		p, ok := b.Pause()
		elapsed += p
		t.Logf("iteration %d pausing for %s (total %s)", i, p, elapsed)
		if !ok {
			break
		}
	}
	if _, ok := b.Pause(); ok {
		t.Fatalf("did not hit the pause timeout")
	}

	if elapsed > maxElapsedTime {
		t.Fatalf("waited too long: %s > %s", elapsed, maxElapsedTime)
	}
}
