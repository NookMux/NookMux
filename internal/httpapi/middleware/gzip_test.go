package middleware

import (
	"bytes"
	"compress/gzip"
	"encoding/binary"
	"io"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strconv"
	"sync/atomic"
	"testing"

	"github.com/NookMux/NookMux/internal/domain/shared"
	"github.com/NookMux/NookMux/internal/httpapi"
	"github.com/andybalholm/brotli"
	"github.com/gin-gonic/gin"
	"github.com/klauspost/compress/zstd"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

var zstdMagic = []byte{0x28, 0xb5, 0x2f, 0xfd}

// zstdRLEBlock 是一个仅重复一次 'a' 的 last block。
var zstdRLEBlock = []byte{0x0b, 0x00, 0x00, 'a'}

func readZstdBody(t *testing.T, body []byte) ([]byte, error) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	router := gin.New()
	var data []byte
	var readErr error
	router.POST("/v1/chat/completions", DecompressRequestMiddleware(), func(c *gin.Context) {
		data, readErr = io.ReadAll(c.Request.Body)
	})
	request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	request.Header.Set("Content-Encoding", "zstd")
	t.Cleanup(func() {
		assert.NoError(t, request.Body.Close())
	})
	router.ServeHTTP(httptest.NewRecorder(), request)
	return data, readErr
}

