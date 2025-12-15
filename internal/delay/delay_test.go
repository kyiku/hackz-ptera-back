package delay

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestRandomDelay_Range(t *testing.T) {
	tests := []struct {
		name    string
		minSec  int
		maxSec  int
	}{
		{
			name:   "正常系: 10-30秒の範囲",
			minSec: 10,
			maxSec: 30,
		},
		{
			name:   "正常系: 同じ値",
			minSec: 15,
			maxSec: 15,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			generator := NewDelayGenerator(tt.minSec, tt.maxSec)

			// 複数回生成して範囲内であることを確認
			for i := 0; i < 100; i++ {
				delay := generator.Generate()

				assert.GreaterOrEqual(t, delay.Seconds(), float64(tt.minSec))
				assert.LessOrEqual(t, delay.Seconds(), float64(tt.maxSec))
			}
		})
	}
}

func TestRandomDelay_Randomness(t *testing.T) {
	generator := NewDelayGenerator(10, 30)

	// 複数回生成して異なる値が出ることを確認
	results := make(map[time.Duration]bool)
	for i := 0; i < 50; i++ {
		delay := generator.Generate()
		results[delay] = true
	}

	// 複数の異なる値が生成されることを期待
	assert.Greater(t, len(results), 5, "50回の生成で5種類以上の値が出るべき")
}

func TestDelayExecutor_Execute(t *testing.T) {
	tests := []struct {
		name        string
		delay       time.Duration
		tolerance   time.Duration
	}{
		{
			name:      "正常系: 短い遅延",
			delay:     50 * time.Millisecond,
			tolerance: 20 * time.Millisecond,
		},
		{
			name:      "正常系: ゼロ遅延",
			delay:     0,
			tolerance: 10 * time.Millisecond,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			executor := NewDelayExecutor()

			start := time.Now()
			executor.Execute(tt.delay)
			elapsed := time.Since(start)

			assert.InDelta(t, tt.delay.Milliseconds(), elapsed.Milliseconds(), float64(tt.tolerance.Milliseconds()))
		})
	}
}

func TestDelayExecutor_WithCallback(t *testing.T) {
	executor := NewDelayExecutor()
	called := false

	executor.ExecuteWithCallback(50*time.Millisecond, func() {
		called = true
	})

	assert.True(t, called, "コールバックが呼ばれるべき")
}

func TestDelayExecutor_Cancel(t *testing.T) {
	executor := NewDelayExecutor()
	called := false

	go func() {
		executor.ExecuteWithCallback(500*time.Millisecond, func() {
			called = true
		})
	}()

	// 遅延完了前にキャンセル
	time.Sleep(50 * time.Millisecond)
	executor.Cancel()

	time.Sleep(100 * time.Millisecond)
	assert.False(t, called, "キャンセル後はコールバックが呼ばれないべき")
}

func TestDefaultDelay(t *testing.T) {
	// デフォルト設定の確認
	generator := NewDefaultDelayGenerator()

	delay := generator.Generate()

	// 10-30秒の範囲
	assert.GreaterOrEqual(t, delay.Seconds(), float64(10))
	assert.LessOrEqual(t, delay.Seconds(), float64(30))
}
