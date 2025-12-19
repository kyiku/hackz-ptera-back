package storage

import (
	"image"
	"testing"

	"hackz-ptera/back/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestS3Client_GetBackgroundImage(t *testing.T) {
	tests := []struct {
		name       string
		setupMock  func(*testutil.MockS3Client)
		wantErr    bool
		wantWidth  int
		wantHeight int
	}{
		{
			name: "正常系: 背景画像取得",
			setupMock: func(m *testutil.MockS3Client) {
				m.Objects = map[string][]byte{
					"backgrounds/bg1.png": testutil.CreateTestPNG(1024, 768),
					"backgrounds/bg2.png": testutil.CreateTestPNG(1024, 768),
				}
			},
			wantErr:    false,
			wantWidth:  1024,
			wantHeight: 768,
		},
		{
			name: "異常系: 背景画像が存在しない",
			setupMock: func(m *testutil.MockS3Client) {
				m.Objects = map[string][]byte{}
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockS3 := testutil.NewMockS3Client()
			tt.setupMock(mockS3)

			client := NewS3Client(mockS3, "test-bucket", "https://test.cloudfront.net")

			img, err := client.GetRandomBackgroundImage()

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.NotNil(t, img)
			assert.Equal(t, tt.wantWidth, img.Bounds().Dx())
			assert.Equal(t, tt.wantHeight, img.Bounds().Dy())
		})
	}
}

func TestS3Client_GetCharacterImage(t *testing.T) {
	tests := []struct {
		name        string
		setupMock   func(*testutil.MockS3Client)
		wantErr     bool
		wantMinSize int
		wantMaxSize int
	}{
		{
			name: "正常系: キャラクター画像取得",
			setupMock: func(m *testutil.MockS3Client) {
				m.Objects = map[string][]byte{
					"character/char.png": testutil.CreateTestPNG(8, 8),
				}
			},
			wantErr:     false,
			wantMinSize: 5,
			wantMaxSize: 8,
		},
		{
			name: "異常系: キャラクター画像が存在しない",
			setupMock: func(m *testutil.MockS3Client) {
				m.Objects = map[string][]byte{}
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockS3 := testutil.NewMockS3Client()
			tt.setupMock(mockS3)

			client := NewS3Client(mockS3, "test-bucket", "https://test.cloudfront.net")

			img, err := client.GetCharacterImage()

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.NotNil(t, img)
			// キャラクターサイズが5-8pxであることを確認
			assert.LessOrEqual(t, img.Bounds().Dx(), tt.wantMaxSize)
			assert.GreaterOrEqual(t, img.Bounds().Dx(), tt.wantMinSize)
		})
	}
}

func TestS3Client_GetFishImage(t *testing.T) {
	tests := []struct {
		name      string
		fishName  string
		setupMock func(*testutil.MockS3Client)
		wantErr   bool
		wantURL   string
	}{
		{
			name:     "正常系: 魚画像URL取得",
			fishName: "onikamasu",
			setupMock: func(m *testutil.MockS3Client) {
				m.Objects = map[string][]byte{
					"fish/onikamasu.jpg": testutil.CreateTestJPEG(400, 300),
				}
			},
			wantErr: false,
			wantURL: "https://test.cloudfront.net/fish/onikamasu.jpg",
		},
		{
			name:     "正常系: 別の魚画像",
			fishName: "houhou",
			setupMock: func(m *testutil.MockS3Client) {
				m.Objects = map[string][]byte{
					"fish/houhou.jpg": testutil.CreateTestJPEG(400, 300),
				}
			},
			wantErr: false,
			wantURL: "https://test.cloudfront.net/fish/houhou.jpg",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockS3 := testutil.NewMockS3Client()
			tt.setupMock(mockS3)

			client := NewS3Client(mockS3, "test-bucket", "https://test.cloudfront.net")

			url, err := client.GetFishImageURL(tt.fishName)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.wantURL, url)
		})
	}
}

func TestS3Client_UploadCaptchaImage(t *testing.T) {
	tests := []struct {
		name        string
		imageWidth  int
		imageHeight int
		setupMock   func(*testutil.MockS3Client)
		wantErr     bool
		wantURLPre  string
	}{
		{
			name:        "正常系: CAPTCHA画像アップロード",
			imageWidth:  1024,
			imageHeight: 768,
			setupMock: func(m *testutil.MockS3Client) {
				// アップロード成功
			},
			wantErr:    false,
			wantURLPre: "https://test.cloudfront.net/captcha/",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockS3 := testutil.NewMockS3Client()
			tt.setupMock(mockS3)

			client := NewS3Client(mockS3, "test-bucket", "https://test.cloudfront.net")

			testImg := image.NewRGBA(image.Rect(0, 0, tt.imageWidth, tt.imageHeight))

			url, err := client.UploadCaptchaImage(testImg)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Contains(t, url, tt.wantURLPre)
			assert.Contains(t, url, ".png")

			// アップロードされたデータが存在することを確認
			assert.Greater(t, len(mockS3.UploadedData), 0, "画像データがアップロードされているべき")
		})
	}
}

func TestS3Client_ListFishImages(t *testing.T) {
	mockS3 := testutil.NewMockS3Client()
	mockS3.Objects = map[string][]byte{
		"fish/onikamasu.jpg":   testutil.CreateTestJPEG(400, 300),
		"fish/houhou.jpg":      testutil.CreateTestJPEG(400, 300),
		"fish/matsukasauo.jpg": testutil.CreateTestJPEG(400, 300),
	}

	client := NewS3Client(mockS3, "test-bucket", "https://test.cloudfront.net")

	fishNames, err := client.ListFishImages()

	require.NoError(t, err)
	assert.Len(t, fishNames, 3)
	assert.Contains(t, fishNames, "onikamasu")
	assert.Contains(t, fishNames, "houhou")
	assert.Contains(t, fishNames, "matsukasauo")
}

func TestS3Client_RandomBackground(t *testing.T) {
	mockS3 := testutil.NewMockS3Client()
	mockS3.Objects = map[string][]byte{
		"backgrounds/bg1.png": testutil.CreateTestPNG(1024, 768),
		"backgrounds/bg2.png": testutil.CreateTestPNG(1024, 768),
		"backgrounds/bg3.png": testutil.CreateTestPNG(1024, 768),
	}

	client := NewS3Client(mockS3, "test-bucket", "https://test.cloudfront.net")

	// 複数回呼び出してランダム性を確認
	for i := 0; i < 20; i++ {
		img, err := client.GetRandomBackgroundImage()
		require.NoError(t, err)
		assert.NotNil(t, img)
	}
}
