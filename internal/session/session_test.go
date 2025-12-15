package session

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSessionStore_Create(t *testing.T) {
	tests := []struct {
		name           string
		wantNonEmptyID bool
		wantStatus     string
	}{
		{
			name:           "正常系: 新規セッション作成",
			wantNonEmptyID: true,
			wantStatus:     "waiting",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := NewSessionStore()

			user, sessionID := store.Create()

			assert.NotEmpty(t, sessionID)
			assert.NotNil(t, user)
			assert.Equal(t, tt.wantStatus, user.Status)
			assert.NotEmpty(t, user.ID)
		})
	}
}

func TestSessionStore_Get(t *testing.T) {
	tests := []struct {
		name      string
		createFirst bool
		useValidID  bool
		wantFound   bool
	}{
		{
			name:        "正常系: 存在するセッション",
			createFirst: true,
			useValidID:  true,
			wantFound:   true,
		},
		{
			name:        "異常系: 存在しないセッション",
			createFirst: false,
			useValidID:  false,
			wantFound:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := NewSessionStore()

			var sessionID string
			if tt.createFirst {
				_, sessionID = store.Create()
			} else {
				sessionID = "non-existent-session"
			}

			if !tt.useValidID {
				sessionID = "invalid-session-id"
			}

			user, found := store.Get(sessionID)

			assert.Equal(t, tt.wantFound, found)
			if tt.wantFound {
				assert.NotNil(t, user)
			} else {
				assert.Nil(t, user)
			}
		})
	}
}

func TestSessionStore_Delete(t *testing.T) {
	store := NewSessionStore()
	_, sessionID := store.Create()

	// 削除前は存在する
	_, found := store.Get(sessionID)
	assert.True(t, found)

	// 削除
	store.Delete(sessionID)

	// 削除後は存在しない
	_, found = store.Get(sessionID)
	assert.False(t, found)
}

func TestSessionStore_Expiry(t *testing.T) {
	tests := []struct {
		name       string
		expiry     time.Duration
		waitTime   time.Duration
		wantFound  bool
	}{
		{
			name:      "正常系: 有効期限内",
			expiry:    100 * time.Millisecond,
			waitTime:  10 * time.Millisecond,
			wantFound: true,
		},
		{
			name:      "異常系: 有効期限切れ",
			expiry:    50 * time.Millisecond,
			waitTime:  100 * time.Millisecond,
			wantFound: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := NewSessionStoreWithExpiry(tt.expiry)
			_, sessionID := store.Create()

			time.Sleep(tt.waitTime)

			_, found := store.Get(sessionID)
			assert.Equal(t, tt.wantFound, found)
		})
	}
}

func TestSessionStore_Concurrent(t *testing.T) {
	store := NewSessionStore()
	done := make(chan bool)

	// 並行してセッションを作成
	for i := 0; i < 100; i++ {
		go func() {
			user, sessionID := store.Create()
			require.NotEmpty(t, sessionID)
			require.NotNil(t, user)

			// 作成したセッションを取得できることを確認
			retrieved, found := store.Get(sessionID)
			require.True(t, found)
			require.Equal(t, user.ID, retrieved.ID)

			done <- true
		}()
	}

	// 全てのゴルーチンが完了するまで待機
	for i := 0; i < 100; i++ {
		<-done
	}
}
