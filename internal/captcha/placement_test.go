package captcha

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPlacement_Intersects(t *testing.T) {
	tests := []struct {
		name string
		p1   Placement
		p2   Placement
		want bool
	}{
		{
			name: "重ならない: 離れている",
			p1:   Placement{X: 0, Y: 0, Width: 10, Height: 10},
			p2:   Placement{X: 20, Y: 20, Width: 10, Height: 10},
			want: false,
		},
		{
			name: "重ならない: 隣接（右）",
			p1:   Placement{X: 0, Y: 0, Width: 10, Height: 10},
			p2:   Placement{X: 10, Y: 0, Width: 10, Height: 10},
			want: false,
		},
		{
			name: "重ならない: 隣接（下）",
			p1:   Placement{X: 0, Y: 0, Width: 10, Height: 10},
			p2:   Placement{X: 0, Y: 10, Width: 10, Height: 10},
			want: false,
		},
		{
			name: "重なる: 部分的に重複",
			p1:   Placement{X: 0, Y: 0, Width: 10, Height: 10},
			p2:   Placement{X: 5, Y: 5, Width: 10, Height: 10},
			want: true,
		},
		{
			name: "重なる: 完全に含む",
			p1:   Placement{X: 0, Y: 0, Width: 100, Height: 100},
			p2:   Placement{X: 25, Y: 25, Width: 10, Height: 10},
			want: true,
		},
		{
			name: "重なる: 同じ位置",
			p1:   Placement{X: 50, Y: 50, Width: 10, Height: 10},
			p2:   Placement{X: 50, Y: 50, Width: 10, Height: 10},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.p1.Intersects(tt.p2)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestPlacementManager_TryPlace(t *testing.T) {
	t.Run("正常系: 配置成功", func(t *testing.T) {
		pm := NewPlacementManager(1000, 1000, 50, 50)

		placement, ok := pm.TryPlace()

		require.True(t, ok)
		assert.GreaterOrEqual(t, placement.X, 0)
		assert.Less(t, placement.X, 950) // 1000 - 50
		assert.GreaterOrEqual(t, placement.Y, 0)
		assert.Less(t, placement.Y, 950)
		assert.Equal(t, 50, placement.Width)
		assert.Equal(t, 50, placement.Height)
		assert.Equal(t, 1, pm.PlacedCount())
	})

	t.Run("正常系: 複数配置が重ならない", func(t *testing.T) {
		pm := NewPlacementManager(1000, 1000, 50, 50)

		placements := make([]Placement, 0)
		for i := 0; i < 20; i++ {
			p, ok := pm.TryPlace()
			if ok {
				placements = append(placements, p)
			}
		}

		// 配置されたものが重なっていないことを確認
		for i := 0; i < len(placements); i++ {
			for j := i + 1; j < len(placements); j++ {
				assert.False(t, placements[i].Intersects(placements[j]),
					"配置 %d と %d が重なっている", i, j)
			}
		}
	})

	t.Run("正常系: 91枚配置可能", func(t *testing.T) {
		// 2816x1536 の背景に 50x50 のキャラクター
		pm := NewPlacementManager(2816, 1536, 50, 50)

		successCount := 0
		for i := 0; i < 91; i++ {
			_, ok := pm.TryPlace()
			if ok {
				successCount++
			}
		}

		// 91枚すべて配置できるはず
		assert.Equal(t, 91, successCount)
	})

	t.Run("異常系: 背景が小さすぎる", func(t *testing.T) {
		pm := NewPlacementManager(10, 10, 50, 50)

		_, ok := pm.TryPlace()

		assert.False(t, ok)
	})
}

func TestPlacementManager_Reset(t *testing.T) {
	pm := NewPlacementManager(1000, 1000, 50, 50)

	// 配置を追加
	pm.TryPlace()
	pm.TryPlace()
	assert.Equal(t, 2, pm.PlacedCount())

	// リセット
	pm.Reset()
	assert.Equal(t, 0, pm.PlacedCount())
}
