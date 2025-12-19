// Package testutil provides common test utilities, mocks, and helpers for testing.
package testutil

import (
	"bytes"
	"encoding/json"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"time"

	"github.com/labstack/echo/v4"
)

// Error code constants for testing
const (
	ErrCodeSessionExpired = "SESSION_EXPIRED"
	ErrCodeInvalidSession = "INVALID_SESSION"
	ErrCodeTokenExpired   = "TOKEN_EXPIRED"
	ErrCodeInternalError  = "INTERNAL_ERROR"
)

// Status constants for testing
const (
	StatusWaiting    = "waiting"
	StatusStage1Dino = "stage1_dino"
	StatusRegistering = "registering"
)

// MockWebSocketConn is a mock implementation of WebSocket connection for testing.
type MockWebSocketConn struct {
	mu          sync.Mutex
	Messages    [][]byte
	LastMessage []byte
	IsClosed    bool
	ReadChan    chan []byte
	CloseChan   chan struct{}
	WriteErr    error
	CloseErr    error
}

// NewMockWebSocketConn creates a new MockWebSocketConn.
func NewMockWebSocketConn() *MockWebSocketConn {
	return &MockWebSocketConn{
		Messages:  make([][]byte, 0),
		ReadChan:  make(chan []byte, 100),
		CloseChan: make(chan struct{}),
	}
}

// WriteMessage mocks writing a message to WebSocket.
func (m *MockWebSocketConn) WriteMessage(messageType int, data []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.WriteErr != nil {
		return m.WriteErr
	}

	m.Messages = append(m.Messages, data)
	m.LastMessage = data
	return nil
}

// WriteJSON mocks writing JSON to WebSocket.
func (m *MockWebSocketConn) WriteJSON(v interface{}) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return m.WriteMessage(1, data)
}

// ReadMessage mocks reading a message from WebSocket.
func (m *MockWebSocketConn) ReadMessage() (int, []byte, error) {
	select {
	case msg := <-m.ReadChan:
		return 1, msg, nil
	case <-m.CloseChan:
		return 0, nil, io.EOF
	}
}

// Close mocks closing the WebSocket connection.
func (m *MockWebSocketConn) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.IsClosed {
		return nil
	}

	m.IsClosed = true
	close(m.CloseChan)

	if m.CloseErr != nil {
		return m.CloseErr
	}
	return nil
}

// GetMessages returns all messages sent through this connection.
func (m *MockWebSocketConn) GetMessages() [][]byte {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.Messages
}

// GetLastMessageAsMap returns the last message as a map.
func (m *MockWebSocketConn) GetLastMessageAsMap() map[string]interface{} {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.LastMessage == nil {
		return nil
	}

	var result map[string]interface{}
	_ = json.Unmarshal(m.LastMessage, &result)
	return result
}

// MockS3Client is a mock implementation of S3 client for testing.
type MockS3Client struct {
	mu           sync.Mutex
	Objects      map[string][]byte
	UploadedData map[string][]byte
	FishImages   []string
	GetErr       error
	PutErr       error
	ListErr      error
}

// NewMockS3Client creates a new MockS3Client.
func NewMockS3Client() *MockS3Client {
	return &MockS3Client{
		Objects:      make(map[string][]byte),
		UploadedData: make(map[string][]byte),
		FishImages:   []string{},
	}
}

// GetObject mocks S3 GetObject.
func (m *MockS3Client) GetObject(key string) ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.GetErr != nil {
		return nil, m.GetErr
	}

	data, ok := m.Objects[key]
	if !ok {
		return nil, &ObjectNotFoundError{Key: key}
	}
	return data, nil
}

// PutObject mocks S3 PutObject.
func (m *MockS3Client) PutObject(key string, data []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.PutErr != nil {
		return m.PutErr
	}

	m.UploadedData[key] = data
	return nil
}

// ListObjects mocks S3 ListObjects.
func (m *MockS3Client) ListObjects(prefix string) ([]string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.ListErr != nil {
		return nil, m.ListErr
	}

	var keys []string
	for key := range m.Objects {
		if len(prefix) == 0 || len(key) >= len(prefix) && key[:len(prefix)] == prefix {
			keys = append(keys, key)
		}
	}
	return keys, nil
}

// ObjectNotFoundError is returned when an S3 object is not found.
type ObjectNotFoundError struct {
	Key string
}

func (e *ObjectNotFoundError) Error() string {
	return "object not found: " + e.Key
}

// MockBedrockClient is a mock implementation of Bedrock client for testing.
type MockBedrockClient struct {
	mu          sync.Mutex
	Response    string
	Err         error
	LastPrompt  string
	LastModelID string
}

// NewMockBedrockClient creates a new MockBedrockClient.
func NewMockBedrockClient() *MockBedrockClient {
	return &MockBedrockClient{}
}

