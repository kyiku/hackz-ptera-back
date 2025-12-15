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
				"backgrounds/bg1.png": testutil.CreateTestPNG(tt.bgWidth, tt.bgHeight),
				"character/char.png":  testutil.CreateTestPNG(tt.charWidth, tt.charHeight),
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
		"backgrounds/bg1.png": testutil.CreateTestPNG(1024, 768),
		"character/char.png":  testutil.CreateTestPNG(8, 8),
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
		"backgrounds/bg1.png": testutil.CreateTestPNG(1024, 768),
		"character/char.png":  testutil.CreateTestPNG(8, 8),
	}

	gen := NewGenerator(mockS3, "https://test.cloudfront.net")

	img := image.NewRGBA(image.Rect(0, 0, 1024, 768))
	url, err := gen.Upload(img)

	require.NoError(t, err)
	assert.Contains(t, url, "https://test.cloudfront.net/captcha/")
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
			"backgrounds/bg1.png": testutil.CreateTestPNG(1024, 768),
			"character/char.png":  testutil.CreateTestPNG(size, size),
		}

		gen := NewGenerator(mockS3, "https://test.cloudfront.net")
		_, _, _, err := gen.Generate()

		assert.NoError(t, err, "サイズ%dのキャラクターで生成できるべき", size)
	}
}
