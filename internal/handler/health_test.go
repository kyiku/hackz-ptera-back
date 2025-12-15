package handler

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/kyiku/hackz-ptera-back/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHealthHandler_Check(t *testing.T) {
	tests := []struct {
		name           string
		method         string
		wantStatusCode int
		wantStatus     string
	}{
		{
			name:           "正常系: GETリクエスト",
			method:         http.MethodGet,
			wantStatusCode: http.StatusOK,
			wantStatus:     "ok",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tc := testutil.NewTestContext(tt.method, "/health", nil)

			h := NewHealthHandler()
			err := h.Check(tc.Context)

			require.NoError(t, err)
			assert.Equal(t, tt.wantStatusCode, tc.Recorder.Code)

			var resp map[string]interface{}
			err = json.Unmarshal(tc.Recorder.Body.Bytes(), &resp)
			require.NoError(t, err)

			assert.Equal(t, tt.wantStatus, resp["status"])
		})
	}
}
