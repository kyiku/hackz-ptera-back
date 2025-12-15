package fish

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFishDataset_GetRandom(t *testing.T) {
	tests := []struct {
		name         string
		wantNonEmpty bool
		wantFilename bool
	}{
		{
			name:         "正常系: ランダム魚取得",
			wantNonEmpty: true,
			wantFilename: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dataset := NewDataset()

			fish, err := dataset.GetRandom()

			require.NoError(t, err)
			if tt.wantNonEmpty {
				assert.NotEmpty(t, fish.Name)
			}
			if tt.wantFilename {
				assert.NotEmpty(t, fish.Filename)
				assert.Contains(t, fish.Filename, ".jpg")
			}
		})
	}
}

func TestFishDataset_GetRandomExcluding(t *testing.T) {
	tests := []struct {
		name       string
		excludeNum int
		wantErr    bool
	}{
		{
			name:       "正常系: 1つ除外",
			excludeNum: 1,
			wantErr:    false,
		},
		{
			name:       "正常系: 5つ除外",
			excludeNum: 5,
			wantErr:    false,
		},
		{
			name:       "正常系: 10つ除外",
			excludeNum: 10,
			wantErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dataset := NewDataset()

			// 除外リストを作成
			excluded := []string{}
			for i := 0; i < tt.excludeNum; i++ {
				fish, err := dataset.GetRandomExcluding(excluded)
				require.NoError(t, err)
				excluded = append(excluded, fish.Name)
			}

			// 除外リストを使って新しい魚を取得
			fish, err := dataset.GetRandomExcluding(excluded)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			// 除外リストに含まれていないことを確認
			assert.NotContains(t, excluded, fish.Name)
		})
	}
}

func TestFishDataset_AllFishDifferent(t *testing.T) {
	dataset := NewDataset()

	usedFish := []string{}

	// 3回連続で異なる魚が取得できることを確認（OTPの再試行シナリオ）
	for i := 0; i < 3; i++ {
		fish, err := dataset.GetRandomExcluding(usedFish)
		require.NoError(t, err)
		assert.NotContains(t, usedFish, fish.Name, "同じ魚が選ばれてはいけない")
		usedFish = append(usedFish, fish.Name)
	}

	// 3つすべて異なることを確認
	assert.Len(t, usedFish, 3)
	unique := make(map[string]bool)
	for _, name := range usedFish {
		unique[name] = true
	}
	assert.Len(t, unique, 3, "3つの異なる魚が選ばれるべき")
}

func TestFishDataset_Count(t *testing.T) {
	dataset := NewDataset()

	count := dataset.Count()

	// 約20種の魚が登録されていることを確認
	assert.GreaterOrEqual(t, count, 15, "最低15種の魚が必要")
	assert.LessOrEqual(t, count, 25, "最大25種程度を想定")
}

func TestFishDataset_GetByName(t *testing.T) {
	tests := []struct {
		name     string
		fishName string
		wantErr  bool
	}{
		{
			name:     "正常系: 存在する魚",
			fishName: "オニカマス",
			wantErr:  false,
		},
		{
			name:     "異常系: 存在しない魚",
			fishName: "存在しない魚",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dataset := NewDataset()

			fish, err := dataset.GetByName(tt.fishName)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.fishName, fish.Name)
			assert.NotEmpty(t, fish.Filename)
		})
	}
}

func TestFishDataset_ListAll(t *testing.T) {
	dataset := NewDataset()

	fishList := dataset.ListAll()

	assert.NotEmpty(t, fishList)

	// 各魚が名前とファイル名を持っていることを確認
	for _, fish := range fishList {
		assert.NotEmpty(t, fish.Name, "魚名が空であってはいけない")
		assert.NotEmpty(t, fish.Filename, "ファイル名が空であってはいけない")
	}

	// 重複がないことを確認
	names := make(map[string]bool)
	for _, fish := range fishList {
		assert.False(t, names[fish.Name], "魚名が重複している: %s", fish.Name)
		names[fish.Name] = true
	}
}

func TestFishDataset_Randomness(t *testing.T) {
	dataset := NewDataset()

	// 複数回呼び出してランダム性を確認
	results := make(map[string]int)
	iterations := 100

	for i := 0; i < iterations; i++ {
		fish, err := dataset.GetRandom()
		require.NoError(t, err)
		results[fish.Name]++
	}

	// 複数の異なる魚が選ばれることを期待
	assert.Greater(t, len(results), 5, "100回の試行で5種類以上の魚が選ばれるべき")
}
