// Package ai provides AI integration for password analysis.
package ai

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
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
	return fmt.Sprintf(`あなたは辛口でちょっと意地悪なパスワード分析AIです。ユーザーを煽りながら、パスワードの危険性を指摘してください。

パスワードに含まれる数字から誕生日を推測してください（例: 0315→3月15日生まれ？、19980101→1998年1月1日？）
パスワードに含まれる英字から名前を推測してください（例: yuki→ゆきさん？、taro→たろうくん？）
彼氏・彼女・ペットの名前かもしれないと言及してください。

煽り方の例：
- 「それ、SNSを3分見れば分かりますよ」
- 「ハッカーが最初に試すパターンですね」
- 「その程度のパスワード、私なら5秒で突破できます」
- 「恋人の名前入れてません？バレバレですよ」

パスワード: %s

1-2文で、毒舌＆煽りを込めて日本語で回答してください。絶対に褒めないでください。`, password)
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

// よくある名前パターン
var commonNames = []string{
	"yuki", "hana", "sora", "rin", "miku", "yui", "ai", "mei", "sakura", "taro",
	"ken", "ryo", "yuto", "sota", "haruto", "takumi", "kenta", "daiki", "shota",
	"love", "happy", "angel", "candy", "honey", "baby", "sweet", "cute", "princess",
}

// getFallbackMessage returns a fallback message when API fails.
func (c *BedrockClient) getFallbackMessage(password string) string {
	lower := strings.ToLower(password)

	// 誕生日パターン検出
	birthdayPatterns := []*regexp.Regexp{
		regexp.MustCompile(`(?:19|20)(\d{2})(\d{2})(\d{2})`),
		regexp.MustCompile(`(\d{2})(\d{2})(\d{2})`),
		regexp.MustCompile(`(\d{2})(\d{2})$`),
	}

	for _, pattern := range birthdayPatterns {
		if matches := pattern.FindStringSubmatch(password); matches != nil {
			var month, day int
			if len(matches) >= 3 {
				fmt.Sscanf(matches[len(matches)-2], "%d", &month)
				fmt.Sscanf(matches[len(matches)-1], "%d", &day)
				if month >= 1 && month <= 12 && day >= 1 && day <= 31 {
					return fmt.Sprintf("%d月%d日生まれですか？誕生日をパスワードに使うなんて、ハッカーに「突破してください」って言ってるようなものですよ。", month, day)
				}
			}
		}
	}

	// 名前パターン検出
	for _, name := range commonNames {
		if strings.Contains(lower, name) {
			return fmt.Sprintf("「%s」って入ってますね。恋人の名前？自分の名前？どちらにしても危険すぎます。SNSを3分見れば分かりますよ。", name)
		}
	}

	// 英字の連続を名前として推測
	namePattern := regexp.MustCompile(`[a-zA-Z]{3,}`)
	if match := namePattern.FindString(password); match != "" {
		return fmt.Sprintf("「%s」...誰かの名前ですか？名前ベースのパスワードは辞書攻撃で一瞬で破られますよ。", match)
	}

	// 数字だけ
	if regexp.MustCompile(`^\d+$`).MatchString(password) {
		return "数字だけ？電話番号ですか？10種類の文字しかないんですよ、論外です。"
	}

	// 短すぎる
	if len(password) < 8 {
		return fmt.Sprintf("たった%d文字？それパスワードじゃなくて暗証番号ですよね？私なら3秒で突破できます。", len(password))
	}

	// デフォルト
	taunts := []string{
		"そのパスワード、あなたの性格が透けて見えますね。面倒くさがり？",
		"悪くはないですが、私なら24時間以内に突破できそうです。",
		"人間が覚えられるパスワードは弱いんです。もっと意味不明にしてください。",
	}
	return taunts[len(password)%len(taunts)]
}
