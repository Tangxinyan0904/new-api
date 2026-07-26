package common

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestInMemoryRateLimiterCheckDoesNotRecord(t *testing.T) {
	var limiter InMemoryRateLimiter
	limiter.Init(0)

	assert.True(t, limiter.Check("success:1", 1, 60))
	assert.True(t, limiter.Check("success:1", 1, 60))
	limiter.Record("success:1", 60)
	assert.False(t, limiter.Check("success:1", 1, 60))
}

func TestInMemoryRateLimiterRequestWithCountAndDelete(t *testing.T) {
	var limiter InMemoryRateLimiter
	limiter.Init(0)

	allowed, count := limiter.RequestWithCount("detection:1", 2, 60)
	assert.True(t, allowed)
	assert.Equal(t, 1, count)

	allowed, count = limiter.RequestWithCount("detection:1", 2, 60)
	assert.True(t, allowed)
	assert.Equal(t, 2, count)

	allowed, count = limiter.RequestWithCount("detection:1", 2, 60)
	assert.False(t, allowed)
	assert.Equal(t, 2, count)

	limiter.Delete("detection:1")
	allowed, count = limiter.RequestWithCount("detection:1", 2, 60)
	assert.True(t, allowed)
	assert.Equal(t, 1, count)
}

func TestInMemoryRateLimiterZeroLimitIsUnlimitedAndUnrecorded(t *testing.T) {
	var limiter InMemoryRateLimiter
	limiter.Init(0)

	for range 3 {
		allowed, count := limiter.RequestWithCount("unlimited:1", 0, 60)
		assert.True(t, allowed)
		assert.Zero(t, count)
	}
}
