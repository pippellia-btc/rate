package rate

import (
	"testing"
	"time"
)

func TestFlatRefill(t *testing.T) {
	refiller := FlatRefiller[string]{
		MaxTokens:         100,
		TokensPerInterval: 10,
		Interval:          time.Hour,
	}

	tests := []struct {
		name     string
		bucket   *Bucket
		expected *Bucket
	}{
		{
			name:     "no refill (too soon)",
			bucket:   &Bucket{Tokens: 10, LastRefill: time.Now()},
			expected: &Bucket{Tokens: 10, LastRefill: time.Now()},
		},
		{
			name:     "2 refills",
			bucket:   &Bucket{Tokens: 10, LastRefill: time.Now().Add(-2 * time.Hour)},
			expected: &Bucket{Tokens: 30, LastRefill: time.Now()},
		},
		{
			name:     "full refill",
			bucket:   &Bucket{Tokens: 10, LastRefill: time.Now().Add(-24 * time.Hour)},
			expected: &Bucket{Tokens: 100, LastRefill: time.Now()},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			refiller.Refill("", test.bucket)

			if test.bucket.Tokens != test.expected.Tokens {
				t.Fatalf("expected tokens %v, got %v", test.expected.Tokens, test.bucket.Tokens)
			}

			if test.expected.LastRefill.Sub(test.bucket.LastRefill) > time.Millisecond {
				t.Fatalf("expected last refill %v, got %v", test.expected.LastRefill, test.bucket.LastRefill)
			}
		})
	}
}
