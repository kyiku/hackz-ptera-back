package util

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestKanaMatch(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		answer    string
		wantMatch bool
	}{
		// 基本的な一致
		{name: "カタカナ完全一致", input: "オニカマス", answer: "オニカマス", wantMatch: true},
		{name: "ひらがな完全一致", input: "おにかます", answer: "おにかます", wantMatch: true},

		// ひらがな/カタカナ相互変換
		{name: "ひらがな入力、カタカナ正解", input: "おにかます", answer: "オニカマス", wantMatch: true},
		{name: "カタカナ入力、ひらがな正解", input: "オニカマス", answer: "おにかます", wantMatch: true},

		// 混合
		{name: "混合入力1", input: "おにカマス", answer: "オニカマス", wantMatch: true},
		{name: "混合入力2", input: "オニかます", answer: "おにかます", wantMatch: true},
		{name: "混合正解に混合入力", input: "おにカマス", answer: "オニかます", wantMatch: true},

		// 不一致
		{name: "異なる文字列", input: "サバ", answer: "オニカマス", wantMatch: false},
		{name: "部分一致", input: "オニカ", answer: "オニカマス", wantMatch: false},
		{name: "余分な文字", input: "オニカマスです", answer: "オニカマス", wantMatch: false},

		// エッジケース
		{name: "空文字列入力", input: "", answer: "オニカマス", wantMatch: false},
		{name: "空文字列正解", input: "オニカマス", answer: "", wantMatch: false},
		{name: "両方空", input: "", answer: "", wantMatch: true},
		{name: "前後の空白", input: "オニカマス ", answer: "オニカマス", wantMatch: true},
		{name: "前の空白", input: " オニカマス", answer: "オニカマス", wantMatch: true},

		// 特殊文字
		{name: "長音記号", input: "ホーボー", answer: "ホウボウ", wantMatch: false},
		{name: "小文字カナ", input: "ャ", answer: "ヤ", wantMatch: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := KanaMatch(tt.input, tt.answer)
			assert.Equal(t, tt.wantMatch, result)
		})
	}
}

func TestHiraganaToKatakana(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "すべてひらがな", input: "おにかます", want: "オニカマス"},
		{name: "すべてカタカナ（変換なし）", input: "オニカマス", want: "オニカマス"},
		{name: "混合", input: "おにカマス", want: "オニカマス"},
		{name: "空文字列", input: "", want: ""},
		{name: "漢字含む", input: "お魚", want: "オ魚"},
		{name: "数字含む", input: "さば123", want: "サバ123"},
		{name: "小文字ひらがな", input: "ぁぃぅぇぉ", want: "ァィゥェォ"},
		{name: "濁音", input: "がぎぐげご", want: "ガギグゲゴ"},
		{name: "半濁音", input: "ぱぴぷぺぽ", want: "パピプペポ"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := HiraganaToKatakana(tt.input)
			assert.Equal(t, tt.want, result)
		})
	}
}

func TestKatakanaToHiragana(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "すべてカタカナ", input: "オニカマス", want: "おにかます"},
		{name: "すべてひらがな（変換なし）", input: "おにかます", want: "おにかます"},
		{name: "混合", input: "オニかます", want: "おにかます"},
		{name: "空文字列", input: "", want: ""},
		{name: "漢字含む", input: "オ魚", want: "お魚"},
		{name: "数字含む", input: "サバ123", want: "さば123"},
		{name: "小文字カタカナ", input: "ァィゥェォ", want: "ぁぃぅぇぉ"},
		{name: "濁音", input: "ガギグゲゴ", want: "がぎぐげご"},
		{name: "半濁音", input: "パピプペポ", want: "ぱぴぷぺぽ"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := KatakanaToHiragana(tt.input)
			assert.Equal(t, tt.want, result)
		})
	}
}

func TestNormalizeForComparison(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "ひらがな正規化", input: "おにかます", want: "オニカマス"},
		{name: "カタカナ正規化", input: "オニカマス", want: "オニカマス"},
		{name: "混合正規化", input: "おにカマス", want: "オニカマス"},
		{name: "前後空白除去", input: " オニカマス ", want: "オニカマス"},
		{name: "全角スペース", input: "オニカマス　", want: "オニカマス"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NormalizeForComparison(tt.input)
			assert.Equal(t, tt.want, result)
		})
	}
}

func TestKanaMatch_RealFishNames(t *testing.T) {
	// 実際の魚名でテスト
	fishNames := []struct {
		katakana string
		hiragana string
	}{
		{"オニカマス", "おにかます"},
		{"ホウボウ", "ほうぼう"},
		{"マツカサウオ", "まつかさうお"},
		{"ハリセンボン", "はりせんぼん"},
		{"カワハギ", "かわはぎ"},
		{"フグ", "ふぐ"},
	}

	for _, fish := range fishNames {
		t.Run(fish.katakana, func(t *testing.T) {
			// カタカナ正解にひらがな入力
			assert.True(t, KanaMatch(fish.hiragana, fish.katakana))
			// ひらがな正解にカタカナ入力
			assert.True(t, KanaMatch(fish.katakana, fish.hiragana))
			// 同じ形式
			assert.True(t, KanaMatch(fish.katakana, fish.katakana))
			assert.True(t, KanaMatch(fish.hiragana, fish.hiragana))
		})
	}
}
