package template

import (
	"math/rand"
	"time"
)

const (
	initialRetryInterval = 500 * time.Millisecond
	maxInterval          = 1 * time.Minute
	maxElapsedTime       = 15 * time.Minute
	randomizationFactor  = 0.5
	multiplier           = 1.5
)

// Backoff 提供了确定在重试操作之前等待的时间算法
type Backoff struct {
	interval    time.Duration
	elapsedTime time.Duration
}

// Pause 返回重试操作之前等待的时间量，如果可以再次尝试则返回 true，否则返回 false，表示操作应该被放弃。
func (b *Backoff) Pause() (time.Duration, bool) {
	if b.interval == 0 {
		// first time
		b.interval = initialRetryInterval
		b.elapsedTime = 0
	}

	// interval from [1 - randomizationFactor, 1 + randomizationFactor)
	randomizedInterval := time.Duration((rand.Float64()*(2*randomizationFactor) + (1 - randomizationFactor)) * float64(b.interval))
	b.elapsedTime += randomizedInterval

	if b.elapsedTime > maxElapsedTime {
		return 0, false
	}

	// 将间隔增加到间隔上限
	b.interval = time.Duration(float64(b.interval) * multiplier)
	if b.interval > maxInterval {
		b.interval = maxInterval
	}

	return randomizedInterval, true
}
