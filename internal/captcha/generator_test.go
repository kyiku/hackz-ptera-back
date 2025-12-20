package captcha

import (
	"image"
	"testing"

	"github.com/kyiku/hackz-ptera-back/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCaptchaGenerator_Generate(t *testing.T) {
	tests := []struct {
		name           string
		bgWidth        int
		bgHeight       int
		charWidth      int
		charHeight     int
		wantErr        bool
		wantImgWidth   int
		wantImgHeight  int
	}{
		{
			name:          "正常系: 標準サイズ",
			bgWidth:       1024,
			bgHeight:      768,
			charWidth:     8,
			charHeight:    8,
			wantErr:       false,
			wantImgWidth:  1024,
			wantImgHeight: 768,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockS3 := testutil.NewMockS3Client()
			mockS3.Objects = map[string][]byte{
				"static/backgrounds/bg1.png": testutil.CreateTestPNG(tt.bgWidth, tt.bgHeight),
				"static/character/char.png":  testutil.CreateTestPNG(tt.charWidth, tt.charHeight),
			}

			gen := NewGenerator(mockS3, "https://test.cloudfront.net")

			img, targetX, targetY, err := gen.Generate()

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.NotNil(t, img)
			assert.Equal(t, tt.wantImgWidth, img.Bounds().Dx())
			assert.Equal(t, tt.wantImgHeight, img.Bounds().Dy())

			// ターゲット座標が画像範囲内
			assert.GreaterOrEqual(t, targetX, 0)
			assert.Less(t, targetX, tt.wantImgWidth)
			assert.GreaterOrEqual(t, targetY, 0)
			assert.Less(t, targetY, tt.wantImgHeight)
		})
	}
}

func TestCaptchaGenerator_RandomPosition(t *testing.T) {
	mockS3 := testutil.NewMockS3Client()
	mockS3.Objects = map[string][]byte{
		"static/backgrounds/bg1.png": testutil.CreateTestPNG(1024, 768),
		"static/character/char.png":  testutil.CreateTestPNG(8, 8),
	}

	gen := NewGenerator(mockS3, "https://test.cloudfront.net")

	positions := make(map[string]bool)
	for i := 0; i < 20; i++ {
		_, x, y, err := gen.Generate()
		require.NoError(t, err)

		key := string(rune(x)) + "," + string(rune(y))
		positions[key] = true
	}

	// 複数の異なる位置にキャラクターが配置されることを確認
	assert.Greater(t, len(positions), 5, "20回の生成で5種類以上の位置が出るべき")
}

func TestCaptchaGenerator_Upload(t *testing.T) {
	mockS3 := testutil.NewMockS3Client()
	mockS3.Objects = map[string][]byte{
		"static/backgrounds/bg1.png": testutil.CreateTestPNG(1024, 768),
		"static/character/char.png":  testutil.CreateTestPNG(8, 8),
	}

	gen := NewGenerator(mockS3, "https://test.cloudfront.net")

	img := image.NewRGBA(image.Rect(0, 0, 1024, 768))
	url, err := gen.Upload(img)

	require.NoError(t, err)
	assert.Contains(t, url, "https://test.cloudfront.net/static/captcha/")
	assert.Contains(t, url, ".png")

	// S3にアップロードされたことを確認
	assert.Greater(t, len(mockS3.UploadedData), 0)
}

func TestCaptchaGenerator_Compose(t *testing.T) {
	tests := []struct {
		name       string
		bgWidth    int
		bgHeight   int
		charWidth  int
		charHeight int
		charX      int
		charY      int
	}{
		{
			name:       "正常系: 中央に配置",
			bgWidth:    1024,
			bgHeight:   768,
			charWidth:  8,
			charHeight: 8,
			charX:      512,
			charY:      384,
		},
		{
			name:       "正常系: 左上に配置",
			bgWidth:    1024,
			bgHeight:   768,
			charWidth:  8,
			charHeight: 8,
			charX:      0,
			charY:      0,
		},
		{
			name:       "正常系: 右下に配置",
			bgWidth:    1024,
			bgHeight:   768,
			charWidth:  8,
			charHeight: 8,
			charX:      1016,
			charY:      760,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bg := testutil.CreateTestImage(tt.bgWidth, tt.bgHeight)
			char := testutil.CreateTestImage(tt.charWidth, tt.charHeight)

			gen := &Generator{}
			result := gen.Compose(bg, char, tt.charX, tt.charY)

			require.NotNil(t, result)
			assert.Equal(t, tt.bgWidth, result.Bounds().Dx())
			assert.Equal(t, tt.bgHeight, result.Bounds().Dy())
		})
	}
}