// InvokeModel mocks Bedrock InvokeModel.
func (m *MockBedrockClient) InvokeModel(modelID string, prompt string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.LastModelID = modelID
	m.LastPrompt = prompt

	if m.Err != nil {
		return "", m.Err
	}
	return m.Response, nil
}

// TestContext wraps Echo context for testing.
type TestContext struct {
	Echo     *echo.Echo
	Context  echo.Context
	Request  *http.Request
	Recorder *httptest.ResponseRecorder
}

// NewTestContext creates a new test context for Echo handlers.
func NewTestContext(method, path string, body io.Reader) *TestContext {
	e := echo.New()
	req := httptest.NewRequest(method, path, body)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	return &TestContext{
		Echo:     e,
		Context:  c,
		Request:  req,
		Recorder: rec,
	}
}

// NewTestContextWithJSON creates a test context with JSON body.
func NewTestContextWithJSON(method, path string, body interface{}) *TestContext {
	jsonBody, _ := json.Marshal(body)
	tc := NewTestContext(method, path, bytes.NewReader(jsonBody))
	tc.Request.Header.Set("Content-Type", "application/json")
	return tc
}

// SetCookie adds a cookie to the test request.
func (tc *TestContext) SetCookie(name, value string) {
	tc.Request.AddCookie(&http.Cookie{Name: name, Value: value})
}

// GetResponseBody returns the response body as a map.
func (tc *TestContext) GetResponseBody() map[string]interface{} {
	var result map[string]interface{}
	_ = json.Unmarshal(tc.Recorder.Body.Bytes(), &result)
	return result
}

// GetResponseCode returns the HTTP response status code.
func (tc *TestContext) GetResponseCode() int {
	return tc.Recorder.Code
}

// WaitFor waits for a condition to be true within timeout.
func WaitFor(timeout, interval time.Duration, condition func() bool) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if condition() {
			return nil
		}
		time.Sleep(interval)
	}
	return &TimeoutError{Timeout: timeout}
}

// TimeoutError is returned when WaitFor times out.
type TimeoutError struct {
	Timeout time.Duration
}

func (e *TimeoutError) Error() string {
	return "timeout waiting for condition"
}

// WaitForMessage waits for a WebSocket message and returns it as a map.
func WaitForMessage(conn *MockWebSocketConn, timeout time.Duration) map[string]interface{} {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		conn.mu.Lock()
		if conn.LastMessage != nil {
			var msg map[string]interface{}
			_ = json.Unmarshal(conn.LastMessage, &msg)
			conn.mu.Unlock()
			return msg
		}
		conn.mu.Unlock()
		time.Sleep(10 * time.Millisecond)
	}
	return nil
}

// WaitForMessages waits for at least n messages and returns them.
func WaitForMessages(conn *MockWebSocketConn, n int, timeout time.Duration) []map[string]interface{} {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		conn.mu.Lock()
		if len(conn.Messages) >= n {
			result := make([]map[string]interface{}, len(conn.Messages))
			for i, msg := range conn.Messages {
				var m map[string]interface{}
				json.Unmarshal(msg, &m)
				result[i] = m
			}
			conn.mu.Unlock()
			return result
		}
		conn.mu.Unlock()
		time.Sleep(10 * time.Millisecond)
	}
	return nil
}

// CreateTestPNG creates a test PNG image with specified dimensions.
func CreateTestPNG(width, height int) []byte {
	img := image.NewRGBA(image.Rect(0, 0, width, height))

	// Fill with a simple pattern
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.Set(x, y, color.RGBA{
				R: uint8(x % 256),
				G: uint8(y % 256),
				B: uint8((x + y) % 256),
				A: 255,
			})
		}
	}

	var buf bytes.Buffer
	_ = png.Encode(&buf, img)
	return buf.Bytes()
}

// CreateTestJPEG creates a test JPEG image with specified dimensions.
func CreateTestJPEG(width, height int) []byte {
	img := image.NewRGBA(image.Rect(0, 0, width, height))

	// Fill with a simple pattern
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.Set(x, y, color.RGBA{
				R: uint8(x % 256),
				G: uint8(y % 256),
				B: uint8((x + y) % 256),
				A: 255,
			})
		}
	}

	var buf bytes.Buffer
	_ = jpeg.Encode(&buf, img, &jpeg.Options{Quality: 80})
	return buf.Bytes()
}

// CreateTestImage creates a test image.Image with specified dimensions.
func CreateTestImage(width, height int) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, width, height))

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.Set(x, y, color.RGBA{
				R: uint8(x % 256),
				G: uint8(y % 256),
				B: uint8((x + y) % 256),
				A: 255,
			})
		}
	}

	return img
}

// AssertJSONResponse parses JSON response and returns as map.
func AssertJSONResponse(rec *httptest.ResponseRecorder) map[string]interface{} {
	var result map[string]interface{}
	json.Unmarshal(rec.Body.Bytes(), &result)
	return result
}
