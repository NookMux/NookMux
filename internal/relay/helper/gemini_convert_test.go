package helper

import (
	"testing"

	"github.com/NookMux/NookMux/internal/common"
	"github.com/NookMux/NookMux/internal/domain/channel/constant"
	"github.com/NookMux/NookMux/internal/domain/shared"
	relaycommon "github.com/NookMux/NookMux/internal/relay/common"
	"github.com/NookMux/NookMux/pkg/jsonx"
)

func TestResponseOpenAI2GeminiPreservesReasoningAndUsage(t *testing.T) {
	thinking := "thinking"
	resp := ResponseOpenAI2Gemini(&shared.OpenAITextResponse{
		Choices: []shared.OpenAITextResponseChoice{{
			Index: 0,
			Message: shared.Message{
				Role:               "assistant",
				Content:            "answer",
				ReasoningContent:   &thinking,
				ReasoningSignature: "sig_123",
			},
			FinishReason: "stop",
		}},
		Usage: shared.Usage{
			PromptTokens:     7,
			CompletionTokens: 5,
			TotalTokens:      12,
			CompletionTokenDetails: shared.OutputTokenDetails{
				ReasoningTokens: 2,
			},
		},
	}, nil)

	if resp.UsageMetadata.PromptTokenCount != 7 || resp.UsageMetadata.CandidatesTokenCount != 3 || resp.UsageMetadata.ThoughtsTokenCount != 2 {
		t.Fatalf("usage metadata = %+v, want prompt=7 candidates=3 thoughts=2", resp.UsageMetadata)
	}
	if len(resp.Candidates) != 1 || len(resp.Candidates[0].Content.Parts) != 2 {
		t.Fatalf("candidates = %+v, want one candidate with reasoning and text parts", resp.Candidates)
	}
	if !resp.Candidates[0].Content.Parts[0].Thought || resp.Candidates[0].Content.Parts[0].Text != "thinking" {
		t.Fatalf("first part = %+v, want thought part", resp.Candidates[0].Content.Parts[0])
	}
	if resp.Candidates[0].Content.Parts[0].GetThoughtSignature() != "sig_123" {
		t.Fatalf("thought signature = %q, want sig_123", resp.Candidates[0].Content.Parts[0].GetThoughtSignature())
	}
	if resp.Candidates[0].Content.Parts[1].Text != "answer" {
		t.Fatalf("second part = %+v, want text part", resp.Candidates[0].Content.Parts[1])
	}
}

func TestStreamResponseOpenAI2GeminiPreservesReasoningDelta(t *testing.T) {
	reasoning := "thinking"
	content := "answer"
	resp := StreamResponseOpenAI2Gemini(&shared.ChatCompletionsStreamResponse{
		Choices: []shared.ChatCompletionsStreamResponseChoice{{
			Index: 0,
			Delta: shared.ChatCompletionsStreamResponseChoiceDelta{
				ReasoningContent: &reasoning,
				Content:          &content,
			},
		}},
	}, &relaycommon.RelayInfo{})

	if resp == nil {
		t.Fatal("response is nil, want non-nil chunk")
	}
	if len(resp.Candidates) != 1 || len(resp.Candidates[0].Content.Parts) != 2 {
		t.Fatalf("candidates = %+v, want one candidate with reasoning and text parts", resp.Candidates)
	}
	if !resp.Candidates[0].Content.Parts[0].Thought || resp.Candidates[0].Content.Parts[0].Text != "thinking" {
		t.Fatalf("first part = %+v, want thought delta", resp.Candidates[0].Content.Parts[0])
	}
	if resp.Candidates[0].Content.Parts[1].Text != "answer" {
		t.Fatalf("second part = %+v, want text delta", resp.Candidates[0].Content.Parts[1])
	}
}

