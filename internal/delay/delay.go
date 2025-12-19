// Package delay provides random delay generation and execution.
package delay

import (
	"math/rand"
	"sync"
	"time"
)

// DelayGenerator generates random delays within a specified range.
type DelayGenerator struct {
	minSec int
	maxSec int
}

// NewDelayGenerator creates a new DelayGenerator with the specified range.
func NewDelayGenerator(minSec, maxSec int) *DelayGenerator {
	return &DelayGenerator{
		minSec: minSec,
		maxSec: maxSec,
	}
}

// NewDefaultDelayGenerator creates a DelayGenerator with default settings (10-30 seconds).
func NewDefaultDelayGenerator() *DelayGenerator {
	return NewDelayGenerator(10, 30)
}

// Generate generates a random delay duration within the configured range.
func (g *DelayGenerator) Generate() time.Duration {
	if g.minSec == g.maxSec {
		return time.Duration(g.minSec) * time.Second
	}
	rangeSize := g.maxSec - g.minSec + 1
	randomSec := g.minSec + rand.Intn(rangeSize)
	return time.Duration(randomSec) * time.Second
}

// DelayExecutor executes delays with optional callbacks.
type DelayExecutor struct {
	mu       sync.Mutex
	canceled bool
	timer    *time.Timer
}

// NewDelayExecutor creates a new DelayExecutor.
func NewDelayExecutor() *DelayExecutor {
	return &DelayExecutor{}
}

// Execute waits for the specified duration.
func (e *DelayExecutor) Execute(delay time.Duration) {
	e.mu.Lock()
	e.canceled = false
	e.mu.Unlock()

	if delay <= 0 {
		return
	}

	time.Sleep(delay)
}

// ExecuteWithCallback waits for the specified duration and then calls the callback.
// The callback is not called if Cancel is called before the delay completes.
func (e *DelayExecutor) ExecuteWithCallback(delay time.Duration, callback func()) {
	e.mu.Lock()
	e.canceled = false
	e.timer = time.NewTimer(delay)
	e.mu.Unlock()

	<-e.timer.C

	e.mu.Lock()
	wasCanceled := e.canceled
	e.mu.Unlock()

	if !wasCanceled && callback != nil {
		callback()
	}
}

// Cancel cancels any pending delay execution.
func (e *DelayExecutor) Cancel() {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.canceled = true
	if e.timer != nil {
		e.timer.Stop()
	}
}
