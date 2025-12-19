package ai

import (
	"errors"
	"testing"

	"hackz-ptera/back/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBedrockClient_AnalyzePassword(t *testing.T) {
	tests := []struct {
		name         string
		password     string
		mockResponse string
		mockErr      error
		wantContains string
		wantErr      bool
	}{
		{
			name:         "正常系: 名前と年を含むパスワード",
			password:     "taro1998",
			mockResponse: `{"content":[{"text":"太郎さんですか？1998年生まれ？"}]}`,
			mockErr:      nil,
			wantContains: "太郎",
			wantErr:      false,
		},
		{
			name:         "正常系: 弱いパスワード",
			password:     "password123",
			mockResponse: `{"content":[{"text":"これは非常に弱いパスワードです。推測されやすいです。"}]}`,
			mockErr:      nil,
			wantContains: "弱いパスワード",
			wantErr:      false,
		},
		{
			name:         "正常系: 強いパスワード",
			password:     "Xy$9kL#mP2qR",
			mockResponse: `{"content":[{"text":"なかなか強そうですね...でも何か意味があるのでは？"}]}`,
			mockErr:      nil,
			wantContains: "強そう",
			wantErr:      false,
		},
		{
			name:         "異常系: Bedrock APIエラー",
			password:     "test",
			mockResponse: "",
			mockErr:      errors.New("Bedrock API error"),
			wantContains: "",
			wantErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockBedrock := testutil.NewMockBedrockClient()
			mockBedrock.Response = tt.mockResponse
			mockBedrock.Err = tt.mockErr

			client := NewBedrockClient(mockBedrock, "ap-northeast-1")

			result, err := client.AnalyzePassword(tt.password)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Contains(t, result, tt.wantContains)
		})
	}
}

func TestBedrockClient_Prompt(t *testing.T) {
	tests := []struct {
		name         string
		password     string
		wantInPrompt []string
	}{
		{
			name:         "パスワードがプロンプトに含まれる",
			password:     "test123",
			wantInPrompt: []string{"test123", "パスワード"},
		},
		{
			name:         "日本語パスワード",
			password:     "たろう1998",
			wantInPrompt: []string{"たろう1998"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockBedrock := testutil.NewMockBedrockClient()
			mockBedrock.Response = `{"content":[{"text":"分析結果"}]}`

			client := NewBedrockClient(mockBedrock, "ap-northeast-1")
			_, _ = client.AnalyzePassword(tt.password)

			for _, want := range tt.wantInPrompt {
				assert.Contains(t, mockBedrock.LastPrompt, want)
			}
		})
	}
}

func TestBedrockClient_ModelID(t *testing.T) {
	mockBedrock := testutil.NewMockBedrockClient()
	mockBedrock.Response = `{"content":[{"text":"結果"}]}`

	client := NewBedrockClient(mockBedrock, "ap-northeast-1")
	_, _ = client.AnalyzePassword("test")

	// Claude 3 Haikuモデルが使用されていることを確認
	assert.Contains(t, mockBedrock.LastModelID, "claude-3-haiku")
}

func TestBedrockClient_ParseResponse(t *testing.T) {
	tests := []struct {
		name         string
		response     string
		wantContains string
		wantErr      bool
	}{
		{
			name:         "正常なレスポンス",
			response:     `{"content":[{"text":"弱いパスワードです"}]}`,
			wantContains: "弱いパスワード",
			wantErr:      false,
		},
		{
			name:         "複数テキストブロック",
			response:     `{"content":[{"text":"最初の"}, {"text":"テキスト"}]}`,
			wantContains: "最初の",
			wantErr:      false,
		},
		{
			name:         "不正なJSON",
			response:     `{invalid json}`,
			wantContains: "",
			wantErr:      true,
		},
		{
			name:         "空のcontent",
			response:     `{"content":[]}`,
			wantContains: "",
			wantErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockBedrock := testutil.NewMockBedrockClient()
			mockBedrock.Response = tt.response

			client := NewBedrockClient(mockBedrock, "ap-northeast-1")
			result, err := client.AnalyzePassword("test")

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Contains(t, result, tt.wantContains)
		})
	}
}

func TestBedrockClient_Fallback(t *testing.T) {
	// Bedrockがエラーを返した場合のフォールバック処理をテスト
	mockBedrock := testutil.NewMockBedrockClient()
	mockBedrock.Err = errors.New("API unavailable")

	client := NewBedrockClient(mockBedrock, "ap-northeast-1")
	client.EnableFallback(true)

	result, err := client.AnalyzePassword("test")

	// フォールバックが有効な場合はエラーにならない
	require.NoError(t, err)
	assert.NotEmpty(t, result, "フォールバックメッセージが返されるべき")
}
