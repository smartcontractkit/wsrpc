package backoff

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestExponential(t *testing.T) {
	t.Parallel()

	var (
		jitter = 0.1
	)

	es := NewExponential(Config{
		BaseDelay:  500 * time.Millisecond,
		Jitter:     jitter,
		Multiplier: 2.0,
		MaxDelay:   5 * time.Second,
	})

	var expResults = []time.Duration{500, 1000, 2000, 4000, 5000, 5000, 5000, 5000, 5000, 5000}
	for i, d := range expResults {
		expResults[i] = d * time.Millisecond
	}

	for _, expected := range expResults {
		// Assert that the next backoff falls in the expected range.
		var actualInterval = es.NextBackOff()

		assert.InDelta(t, expected, actualInterval, jitter*float64(expected))
	}

	// Test Reset
	es.Reset()
	assert.InDelta(t, expResults[0], es.NextBackOff(), jitter*float64(expResults[0]))
}
