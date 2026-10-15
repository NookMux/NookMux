package vertex

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/NookMux/NookMux/internal/domain/shared"
	relaycommon "github.com/NookMux/NookMux/internal/relay/common"
	relayconstant "github.com/NookMux/NookMux/internal/relay/constant"
	"github.com/NookMux/NookMux/pkg/jsonx"
	"github.com/gin-gonic/gin"
)

func vertexInfo(t *testing.T, baseURL string, apiKey string, apiVersion string, stream bool) *relaycommon.RelayInfo {
	t.Helper()
	return &relaycommon.RelayInfo{
		IsStream:        stream,
		OriginModelName: "gemini-2.5-pro",
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType:       17,
			ChannelBaseUrl:    baseURL,
			ApiKey:            apiKey,
			ApiVersion:        apiVersion,
			UpstreamModelName: "gemini-2.5-pro",
			ChannelOtherSettings: shared.ChannelOtherSettings{
				VertexKeyType: shared.VertexKeyTypeJSON,
			},
		},
	}
}

func serviceAccountJSON(t *testing.T, projectID string) string {
	t.Helper()
	raw, err := jsonx.Marshal(Credentials{ProjectID: projectID})
	if err != nil {
		t.Fatalf("marshal credentials: %v", err)
	}
	return string(raw)
}

// TestAdaptorGetRequestURL_HonorsCustomBaseURL verifies the user-facing fix for
// #148: a Vertex channel configured with a custom base_url/gateway prefix must
// send requests to that gateway instead of the hardcoded Google host.
func TestAdaptorGetRequestURL_HonorsCustomBaseURL(t *testing.T) {
	a := &Adaptor{RequestMode: RequestModeGemini}
	info := vertexInfo(t, "https://gateway.example/vertex", serviceAccountJSON(t, "p1"), "us-central1", false)

	got, err := a.GetRequestURL(info)
	if err != nil {
		t.Fatalf("GetRequestURL error = %v", err)
	}
	if !strings.HasPrefix(got, "https://gateway.example/vertex/") {
		t.Fatalf("Gemini URL did not honor custom base_url: %s", got)
	}
	if !strings.Contains(got, "publishers/google/models/gemini-2.5-pro:generateContent") {
		t.Fatalf("Gemini URL lost model/action suffix: %s", got)
	}

	// Streaming must keep the alt=sse query on the same gateway host.
	info.IsStream = true
	got, err = a.GetRequestURL(info)
	if err != nil {
		t.Fatalf("GetRequestURL stream error = %v", err)
	}
	if !strings.HasPrefix(got, "https://gateway.example/vertex/") {
		t.Fatalf("Gemini stream URL did not honor custom base_url: %s", got)
	}
	if !strings.Contains(got, "streamGenerateContent?alt=sse") {
		t.Fatalf("Gemini stream URL lost streaming suffix: %s", got)
	}
}

// TestAdaptorGetRequestURL_ClaudeHonorsCustomBaseURL ensures Claude-on-Vertex
// requests also route through a configured gateway prefix.
func TestAdaptorGetRequestURL_ClaudeHonorsCustomBaseURL(t *testing.T) {
	a := &Adaptor{RequestMode: RequestModeClaude}
	info := vertexInfo(t, "https://gateway.example/vertex", serviceAccountJSON(t, "p1"), "global", false)
	info.UpstreamModelName = "claude-sonnet-4-20250514"
	info.OriginModelName = "claude-sonnet-4-20250514"

	got, err := a.GetRequestURL(info)
	if err != nil {
		t.Fatalf("GetRequestURL error = %v", err)
	}
	if !strings.HasPrefix(got, "https://gateway.example/vertex/") {
		t.Fatalf("Claude URL did not honor custom base_url: %s", got)
	}
	if !strings.Contains(got, "publishers/anthropic/models/claude-sonnet-4@20250514:rawPredict") {
		t.Fatalf("Claude URL lost anthropic publisher/model/action: %s", got)
	}
}

// TestAdaptorGetRequestURL_DefaultHostUnchanged guards against regressing the
// default Google host when no base_url is configured.
func TestAdaptorGetRequestURL_DefaultHostUnchanged(t *testing.T) {
	a := &Adaptor{RequestMode: RequestModeGemini}
	info := vertexInfo(t, "", serviceAccountJSON(t, "p1"), "global", false)

	got, err := a.GetRequestURL(info)
	if err != nil {
		t.Fatalf("GetRequestURL error = %v", err)
	}
	if !strings.HasPrefix(got, "https://aiplatform.googleapis.com/") {
		t.Fatalf("default Gemini URL should keep Google host, got: %s", got)
	}
}

// TestAdaptorGetRequestURL_APIKeyModeKeepsKeyOutOfURL ensures the express-mode
// API key travels in the x-goog-api-key header instead of the URL query, and
// that the URL keeps the project-less express shape.
func TestAdaptorGetRequestURL_APIKeyModeKeepsKeyOutOfURL(t *testing.T) {
	a := &Adaptor{RequestMode: RequestModeGemini}
	info := vertexInfo(t, "", "express-key", "global", false)
	info.ChannelOtherSettings.VertexKeyType = shared.VertexKeyTypeAPIKey

	got, err := a.GetRequestURL(info)
	if err != nil {
		t.Fatalf("GetRequestURL error = %v", err)
	}
	if strings.Contains(got, "key=") {
		t.Fatalf("API key leaked into URL query: %s", got)
	}
	if !strings.HasPrefix(got, "https://aiplatform.googleapis.com/v1/publishers/google/models/") {
		t.Fatalf("express URL should drop projects/locations prefix, got: %s", got)
	}

	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1beta/models/gemini-2.5-pro:generateContent", nil)
	header := http.Header{}
	if err := a.SetupRequestHeader(ctx, &header, info); err != nil {
		t.Fatalf("SetupRequestHeader error = %v", err)
	}
	if header.Get("x-goog-api-key") != "express-key" {
		t.Fatalf("x-goog-api-key header not set correctly, got: %q", header.Get("x-goog-api-key"))
	}
}

// TestAdaptorGetRequestURL_APIKeyModeRejectsOpenSource verifies the explicit
// error for the unsupported api_key + open-source model combination.
func TestAdaptorGetRequestURL_APIKeyModeRejectsOpenSource(t *testing.T) {
	a := &Adaptor{RequestMode: RequestModeOpenSource}
	info := vertexInfo(t, "", "express-key", "global", false)
	info.ChannelOtherSettings.VertexKeyType = shared.VertexKeyTypeAPIKey

	_, err := a.GetRequestURL(info)
	if err == nil {
		t.Fatal("expected error for api_key mode with open-source model endpoint")
	}
	if !strings.Contains(err.Error(), "service account") {
		t.Fatalf("unexpected error message: %v", err)
	}
}

// TestAdaptorGetRequestURL_CountTokensAction verifies the Gemini-native
// countTokens action maps to the upstream :countTokens endpoint.
func TestAdaptorGetRequestURL_CountTokensAction(t *testing.T) {
	a := &Adaptor{RequestMode: RequestModeGemini}
	info := vertexInfo(t, "", serviceAccountJSON(t, "p1"), "us-central1", false)
	info.RelayMode = relayconstant.RelayModeGemini
	info.RequestURLPath = "/v1beta/models/gemini-2.5-pro:countTokens"

	got, err := a.GetRequestURL(info)
	if err != nil {
		t.Fatalf("GetRequestURL error = %v", err)
	}
	if !strings.HasSuffix(got, "publishers/google/models/gemini-2.5-pro:countTokens") {
		t.Fatalf("countTokens URL lost action suffix: %s", got)
	}
}
