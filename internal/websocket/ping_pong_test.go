package websocket

import (
	"encoding/json"
	"testing"

	"hackz-ptera/back/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPingHandler_HandlePing(t *testing.T) {
	tests := []struct {
		name        string
		inputMsg    map[string]interface{}
		wantHandled bool
		wantPong    bool
	}{
		{
			name:        "pingメッセージを処理",
			inputMsg:    map[string]interface{}{"type": "ping"},
			wantHandled: true,
			wantPong:    true,
		},
		{
			name:        "ping以外のメッセージは無視",
			inputMsg:    map[string]interface{}{"type": "other"},
			wantHandled: false,
			wantPong:    false,
		},
		{
			name:        "typeフィールドがないメッセージ",
			inputMsg:    map[string]interface{}{"data": "test"},
			wantHandled: false,
			wantPong:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockConn := testutil.NewMockWebSocketConn()
			handler := NewPingHandler(mockConn)

			msgBytes, _ := json.Marshal(tt.inputMsg)
			handled := handler.Handle(msgBytes)

			assert.Equal(t, tt.wantHandled, handled)

			if tt.wantPong {
				pong := mockConn.GetLastMessageAsMap()
				require.NotNil(t, pong)
				assert.Equal(t, "pong", pong["type"])
			}
		})
	}
}

func TestPingHandler_MultiplePings(t *testing.T) {
	mockConn := testutil.NewMockWebSocketConn()
	handler := NewPingHandler(mockConn)

	// 複数回のpingを処理
	for i := 0; i < 10; i++ {
		pingMsg, _ := json.Marshal(map[string]interface{}{"type": "ping"})
		handled := handler.Handle(pingMsg)
		assert.True(t, handled)
	}

	// すべてのpingに対してpongが送信された
	assert.Len(t, mockConn.GetMessages(), 10)
}

func TestPingHandler_ConnectionNotClosed(t *testing.T) {
	mockConn := testutil.NewMockWebSocketConn()
	handler := NewPingHandler(mockConn)

	pingMsg, _ := json.Marshal(map[string]interface{}{"type": "ping"})
	handler.Handle(pingMsg)

	// ping処理で接続が閉じられないことを確認
	assert.False(t, mockConn.GetIsClosed())
}

func TestPingHandler_InvalidJSON(t *testing.T) {
	mockConn := testutil.NewMockWebSocketConn()
	handler := NewPingHandler(mockConn)

	// 不正なJSONは処理されない
	handled := handler.Handle([]byte("invalid json"))

	assert.False(t, handled)
	assert.Empty(t, mockConn.GetMessages())
}

func TestIsPingMessage(t *testing.T) {
	tests := []struct {
		name    string
		message []byte
		want    bool
	}{
		{
			name:    "pingメッセージ",
			message: []byte(`{"type": "ping"}`),
			want:    true,
		},
		{
			name:    "pongメッセージ",
			message: []byte(`{"type": "pong"}`),
			want:    false,
		},
		{
			name:    "その他のメッセージ",
			message: []byte(`{"type": "message", "data": "hello"}`),
			want:    false,
		},
		{
			name:    "不正なJSON",
			message: []byte(`invalid`),
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsPingMessage(tt.message)
			assert.Equal(t, tt.want, result)
		})
	}
}
