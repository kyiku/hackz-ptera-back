package testutil

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMockWebSocketConn_WriteMessage(t *testing.T) {
	conn := NewMockWebSocketConn()

	err := conn.WriteMessage(1, []byte(`{"type":"test"}`))
	require.NoError(t, err)

	assert.Len(t, conn.Messages, 1)
	assert.Equal(t, []byte(`{"type":"test"}`), conn.LastMessage)
}

func TestMockWebSocketConn_WriteJSON(t *testing.T) {
	conn := NewMockWebSocketConn()

	msg := map[string]string{"type": "test"}
	err := conn.WriteJSON(msg)
	require.NoError(t, err)

	assert.Len(t, conn.Messages, 1)
	result := conn.GetLastMessageAsMap()
	assert.Equal(t, "test", result["type"])
}

func TestMockWebSocketConn_Close(t *testing.T) {
	conn := NewMockWebSocketConn()

	assert.False(t, conn.IsClosed)

	err := conn.Close()
	require.NoError(t, err)
	assert.True(t, conn.IsClosed)

	// Closing again should not error
	err = conn.Close()
	require.NoError(t, err)
}

func TestMockS3Client_GetObject(t *testing.T) {
	client := NewMockS3Client()
	client.Objects["test-key"] = []byte("test-content")

	data, err := client.GetObject("test-key")
	require.NoError(t, err)
	assert.Equal(t, []byte("test-content"), data)

	// Not found
	_, err = client.GetObject("nonexistent")
	assert.Error(t, err)
	assert.IsType(t, &ObjectNotFoundError{}, err)
}

func TestMockS3Client_PutObject(t *testing.T) {
	client := NewMockS3Client()

	err := client.PutObject("test-key", []byte("test-content"))
	require.NoError(t, err)

	assert.Equal(t, []byte("test-content"), client.UploadedData["test-key"])
}

func TestMockS3Client_ListObjects(t *testing.T) {
	client := NewMockS3Client()
	client.Objects["fish/salmon.jpg"] = []byte("data1")
	client.Objects["fish/tuna.jpg"] = []byte("data2")
	client.Objects["captcha/bg.png"] = []byte("data3")

	keys, err := client.ListObjects("fish/")
	require.NoError(t, err)
	assert.Len(t, keys, 2)
}

func TestMockBedrockClient_InvokeModel(t *testing.T) {
	client := NewMockBedrockClient()
	client.Response = "test response"

	response, err := client.InvokeModel("model-id", "test prompt")
	require.NoError(t, err)
	assert.Equal(t, "test response", response)
	assert.Equal(t, "test prompt", client.LastPrompt)
	assert.Equal(t, "model-id", client.LastModelID)
}

func TestTestContext(t *testing.T) {
	tc := NewTestContext(http.MethodGet, "/test", nil)

	assert.NotNil(t, tc.Echo)
	assert.NotNil(t, tc.Context)
	assert.NotNil(t, tc.Request)
	assert.NotNil(t, tc.Recorder)
}

func TestTestContextWithJSON(t *testing.T) {
	body := map[string]string{"key": "value"}
	tc := NewTestContextWithJSON(http.MethodPost, "/test", body)

	assert.Equal(t, "application/json", tc.Request.Header.Get("Content-Type"))
}

func TestTestContext_SetCookie(t *testing.T) {
	tc := NewTestContext(http.MethodGet, "/test", nil)
	tc.SetCookie("session_id", "test-session")

	cookie, err := tc.Request.Cookie("session_id")
	require.NoError(t, err)
	assert.Equal(t, "test-session", cookie.Value)
}

func TestTestContext_GetResponseBody(t *testing.T) {
	tc := NewTestContext(http.MethodGet, "/test", nil)
	tc.Recorder.WriteHeader(http.StatusOK)
	_, _ = tc.Recorder.Write([]byte(`{"status":"ok"}`))

	body := tc.GetResponseBody()
	assert.Equal(t, "ok", body["status"])
}

func TestTestContext_GetResponseCode(t *testing.T) {
	tc := NewTestContext(http.MethodGet, "/test", nil)
	tc.Recorder.WriteHeader(http.StatusCreated)

	assert.Equal(t, http.StatusCreated, tc.GetResponseCode())
}

func TestWaitFor(t *testing.T) {
	counter := 0

	err := WaitFor(100*time.Millisecond, 10*time.Millisecond, func() bool {
		counter++
		return counter >= 3
	})

	require.NoError(t, err)
	assert.GreaterOrEqual(t, counter, 3)
}

func TestWaitFor_Timeout(t *testing.T) {
	err := WaitFor(50*time.Millisecond, 10*time.Millisecond, func() bool {
		return false // Never true
	})

	assert.Error(t, err)
	assert.IsType(t, &TimeoutError{}, err)
}

func TestCreateTestPNG(t *testing.T) {
	data := CreateTestPNG(100, 100)
	assert.NotEmpty(t, data)
	// PNG files start with specific bytes
	assert.Equal(t, uint8(0x89), data[0])
	assert.Equal(t, uint8('P'), data[1])
	assert.Equal(t, uint8('N'), data[2])
	assert.Equal(t, uint8('G'), data[3])
}

func TestCreateTestJPEG(t *testing.T) {
	data := CreateTestJPEG(100, 100)
	assert.NotEmpty(t, data)
	// JPEG files start with 0xFFD8
	assert.Equal(t, uint8(0xFF), data[0])
	assert.Equal(t, uint8(0xD8), data[1])
}

func TestCreateTestImage(t *testing.T) {
	img := CreateTestImage(200, 150)
	assert.NotNil(t, img)
	assert.Equal(t, 200, img.Bounds().Dx())
	assert.Equal(t, 150, img.Bounds().Dy())
}

func TestConstants(t *testing.T) {
	// Error code constants
	assert.Equal(t, "SESSION_EXPIRED", ErrCodeSessionExpired)
	assert.Equal(t, "INVALID_SESSION", ErrCodeInvalidSession)
	assert.Equal(t, "TOKEN_EXPIRED", ErrCodeTokenExpired)
	assert.Equal(t, "INTERNAL_ERROR", ErrCodeInternalError)

	// Status constants
	assert.Equal(t, "waiting", StatusWaiting)
	assert.Equal(t, "stage1_dino", StatusStage1Dino)
	assert.Equal(t, "registering", StatusRegistering)
}
