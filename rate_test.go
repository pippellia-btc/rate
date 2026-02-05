package rate

import (
	"sync"
	"testing"
	"time"
)

func TestAllow(t *testing.T) {
	refiller := FlatRefiller[string]{
		InitialTokens:     100,
		MaxTokens:         100,
		TokensPerInterval: 0,
	}

	limiter := NewLimiter[string](refiller)
	entity := "lewis"
	allowed := 0

	for {
		ok, err := limiter.Allow(entity, 1)
		if err != nil {
			t.Fatalf("failed to allow: %v", err)
		}

		if !ok {
			break
		}
		allowed++
	}

	if allowed != 100 {
		t.Fatalf("lewis should have been allowed exactly 100 times, got %d", allowed)
	}
}

func TestPenalize(t *testing.T) {
	refiller := FlatRefiller[string]{
		InitialTokens:     100,
		MaxTokens:         100,
		TokensPerInterval: 0,
	}

	limiter := NewLimiter[string](refiller)
	entity := "lewis"

	// Penalize the entity by 150 tokens (more than initial)
	if err := limiter.Penalize(entity, 150); err != nil {
		t.Fatalf("failed to penalize: %v", err)
	}

	// Entity should now be at -50 tokens, so Allow should fail
	ok, err := limiter.Allow(entity, 1)
	if err != nil {
		t.Fatalf("failed to allow: %v", err)
	}
	if ok {
		t.Fatalf("lewis should have been rejected after being penalized")
	}
}

func TestPenalizeUnknownEntity(t *testing.T) {
	refiller := FlatRefiller[string]{
		InitialTokens:     100,
		MaxTokens:         100,
		TokensPerInterval: 0,
	}

	limiter := NewLimiter[string](refiller)
	entity := "unknown"

	// Penalize an entity that has never been seen before
	if err := limiter.Penalize(entity, 50); err != nil {
		t.Fatalf("failed to penalize unknown entity: %v", err)
	}

	// Entity should have a bucket now, with 100 - 50 = 50 tokens
	allowed := 0
	for {
		ok, err := limiter.Allow(entity, 1)
		if err != nil {
			t.Fatalf("failed to allow: %v", err)
		}
		if !ok {
			break
		}
		allowed++
	}

	if allowed != 50 {
		t.Fatalf("unknown should have been allowed exactly 50 times after penalty, got %d", allowed)
	}
}

// Run this test with go test --race
func TestConcurrency(t *testing.T) {
	refiller := FlatRefiller[string]{
		InitialTokens:     1000,
		MaxTokens:         1000,
		TokensPerInterval: 100,
		Interval:          time.Hour,
	}

	limiter := NewLimiter(refiller)
	entity := "lewis"

	wg := sync.WaitGroup{}
	wg.Add(10_000)

	for range 10_000 {
		go func() {
			defer wg.Done()
			if _, err := limiter.Allow(entity, 1); err != nil {
				t.Errorf("failed to allow: %v", err)
			}
		}()
	}
	wg.Wait()
}