func TestStreamResponseOpenAI2GeminiBuffersToolCallArguments(t *testing.T) {
	info := &relaycommon.RelayInfo{}
	firstChunk := StreamResponseOpenAI2Gemini(&shared.ChatCompletionsStreamResponse{
		Choices: []shared.ChatCompletionsStreamResponseChoice{{
			Index: 0,
			Delta: shared.ChatCompletionsStreamResponseChoiceDelta{
				ToolCalls: []shared.ToolCallResponse{{
					Index: common.GetPointer(0),
					ID:    "call_1",
					Type:  "function",
					Function: shared.FunctionResponse{
						Name:      "weather",
						Arguments: `{"city":"Shang`,
					},
				}},
			},
		}},
	}, info)

	if firstChunk != nil {
		t.Fatalf("first chunk = %+v, want nil until tool arguments become valid JSON", firstChunk)
	}

	finishReason := "tool_calls"
	secondChunk := StreamResponseOpenAI2Gemini(&shared.ChatCompletionsStreamResponse{
		Choices: []shared.ChatCompletionsStreamResponseChoice{{
			Index: 0,
			Delta: shared.ChatCompletionsStreamResponseChoiceDelta{
				ToolCalls: []shared.ToolCallResponse{{
					Index: common.GetPointer(0),
					ID:    "call_1",
					Type:  "function",
					Function: shared.FunctionResponse{
						Arguments: `hai"}`,
					},
				}},
			},
			FinishReason: &finishReason,
		}},
	}, info)

	if secondChunk == nil || len(secondChunk.Candidates) != 1 || len(secondChunk.Candidates[0].Content.Parts) != 1 {
		t.Fatalf("second chunk = %+v, want one emitted function call", secondChunk)
	}
	part := secondChunk.Candidates[0].Content.Parts[0]
	if part.FunctionCall == nil || part.FunctionCall.FunctionName != "weather" {
		t.Fatalf("function call = %+v, want weather call", part.FunctionCall)
	}
	args, ok := part.FunctionCall.Arguments.(map[string]interface{})
	if !ok || args["city"] != "Shanghai" {
		t.Fatalf("function call args = %#v, want {city: Shanghai}", part.FunctionCall.Arguments)
	}
}

func TestGeminiToOpenAIRequestStopSequencesTruncateByActualLength(t *testing.T) {
	tests := []struct {
		name     string
		payload  string
		expected []string
	}{
		{
			name:     "single stop sequence",
			payload:  `{"contents":[{"role":"user","parts":[{"text":"hi"}]}],"generationConfig":{"stopSequences":["A"]}}`,
			expected: []string{"A"},
		},
		{
			name:     "two stop sequences",
			payload:  `{"contents":[{"role":"user","parts":[{"text":"hi"}]}],"generationConfig":{"stopSequences":["A","B"]}}`,
			expected: []string{"A", "B"},
		},
		{
			name:     "three stop sequences",
			payload:  `{"contents":[{"role":"user","parts":[{"text":"hi"}]}],"generationConfig":{"stopSequences":["A","B","C"]}}`,
			expected: []string{"A", "B", "C"},
		},
		{
			name:     "five stop sequences truncated to four",
			payload:  `{"contents":[{"role":"user","parts":[{"text":"hi"}]}],"generationConfig":{"stopSequences":["A","B","C","D","E"]}}`,
			expected: []string{"A", "B", "C", "D"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var geminiReq shared.GeminiChatRequest
			if err := jsonx.UnmarshalJsonStr(tt.payload, &geminiReq); err != nil {
				t.Fatalf("unmarshal gemini request failed: %v", err)
			}

			info := &relaycommon.RelayInfo{
				OriginModelName: "gemini-2.5-pro",
				ChannelMeta: &relaycommon.ChannelMeta{
					ChannelType:       constant.ChannelTypeGemini,
					UpstreamModelName: "gemini-2.5-pro",
				},
			}
			openaiReq, err := GeminiToOpenAIRequest(&geminiReq, info)
			if err != nil {
				t.Fatalf("GeminiToOpenAIRequest returned error: %v", err)
			}
			if openaiReq.Stop == nil {
				t.Fatal("stop is nil, want converted stop sequences")
			}
			got, ok := openaiReq.Stop.([]string)
			if !ok {
				t.Fatalf("stop type = %T, want []string", openaiReq.Stop)
			}
			if len(got) != len(tt.expected) {
				t.Fatalf("stop = %v, want %v", got, tt.expected)
			}
			for i := range tt.expected {
				if got[i] != tt.expected[i] {
					t.Fatalf("stop[%d] = %q, want %q", i, got[i], tt.expected[i])
				}
			}
		})
	}
}
