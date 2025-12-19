// Package ai provides AI integration for password analysis.
package ai

import (
	"encoding/json"
	"errors"
	"fmt"
)

// BedrockClientInterface defines the interface for Bedrock client.
type BedrockClientInterface interface {
	InvokeModel(modelID string, prompt string) (string, error)
}

// BedrockClient wraps the Bedrock client for password analysis.
type BedrockClient struct {
	client          BedrockClientInterface
	region          string
	fallbackEnabled bool
}

// ClaudeResponse represents the response from Claude.
type ClaudeResponse struct {
	Content []ContentBlock `json:"content"`
}

// ContentBlock represents a content block in Claude's response.
type ContentBlock struct {
	Text string `json:"text"`
}

// Claude 3 Haiku model ID
const claudeHaikuModelID = "anthropic.claude-3-haiku-20240307-v1:0"

// NewBedrockClient creates a new BedrockClient.
func NewBedrockClient(client BedrockClientInterface, region string) *BedrockClient {
	return &BedrockClient{
		client:          client,
		region:          region,
		fallbackEnabled: false,
	}
}

// EnableFallback enables or disables fallback mode.
// When enabled, returns a fallback message instead of error when API fails.
func (c *BedrockClient) EnableFallback(enabled bool) {
	c.fallbackEnabled = enabled
}

// AnalyzePassword sends the password to Claude for analysis.
func (c *BedrockClient) AnalyzePassword(password string) (string, error) {
	prompt := c.buildPrompt(password)

	response, err := c.client.InvokeModel(claudeHaikuModelID, prompt)
	if err != nil {
		if c.fallbackEnabled {
			return c.getFallbackMessage(password), nil
		}
		return "", fmt.Errorf("failed to invoke Bedrock: %w", err)
	}

	result, err := c.parseResponse(response)
	if err != nil {
		if c.fallbackEnabled {
			return c.getFallbackMessage(password), nil
		}
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	return result, nil
}

// buildPrompt creates the prompt for password analysis.
func (c *BedrockClient) buildPrompt(password string) string {
	return fmt.Sprintf(`あなたはパスワードを分析する皮肉っぽいAIアシスタントです。
ユーザーが入力したパスワードを見て、そのパスワードから推測できることを指摘してください。
名前や生年月日、ペットの名前など、個人情報が含まれていそうな場合は指摘してください。
弱いパスワードの場合は「弱いパスワード」という言葉を使って批評してください。
強そうなパスワードでも何か意味がありそうなら推測してください。

パスワード: %s

短く（1-2文で）日本語で回答してください。`, password)
}

// parseResponse parses the Claude response JSON.
func (c *BedrockClient) parseResponse(response string) (string, error) {
	var claudeResp ClaudeResponse
	if err := json.Unmarshal([]byte(response), &claudeResp); err != nil {
		return "", err
	}

	if len(claudeResp.Content) == 0 {
		return "", errors.New("empty content in response")
	}

	return claudeResp.Content[0].Text, nil
}

// getFallbackMessage returns a fallback message when API fails.
func (c *BedrockClient) getFallbackMessage(password string) string {
	// Simple fallback analysis
	if len(password) < 8 {
		return "短すぎるパスワードです。もう少し長くしてください。"
	}

	hasDigit := false
	hasLetter := false
	for _, r := range password {
		if r >= '0' && r <= '9' {
			hasDigit = true
		}
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
			hasLetter = true
		}
	}

	if !hasDigit || !hasLetter {
		return "数字と文字を組み合わせてください。"
	}

	return "まあまあのパスワードですね。"
}