func TestDecompressRequestZstd(t *testing.T) {
	previousMaxBodyMB := shared.MaxRequestBodyMB
	shared.MaxRequestBodyMB = 16
	t.Cleanup(func() {
		shared.MaxRequestBodyMB = previousMaxBodyMB
	})

	encoder, err := zstd.NewWriter(nil, zstd.WithEncoderConcurrency(1))
	require.NoError(t, err)
	payload := bytes.Repeat([]byte(`{"role":"user","content":"hello"},`), 1<<15)
	compressed := encoder.EncodeAll(payload, nil)
	require.NoError(t, encoder.Close())

	// 窗口描述符 0x50、0x68、0x69、0x98 分别声明 1MiB、8MiB、9MiB、512MiB 窗口。
	smallWindowFrame := append(append(append([]byte{}, zstdMagic...), 0x00, 0x50), zstdRLEBlock...)
	boundaryWindowFrame := append(append(append([]byte{}, zstdMagic...), 0x00, 0x68), zstdRLEBlock...)
	aboveWindowFrame := append(append(append([]byte{}, zstdMagic...), 0x00, 0x69), zstdRLEBlock...)
	largeWindowFrame := append(append(append([]byte{}, zstdMagic...), 0x00, 0x98), zstdRLEBlock...)
	// 单段帧用声明的 content size（此处 100MiB）作为窗口。
	largeContentFrame := binary.LittleEndian.AppendUint64(append(append([]byte{}, zstdMagic...), 0xe0), 100<<20)
	largeContentFrame = append(largeContentFrame, zstdRLEBlock...)

	largePayload := bytes.Repeat([]byte("a"), 9<<20)
	boundedEncoder, err := zstd.NewWriter(nil, zstd.WithWindowSize(1<<20), zstd.WithSingleSegment(false), zstd.WithEncoderConcurrency(1))
	require.NoError(t, err)
	largeBody := boundedEncoder.EncodeAll(largePayload, nil)
	require.NoError(t, boundedEncoder.Close())
	var header zstd.Header
	require.NoError(t, header.Decode(largeBody))
	require.False(t, header.SingleSegment)
	require.LessOrEqual(t, header.WindowSize, uint64(8<<20))

	singleEncoder, err := zstd.NewWriter(nil, zstd.WithSingleSegment(true), zstd.WithEncoderConcurrency(1))
	require.NoError(t, err)
	boundaryContentFrame := singleEncoder.EncodeAll(largePayload[:8<<20], nil)
	aboveContentFrame := singleEncoder.EncodeAll(largePayload[:(8<<20)+1], nil)
	require.NoError(t, singleEncoder.Close())
	for _, frame := range [][]byte{boundaryContentFrame, aboveContentFrame} {
		require.NoError(t, header.Decode(frame))
		require.True(t, header.SingleSegment)
	}

	cases := []struct {
		name           string
		body           []byte
		wantBody       []byte
		wantErr        error
		maxBodyMB      int
		wantLimitError bool
	}{
		{name: "regular body is decompressed", body: compressed, wantBody: payload},
		{name: "512MiB window is rejected", body: largeWindowFrame, wantErr: zstd.ErrWindowSizeExceeded},
		{name: "100MiB single-segment window is rejected", body: largeContentFrame, wantErr: zstd.ErrDecoderSizeExceeded},
		{name: "8MiB window is accepted", body: boundaryWindowFrame, wantBody: []byte("a")},
		{name: "9MiB window is rejected", body: aboveWindowFrame, wantErr: zstd.ErrWindowSizeExceeded},
		{name: "8MiB single-segment body is accepted", body: boundaryContentFrame, wantBody: largePayload[:8<<20]},
		{name: "single-segment body one byte above 8MiB is rejected", body: aboveContentFrame, wantErr: zstd.ErrDecoderSizeExceeded},
		{name: "9MiB body with a bounded window is decompressed", body: largeBody, wantBody: largePayload},
		{name: "bounded window does not bypass the decoded body limit", body: largeBody, wantBody: largePayload[:1<<20], maxBodyMB: 1, wantLimitError: true},
		{name: "window equal to a smaller request limit is accepted", body: smallWindowFrame, wantBody: []byte("a"), maxBodyMB: 1},
		{name: "window above a smaller request limit is rejected", body: boundaryWindowFrame, wantErr: zstd.ErrWindowSizeExceeded, maxBodyMB: 1},
		{name: "single-segment window above a smaller request limit is rejected", body: boundaryContentFrame, wantErr: zstd.ErrDecoderSizeExceeded, maxBodyMB: 1},
		{name: "oversized window in a later frame is rejected", body: append(append([]byte{}, compressed...), largeWindowFrame...), wantBody: payload, wantErr: zstd.ErrWindowSizeExceeded},
		{name: "oversized single-segment window in a later frame is rejected", body: append(append([]byte{}, compressed...), aboveContentFrame...), wantBody: payload, wantErr: zstd.ErrDecoderSizeExceeded},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.maxBodyMB > 0 {
				shared.MaxRequestBodyMB = tc.maxBodyMB
				t.Cleanup(func() {
					shared.MaxRequestBodyMB = 16
				})
			}
			data, err := readZstdBody(t, tc.body)

			switch {
			case tc.wantLimitError:
				var sizeErr *http.MaxBytesError
				require.ErrorAs(t, err, &sizeErr)
				assert.Equal(t, int64(tc.maxBodyMB)<<20, sizeErr.Limit)
			case tc.wantErr != nil:
				require.ErrorIs(t, err, tc.wantErr)
			default:
				require.NoError(t, err)
			}
			if tc.wantBody == nil {
				assert.Empty(t, data)
				return
			}
			assert.Equal(t, tc.wantBody, data)
		})
	}
}

type zstdBodyReadCounter struct {
	reads atomic.Int32
}

func (b *zstdBodyReadCounter) Read([]byte) (int, error) {
	b.reads.Add(1)
	return 0, io.EOF
}

func TestDecompressRequestZstdEarlyRejectionDoesNotReadBody(t *testing.T) {
	// 在默认解码器本就可能同步的主机（GOMAXPROCS=1）上保持回归有效性。
	previousProcs := runtime.GOMAXPROCS(2)
	t.Cleanup(func() {
		runtime.GOMAXPROCS(previousProcs)
	})

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/v1/chat/completions", DecompressRequestMiddleware(), func(c *gin.Context) {
		c.AbortWithStatus(http.StatusUnauthorized)
	})
	body := &zstdBodyReadCounter{}
	request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", io.NopCloser(body))
	request.Header.Set("Content-Encoding", "zstd")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	// Close 同时汇合可能存在的异步解码器，再检查读取计数。
	require.NoError(t, request.Body.Close())
	assert.Equal(t, http.StatusUnauthorized, response.Code)
	assert.Zero(t, body.reads.Load(), "an early rejection must not consume the request body")
}