func TestCaptchaGenerator_CharacterSize(t *testing.T) {
	// キャラクターサイズが5-8pxであることを確認
	mockS3 := testutil.NewMockS3Client()

	for size := 5; size <= 8; size++ {
		mockS3.Objects = map[string][]byte{
			"static/backgrounds/bg1.png": testutil.CreateTestPNG(1024, 768),
			"static/character/char.png":  testutil.CreateTestPNG(size, size),
		}

		gen := NewGenerator(mockS3, "https://test.cloudfront.net")
		_, _, _, err := gen.Generate()

		assert.NoError(t, err, "サイズ%dのキャラクターで生成できるべき", size)
	}
}

func TestCaptchaGenerator_GenerateMultiCharacter(t *testing.T) {
	t.Run("正常系: 91枚配置", func(t *testing.T) {
		mockS3 := testutil.NewMockS3Client()
		mockS3.Objects = map[string][]byte{
			"static/backgrounds/bg1.png":  testutil.CreateTestPNG(2816, 1536),
			"static/character/char1.png":  testutil.CreateTestPNG(100, 100),
			"static/character/char2.png":  testutil.CreateTestPNG(100, 100),
			"static/character/char3.png":  testutil.CreateTestPNG(100, 100),
			"static/character/char4.png":  testutil.CreateTestPNG(100, 100),
		}

		gen := NewGenerator(mockS3, "https://test.cloudfront.net")
		result, err := gen.GenerateMultiCharacter()

		require.NoError(t, err)
		require.NotNil(t, result)

		// 画像サイズが背景と同じ
		assert.Equal(t, 2816, result.Image.Bounds().Dx())
		assert.Equal(t, 1536, result.Image.Bounds().Dy())

		// ターゲット座標が有効範囲内（中心座標）
		assert.GreaterOrEqual(t, result.TargetX, CharacterSize/2)
		assert.Less(t, result.TargetX, 2816-CharacterSize/2)
		assert.GreaterOrEqual(t, result.TargetY, CharacterSize/2)
		assert.Less(t, result.TargetY, 1536-CharacterSize/2)

		// ターゲットキーが設定されている
		assert.NotEmpty(t, result.TargetKey)
		assert.Contains(t, result.TargetKey, "static/character/")

		// サイズ情報が設定されている
		assert.Equal(t, CharacterSize, result.TargetWidth)
		assert.Equal(t, CharacterSize, result.TargetHeight)
	})

	t.Run("異常系: キャラクターが4種類未満", func(t *testing.T) {
		mockS3 := testutil.NewMockS3Client()
		mockS3.Objects = map[string][]byte{
			"static/backgrounds/bg1.png":  testutil.CreateTestPNG(1024, 768),
			"static/character/char1.png":  testutil.CreateTestPNG(50, 50),
			"static/character/char2.png":  testutil.CreateTestPNG(50, 50),
			"static/character/char3.png":  testutil.CreateTestPNG(50, 50),
			// 4つ目がない
		}

		gen := NewGenerator(mockS3, "https://test.cloudfront.net")
		result, err := gen.GenerateMultiCharacter()

		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "need at least 4 character types")
	})
}

func TestResizeImage(t *testing.T) {
	t.Run("正常系: 大きい画像を縮小", func(t *testing.T) {
		src := testutil.CreateTestImage(540, 462)

		result := resizeImage(src, 50, 50)

		assert.Equal(t, 50, result.Bounds().Dx())
		assert.Equal(t, 50, result.Bounds().Dy())
	})

	t.Run("正常系: 小さい画像を拡大", func(t *testing.T) {
		src := testutil.CreateTestImage(10, 10)

		result := resizeImage(src, 50, 50)

		assert.Equal(t, 50, result.Bounds().Dx())
		assert.Equal(t, 50, result.Bounds().Dy())
	})
}
