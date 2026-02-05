package rate

import (
	"sync"
	"testing"
)

func TestAllow(t *testing.T) {
	tokens := 100.0
	refiller := NoRefill[string]{InitialTokens: tokens}
	limiter := NewLimiter[string](refiller)

	for {
		if !limiter.Allow("lewis", 1) {
			break
		}
		tokens--
	}

	if tokens != 0 {
		t.Fatalf("lewis should have been allowed exactly %f times, got %f", refiller.InitialTokens, tokens)
	}
}

func TestPenalize(t *testing.T) {
	refiller := NoRefill[string]{InitialTokens: 100}
	limiter := NewLimiter[string](refiller)

	limiter.Penalize("lewis", 150)
	balance := limiter.Balance("lewis")
	if balance != -50 {
		t.Fatalf("lewis should have been penalized to -50 tokens, got %f", balance)
	}
}

func TestPenalizeUnknownEntity(t *testing.T) {
	refiller := NoRefill[string]{InitialTokens: 100}
	limiter := NewLimiter[string](refiller)

	limiter.Penalize("unknown", 50)
	balance := limiter.Balance("unknown")
	if balance != 50 {
		t.Fatalf("unknown should have been penalized to 50 tokens, got %f", balance)
	}
}

// Run this test with go test --race
func TestConcurrency(t *testing.T) {
	refiller := NoRefill[string]{InitialTokens: 1000}
	limiter := NewLimiter(refiller)

	wg := sync.WaitGroup{}
	wg.Add(10_000)

	for range 10_000 {
		go func() {
			defer wg.Done()
			limiter.Allow("lewis", 1)
		}()
	}
	wg.Wait()
}
