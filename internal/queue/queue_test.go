package queue

import (
	"sync"
	"testing"
	"time"

	"hackz-ptera/back/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWaitingQueue_Add(t *testing.T) {
	tests := []struct {
		name      string
		addCount  int
		wantLen   int
	}{
		{
			name:     "正常系: 1人追加",
			addCount: 1,
			wantLen:  1,
		},
		{
			name:     "正常系: 複数人追加",
			addCount: 5,
			wantLen:  5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			q := NewWaitingQueue()

			for i := 0; i < tt.addCount; i++ {
				mockConn := testutil.NewMockWebSocketConn()
				q.AddUser(&QueueUser{
					ID:   string(rune('a' + i)),
					Conn: mockConn,
				})
			}

			assert.Equal(t, tt.wantLen, q.Len())
		})
	}
}

func TestWaitingQueue_Remove(t *testing.T) {
	q := NewWaitingQueue()

	mockConn := testutil.NewMockWebSocketConn()
	q.AddUser(&QueueUser{ID: "user1", Conn: mockConn})
	q.AddUser(&QueueUser{ID: "user2", Conn: testutil.NewMockWebSocketConn()})
	q.AddUser(&QueueUser{ID: "user3", Conn: testutil.NewMockWebSocketConn()})

	assert.Equal(t, 3, q.Len())

	// 中間のユーザーを削除
	q.Remove("user2")

	assert.Equal(t, 2, q.Len())

	// 存在しないユーザーの削除は影響なし
	q.Remove("non-existent")
	assert.Equal(t, 2, q.Len())
}

func TestWaitingQueue_GetPosition(t *testing.T) {
	tests := []struct {
		name         string
		userID       string
		totalUsers   int
		wantPosition int
		wantFound    bool
	}{
		{
			name:         "正常系: 先頭ユーザー",
			userID:       "user0",
			totalUsers:   5,
			wantPosition: 1,
			wantFound:    true,
		},
		{
			name:         "正常系: 中間ユーザー",
			userID:       "user2",
			totalUsers:   5,
			wantPosition: 3,
			wantFound:    true,
		},
		{
			name:         "正常系: 最後尾ユーザー",
			userID:       "user4",
			totalUsers:   5,
			wantPosition: 5,
			wantFound:    true,
		},
		{
			name:         "異常系: 存在しないユーザー",
			userID:       "non-existent",
			totalUsers:   5,
			wantPosition: 0,
			wantFound:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			q := NewWaitingQueue()

			for i := 0; i < tt.totalUsers; i++ {
				q.AddUser(&QueueUser{
					ID:   "user" + string(rune('0'+i)),
					Conn: testutil.NewMockWebSocketConn(),
				})
			}

			position, found := q.GetPosition(tt.userID)

			assert.Equal(t, tt.wantFound, found)
			assert.Equal(t, tt.wantPosition, position)
		})
	}
}

func TestWaitingQueue_BroadcastPositions(t *testing.T) {
	q := NewWaitingQueue()
	mockConns := make([]*testutil.MockWebSocketConn, 3)

	for i := 0; i < 3; i++ {
		mockConns[i] = testutil.NewMockWebSocketConn()
		q.AddUser(&QueueUser{
			ID:   "user" + string(rune('0'+i)),
			Conn: mockConns[i],
		})
	}

	// ブロードキャスト実行
	q.BroadcastPositions()

	// 各ユーザーがメッセージを受信したことを確認
	for i, conn := range mockConns {
		err := testutil.WaitFor(100*time.Millisecond, 10*time.Millisecond, func() bool {
			return conn.LastMessage != nil
		})
		require.NoError(t, err)

		msg := conn.GetLastMessageAsMap()
		require.NotNil(t, msg)

		assert.Equal(t, "queue_update", msg["type"])
		assert.Equal(t, float64(i+1), msg["position"])
		assert.Equal(t, float64(3), msg["total"])
	}
}

func TestWaitingQueue_PopFront(t *testing.T) {
	q := NewWaitingQueue()

	q.AddUser(&QueueUser{ID: "user1", Conn: testutil.NewMockWebSocketConn()})
	q.AddUser(&QueueUser{ID: "user2", Conn: testutil.NewMockWebSocketConn()})
	q.AddUser(&QueueUser{ID: "user3", Conn: testutil.NewMockWebSocketConn()})

	// 先頭を取り出し
	user := q.PopFront()
	assert.Equal(t, "user1", user.ID)
	assert.Equal(t, 2, q.Len())

	// 次の先頭を取り出し
	user = q.PopFront()
	assert.Equal(t, "user2", user.ID)
	assert.Equal(t, 1, q.Len())

	// 空のキューから取り出し
	q.PopFront()
	user = q.PopFront()
	assert.Nil(t, user)
}

func TestWaitingQueue_Concurrent(t *testing.T) {
	q := NewWaitingQueue()
	var wg sync.WaitGroup

	// 並行して追加・削除
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			userID := "user" + string(rune('0'+id%10))
			conn := testutil.NewMockWebSocketConn()

			q.AddUser(&QueueUser{ID: userID, Conn: conn})

			// 少し待ってから位置を取得
			time.Sleep(time.Millisecond)
			q.GetPosition(userID)

			// 一部を削除
			if id%3 == 0 {
				q.Remove(userID)
			}
		}(i)
	}

	wg.Wait()

	// 正常に完了したことを確認（パニックしていない）
	assert.True(t, true)
}
