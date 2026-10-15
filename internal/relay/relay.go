// Package relay is the entry facade of the relay layer. The actual
// implementations live in the domain-oriented subpackages:
//
//   - core/    adaptor dispatch, websocket relay and stored-asset signed URLs
//   - wire/    OpenAI wire protocol family (auto conversion, request/response
//     converters in wire/convert, stream writers in wire/stream)
//   - handler/ per-modality relay handlers
//
// The re-exports below keep the historical import path
// `github.com/NookMux/NookMux/internal/relay` stable for callers outside the
// relay layer (router, controllers).
package relay

import (
	"github.com/NookMux/NookMux/internal/domain/shared"
	"github.com/NookMux/NookMux/internal/relay/channel"
	relaycommon "github.com/NookMux/NookMux/internal/relay/common"
	"github.com/NookMux/NookMux/internal/relay/core"
	"github.com/NookMux/NookMux/internal/relay/handler"
	"github.com/NookMux/NookMux/internal/relay/wire"

	"github.com/gin-gonic/gin"
)

// GetAdaptor returns the provider adaptor for the given API type.
func GetAdaptor(apiType int) channel.Adaptor {
	return core.GetAdaptor(apiType)
}

// WssHelper relays a realtime websocket session.
func WssHelper(c *gin.Context, info *relaycommon.RelayInfo) (newAPIError *shared.NookMuxError) {
	return core.WssHelper(c, info)
}

// OpenAIWireHelper auto-converts between ChatCompletions and Responses based on channel setting openai_wire_api.
// It only applies to endpoints:
//   - /v1/chat/completions
//   - /v1/responses
//   - /v1/responses/compact (conversion not supported when chat-only)
func OpenAIWireHelper(c *gin.Context, info *relaycommon.RelayInfo) *shared.NookMuxError {
	return wire.OpenAIWireHelper(c, info)
}

// TextHelper relays OpenAI-compatible chat completions.
func TextHelper(c *gin.Context, info *relaycommon.RelayInfo) (newAPIError *shared.NookMuxError) {
	return handler.TextHelper(c, info)
}

// ClaudeHelper relays Anthropic Messages requests.
func ClaudeHelper(c *gin.Context, info *relaycommon.RelayInfo) (newAPIError *shared.NookMuxError) {
	return handler.ClaudeHelper(c, info)
}

// GeminiHelper relays Gemini generateContent requests.
func GeminiHelper(c *gin.Context, info *relaycommon.RelayInfo) (newAPIError *shared.NookMuxError) {
	return handler.GeminiHelper(c, info)
}

// GeminiEmbeddingHandler relays Gemini embedding requests.
func GeminiEmbeddingHandler(c *gin.Context, info *relaycommon.RelayInfo) (newAPIError *shared.NookMuxError) {
	return handler.GeminiEmbeddingHandler(c, info)
}

// GeminiCountTokensHandler relays Gemini countTokens requests.
func GeminiCountTokensHandler(c *gin.Context, info *relaycommon.RelayInfo) (newAPIError *shared.NookMuxError) {
	return handler.GeminiCountTokensHandler(c, info)
}

// AudioHelper relays audio transcription/translation/speech requests.
func AudioHelper(c *gin.Context, info *relaycommon.RelayInfo) (newAPIError *shared.NookMuxError) {
	return handler.AudioHelper(c, info)
}

// ImageHelper relays image generation requests.
func ImageHelper(c *gin.Context, info *relaycommon.RelayInfo) (newAPIError *shared.NookMuxError) {
	return handler.ImageHelper(c, info)
}

// EmbeddingHelper relays embedding requests.
func EmbeddingHelper(c *gin.Context, info *relaycommon.RelayInfo) (newAPIError *shared.NookMuxError) {
	return handler.EmbeddingHelper(c, info)
}

// RerankHelper relays rerank requests.
func RerankHelper(c *gin.Context, info *relaycommon.RelayInfo) (newAPIError *shared.NookMuxError) {
	return handler.RerankHelper(c, info)
}

// ResponsesHelper relays OpenAI Responses requests.
func ResponsesHelper(c *gin.Context, info *relaycommon.RelayInfo) (newAPIError *shared.NookMuxError) {
	return handler.ResponsesHelper(c, info)
}

// RelayStoredImage serves images persisted for "multimodal auto convert to URL".
func RelayStoredImage(c *gin.Context) {
	handler.RelayStoredImage(c)
}

// RelayStoredVideo serves videos persisted for "multimodal auto convert to URL".
func RelayStoredVideo(c *gin.Context) {
	handler.RelayStoredVideo(c)
}
