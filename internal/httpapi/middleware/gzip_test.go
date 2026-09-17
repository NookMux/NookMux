package middleware

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/NookMux/NookMux/internal/domain/shared"
	"github.com/NookMux/NookMux/internal/httpapi"
	"github.com/andybalholm/brotli"
	"github.com/gin-gonic/gin"
	"github.com/klauspost/compress/zstd"
)

func TestDecompressRequestMiddlewareSupportsRequestEncodings(t *testing.T) {
	const payload = `{"model":"gpt-5","input":[{"type":"function_call","call_id":"call_weather","name":"get_weather","arguments":"{\"city\":\"beijing\"}"},{"type":"function_call_output","call_id":"call_weather","output":"sunny"}],"stream":true}`

	tests := []struct {
		name    string
		header  string
		encoder func([]byte) ([]byte, error)
	}{
		{
			name:   "gzip",
			header: "gzip",
			encoder: func(body []byte) ([]byte, error) {
				var compressed bytes.Buffer
				writer := gzip.NewWriter(&compressed)
				if _, err := writer.Write(body); err != nil {
					return nil, err
				}
				if err := writer.Close(); err != nil {
					return nil, err
				}
				return compressed.Bytes(), nil
			},
		},
		{
			name:   "brotli",
			header: "br",
			encoder: func(body []byte) ([]byte, error) {
				var compressed bytes.Buffer
				writer := brotli.NewWriter(&compressed)
				if _, err := writer.Write(body); err != nil {
					return nil, err
				}
				if err := writer.Close(); err != nil {
					return nil, err
				}
				return compressed.Bytes(), nil
			},
		},
		{
			name:   "zstd",
			header: " ZSTD ",
			encoder: func(body []byte) ([]byte, error) {
				var compressed bytes.Buffer
				writer, err := zstd.NewWriter(&compressed)
				if err != nil {
					return nil, err
				}
				if _, err := writer.Write(body); err != nil {
					_ = writer.Close()
					return nil, err
				}
				if err := writer.Close(); err != nil {
					return nil, err
				}
				return compressed.Bytes(), nil
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			router := gin.New()
			router.POST("/resource", DecompressRequestMiddleware(), func(c *gin.Context) {
				if c.Request.ContentLength != -1 {
					c.String(http.StatusExpectationFailed, "content length = %d, want -1 after decompression", c.Request.ContentLength)
					return
				}
				if got := c.GetHeader("Content-Length"); got != "" {
					c.String(http.StatusExpectationFailed, "content-length header was not removed: %q", got)
					return
				}
				body, err := io.ReadAll(c.Request.Body)
				if err != nil {
					c.String(http.StatusBadRequest, "read body: %v", err)
					return
				}
				if got := c.GetHeader("Content-Encoding"); got != "" {
					c.String(http.StatusExpectationFailed, "content encoding was not removed: %q", got)
					return
				}
				c.Data(http.StatusOK, http.DetectContentType(body), body)
			})

			compressed, err := tt.encoder([]byte(payload))
			if err != nil {
				t.Fatalf("compress body: %v", err)
			}
			req := httptest.NewRequest(http.MethodPost, "/resource", bytes.NewReader(compressed))
			req.Header.Set("Content-Encoding", tt.header)
			req.Header.Set("Content-Length", strconv.Itoa(len(compressed)))
			req.Header.Set("Content-Type", "application/json")
			recorder := httptest.NewRecorder()

			router.ServeHTTP(recorder, req)

			if recorder.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d; body = %q", recorder.Code, http.StatusOK, recorder.Body.String())
			}
			if recorder.Body.String() != payload {
				t.Fatalf("body = %q, want %q", recorder.Body.String(), payload)
			}
		})
	}
}

func TestDecompressRequestMiddlewarePreservesZstdToolResultBodyForReusableReads(t *testing.T) {
	const payload = `{"model":"gpt-5","previous_response_id":"resp_previous","input":[{"type":"function_call_output","call_id":"call_weather","output":"sunny"}],"stream":true}`

	var compressed bytes.Buffer
	writer, err := zstd.NewWriter(&compressed)
	if err != nil {
		t.Fatalf("create zstd writer: %v", err)
	}
	if _, err := writer.Write([]byte(payload)); err != nil {
		_ = writer.Close()
		t.Fatalf("compress request body: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close zstd writer: %v", err)
	}

	type responsesToolResultRequest struct {
		Model              string `json:"model"`
		PreviousResponseID string `json:"previous_response_id"`
		Input              []struct {
			Type   string `json:"type"`
			CallID string `json:"call_id"`
			Output string `json:"output"`
		} `json:"input"`
		Stream bool `json:"stream"`
	}

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/v1/responses", DecompressRequestMiddleware(), func(c *gin.Context) {
		var firstRead responsesToolResultRequest
		if err := httpapi.UnmarshalBodyReusable(c, &firstRead); err != nil {
			c.String(http.StatusBadRequest, "first reusable read: %v", err)
			return
		}

		var secondRead responsesToolResultRequest
		if err := httpapi.UnmarshalBodyReusable(c, &secondRead); err != nil {
			c.String(http.StatusBadRequest, "second reusable read: %v", err)
			return
		}

		if firstRead.Model != "gpt-5" || firstRead.PreviousResponseID != "resp_previous" || !firstRead.Stream {
			c.String(http.StatusExpectationFailed, "first read lost responses metadata: %#v", firstRead)
			return
		}
		if len(firstRead.Input) != 1 || firstRead.Input[0].Type != "function_call_output" || firstRead.Input[0].CallID != "call_weather" || firstRead.Input[0].Output != "sunny" {
			c.String(http.StatusExpectationFailed, "first read lost tool result: %#v", firstRead.Input)
			return
		}
		if secondRead.Model != firstRead.Model || secondRead.PreviousResponseID != firstRead.PreviousResponseID || len(secondRead.Input) != len(firstRead.Input) {
			c.String(http.StatusExpectationFailed, "second read differs from first read: %#v", secondRead)
			return
		}

		rawBody, err := httpapi.GetRequestBody(c)
		if err != nil {
			c.String(http.StatusBadRequest, "read cached request body: %v", err)
			return
		}
		if string(rawBody) != payload {
			c.String(http.StatusExpectationFailed, "cached body = %q, want %q", string(rawBody), payload)
			return
		}

		c.Status(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(compressed.Bytes()))
	req.Header.Set("Content-Encoding", "zstd")
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d; body = %q", recorder.Code, http.StatusNoContent, recorder.Body.String())
	}
}

func TestDecompressRequestMiddlewareRejectsInvalidZstdBody(t *testing.T) {
	oldLimit := shared.MaxRequestBodyMB
	shared.MaxRequestBodyMB = 1
	t.Cleanup(func() {
		shared.MaxRequestBodyMB = oldLimit
	})

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/resource", DecompressRequestMiddleware(), func(c *gin.Context) {
		if _, err := io.ReadAll(c.Request.Body); err != nil {
			c.String(http.StatusBadRequest, "read body: %v", err)
			return
		}
		c.Status(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodPost, "/resource", bytes.NewReader([]byte("not-zstd")))
	req.Header.Set("Content-Encoding", "zstd")
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusBadRequest)
	}
}
