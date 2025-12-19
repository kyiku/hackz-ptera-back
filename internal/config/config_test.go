package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadConfig(t *testing.T) {
	tests := []struct {
		name     string
		envVars  map[string]string
		wantPort string
		wantErr  bool
	}{
		{
			name: "すべての環境変数が設定されている",
			envVars: map[string]string{
				"PORT":              "8080",
				"ALLOWED_ORIGIN":    "http://localhost:5173",
				"AWS_REGION":        "ap-northeast-1",
				"S3_BUCKET":         "test-bucket",
				"CLOUDFRONT_DOMAIN": "https://test.cloudfront.net",
			},
			wantPort: "8080",
			wantErr:  false,
		},
		{
			name:     "デフォルト値が使用される",
			envVars:  map[string]string{},
			wantPort: "8080", // デフォルト
			wantErr:  false,
		},
		{
			name: "PORTのみカスタム",
			envVars: map[string]string{
				"PORT": "3000",
			},
			wantPort: "3000",
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 既存の環境変数を保存
			savedEnv := make(map[string]string)
			for _, key := range []string{"PORT", "ALLOWED_ORIGIN", "AWS_REGION", "S3_BUCKET", "CLOUDFRONT_DOMAIN"} {
				savedEnv[key] = os.Getenv(key)
				os.Unsetenv(key)
			}

			// テスト用の環境変数を設定
			for k, v := range tt.envVars {
				os.Setenv(k, v)
			}

			// テスト後に環境変数を復元
			defer func() {
				for k, v := range savedEnv {
					if v != "" {
						os.Setenv(k, v)
					} else {
						os.Unsetenv(k)
					}
				}
			}()

			cfg, err := LoadConfig()

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.wantPort, cfg.Port)
		})
	}
}

func TestConfig_Defaults(t *testing.T) {
	// 既存の環境変数を保存
	savedEnv := make(map[string]string)
	for _, key := range []string{"PORT", "ALLOWED_ORIGIN", "AWS_REGION", "S3_BUCKET", "CLOUDFRONT_DOMAIN"} {
		savedEnv[key] = os.Getenv(key)
		os.Unsetenv(key)
	}

	// テスト後に環境変数を復元
	defer func() {
		for k, v := range savedEnv {
			if v != "" {
				os.Setenv(k, v)
			} else {
				os.Unsetenv(k)
			}
		}
	}()

	cfg, err := LoadConfig()
	require.NoError(t, err)

	// デフォルト値の確認
	assert.Equal(t, "8080", cfg.Port)
	assert.Equal(t, "ap-northeast-1", cfg.AWSRegion)
	assert.Equal(t, "http://localhost:5173", cfg.AllowedOrigin)
}

func TestConfig_Validation(t *testing.T) {
	tests := []struct {
		name    string
		config  *Config
		wantErr bool
	}{
		{
			name: "有効な設定",
			config: &Config{
				Port:          "8080",
				AllowedOrigin: "http://localhost:5173",
				AWSRegion:     "ap-northeast-1",
			},
			wantErr: false,
		},
		{
			name: "不正なポート（文字列）",
			config: &Config{
				Port: "invalid",
			},
			wantErr: true,
		},
		{
			name: "空のポート",
			config: &Config{
				Port: "",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
