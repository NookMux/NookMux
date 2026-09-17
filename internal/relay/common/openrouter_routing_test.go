package common

import (
	"reflect"
	"testing"

	channelconstant "github.com/NookMux/NookMux/internal/domain/channel/constant"
	"github.com/NookMux/NookMux/internal/domain/shared"
	"github.com/NookMux/NookMux/pkg/jsonx"
	"github.com/tidwall/gjson"
)

func newOpenRouterRoutingInfo(channelType int, routing *shared.OpenRouterRouting) *RelayInfo {
	return &RelayInfo{ChannelMeta: &ChannelMeta{
		ChannelType:          channelType,
		ChannelOtherSettings: shared.ChannelOtherSettings{OpenRouterRouting: routing},
	}}
}

func TestApplyProviderRoutingStripsClientProviderForNonOpenRouterChannels(t *testing.T) {
	// 客户端 provider 只对 OpenRouter 上游有意义；其余渠道必须剥离，保持
	// DTO 加字段之前"未知字段被丢弃"的既有行为。
	body := []byte(`{"model":"gpt-4o","provider":{"order":["openai"]},"messages":[{"role":"user","content":"hi"}]}`)
	routing := &shared.OpenRouterRouting{DataCollection: "deny"}
	out, err := ApplyProviderRouting(body, newOpenRouterRoutingInfo(channelconstant.ChannelTypeOpenAI, routing))
	if err != nil {
		t.Fatalf("ApplyProviderRouting returned error: %v", err)
	}
	if gjson.GetBytes(out, "provider").Exists() {
		t.Fatalf("expected provider stripped for non-OpenRouter channel, got %s", string(out))
	}
	if !gjson.GetBytes(out, "model").Exists() {
		t.Fatalf("expected other fields untouched, got %s", string(out))
	}
}

// TestApplyProviderRoutingPipelineThroughChatRequestStruct 证明客户端 provider
// 能穿过类型化解析（GeneralOpenAIRequest）存活到注入点——这是字段级合并
// 在生产路径可达的前提。
func TestApplyProviderRoutingPipelineThroughChatRequestStruct(t *testing.T) {
	clientJSON := []byte(`{"model":"openai/gpt-4o","messages":[{"role":"user","content":"hi"}],"provider":{"order":["openai"],"ignore":["bad-provider"]}}`)

	request := &shared.GeneralOpenAIRequest{}
	if err := jsonx.Unmarshal(clientJSON, request); err != nil {
		t.Fatalf("unmarshal client request: %v", err)
	}
	if len(request.Provider) == 0 {
		t.Fatalf("expected GeneralOpenAIRequest to capture client provider field")
	}

	jsonData, err := jsonx.Marshal(request)
	if err != nil {
		t.Fatalf("marshal converted request: %v", err)
	}

	routing := &shared.OpenRouterRouting{
		Order:          []string{"deepinfra"},
		DataCollection: "deny",
	}
	out, err := ApplyProviderRouting(jsonData, newOpenRouterRoutingInfo(channelconstant.ChannelTypeOpenRouter, routing))
	if err != nil {
		t.Fatalf("ApplyProviderRouting returned error: %v", err)
	}

	provider := decodeProvider(t, out)
	if !reflect.DeepEqual(provider["order"], []interface{}{"deepinfra"}) {
		t.Fatalf("expected channel order to override client order, got %v", provider["order"])
	}
	if provider["data_collection"] != "deny" {
		t.Fatalf("expected channel data_collection, got %v", provider["data_collection"])
	}
	if !reflect.DeepEqual(provider["ignore"], []interface{}{"bad-provider"}) {
		t.Fatalf("expected client ignore list to survive the typed pipeline, got %v", provider["ignore"])
	}
}

// TestApplyProviderRoutingPipelineThroughClaudeRequestStruct 覆盖 Claude 原生
// 透传路径：ClaudeRequest 捕获客户端 provider 后在 OpenRouter 渠道上参与合并。
func TestApplyProviderRoutingPipelineThroughClaudeRequestStruct(t *testing.T) {
	clientJSON := []byte(`{"model":"anthropic/claude-sonnet-4","max_tokens":64,"messages":[{"role":"user","content":"hi"}],"provider":{"data_collection":"allow"}}`)

	request := &shared.ClaudeRequest{}
	if err := jsonx.Unmarshal(clientJSON, request); err != nil {
		t.Fatalf("unmarshal client request: %v", err)
	}
	if len(request.Provider) == 0 {
		t.Fatalf("expected ClaudeRequest to capture client provider field")
	}

	jsonData, err := jsonx.Marshal(request)
	if err != nil {
		t.Fatalf("marshal converted request: %v", err)
	}

	routing := &shared.OpenRouterRouting{DataCollection: "deny"}
	out, err := ApplyProviderRouting(jsonData, newOpenRouterRoutingInfo(channelconstant.ChannelTypeOpenRouter, routing))
	if err != nil {
		t.Fatalf("ApplyProviderRouting returned error: %v", err)
	}

	provider := decodeProvider(t, out)
	if provider["data_collection"] != "deny" {
		t.Fatalf("expected channel data_collection to override client value, got %v", provider["data_collection"])
	}
}

func decodeProvider(t *testing.T, body []byte) map[string]interface{} {
	t.Helper()
	provider := gjson.GetBytes(body, "provider")
	if !provider.Exists() {
		t.Fatalf("provider field missing in body: %s", string(body))
	}
	var decoded map[string]interface{}
	if err := jsonx.Unmarshal([]byte(provider.Raw), &decoded); err != nil {
		t.Fatalf("failed to decode provider: %v", err)
	}
	return decoded
}

func TestApplyProviderRoutingSkipsEmptyRouting(t *testing.T) {
	body := []byte(`{"model":"openai/gpt-4o","provider":{"order":["openai"]}}`)
	out, err := ApplyProviderRouting(body, newOpenRouterRoutingInfo(channelconstant.ChannelTypeOpenRouter, nil))
	if err != nil {
		t.Fatalf("ApplyProviderRouting returned error: %v", err)
	}
	if string(out) != string(body) {
		t.Fatalf("expected body unchanged when routing is nil, got %s", string(out))
	}

	out, err = ApplyProviderRouting(body, newOpenRouterRoutingInfo(channelconstant.ChannelTypeOpenRouter, &shared.OpenRouterRouting{}))
	if err != nil {
		t.Fatalf("ApplyProviderRouting returned error: %v", err)
	}
	if string(out) != string(body) {
		t.Fatalf("expected body unchanged when routing is empty, got %s", string(out))
	}
}

func TestApplyProviderRoutingInjectsWhenClientProviderAbsent(t *testing.T) {
	body := []byte(`{"model":"anthropic/claude-sonnet-4","messages":[{"role":"user","content":"hi"}]}`)
	falseVal := false
	routing := &shared.OpenRouterRouting{
		Order:          []string{"anthropic", "google-vertex"},
		AllowFallbacks: &falseVal,
	}
	out, err := ApplyProviderRouting(body, newOpenRouterRoutingInfo(channelconstant.ChannelTypeOpenRouter, routing))
	if err != nil {
		t.Fatalf("ApplyProviderRouting returned error: %v", err)
	}
	provider := decodeProvider(t, out)
	expected := map[string]interface{}{
		"order":           []interface{}{"anthropic", "google-vertex"},
		"allow_fallbacks": false,
	}
	if !reflect.DeepEqual(provider, expected) {
		t.Fatalf("expected provider %v, got %v", expected, provider)
	}
}

func TestApplyProviderRoutingMergesFieldByFieldWithChannelPriority(t *testing.T) {
	body := []byte(`{"model":"openai/gpt-4o","provider":{"order":["openai"],"data_collection":"allow","ignore":["bad-provider"],"session_fallback":"client-only"}}`)
	routing := &shared.OpenRouterRouting{
		Order:          []string{"deepinfra"},
		DataCollection: "deny",
	}
	out, err := ApplyProviderRouting(body, newOpenRouterRoutingInfo(channelconstant.ChannelTypeOpenRouter, routing))
	if err != nil {
		t.Fatalf("ApplyProviderRouting returned error: %v", err)
	}
	provider := decodeProvider(t, out)
	// Channel-configured fields win; client-only fields survive.
	if !reflect.DeepEqual(provider["order"], []interface{}{"deepinfra"}) {
		t.Fatalf("expected channel order to override client order, got %v", provider["order"])
	}
	if provider["data_collection"] != "deny" {
		t.Fatalf("expected channel data_collection to override client value, got %v", provider["data_collection"])
	}
	if !reflect.DeepEqual(provider["ignore"], []interface{}{"bad-provider"}) {
		t.Fatalf("expected client ignore list to survive, got %v", provider["ignore"])
	}
	if provider["session_fallback"] != "client-only" {
		t.Fatalf("expected unknown client fields to survive, got %v", provider["session_fallback"])
	}
}

func TestApplyProviderRoutingReplacesNestedObjectsWholesale(t *testing.T) {
	body := []byte(`{"model":"openai/gpt-4o","provider":{"max_price":{"prompt":5,"completion":10,"image":3}}}`)
	prompt := 1.0
	completion := 2.0
	routing := &shared.OpenRouterRouting{
		MaxPrice: &shared.OpenRouterMaxPrice{Prompt: &prompt, Completion: &completion},
	}
	out, err := ApplyProviderRouting(body, newOpenRouterRoutingInfo(channelconstant.ChannelTypeOpenRouter, routing))
	if err != nil {
		t.Fatalf("ApplyProviderRouting returned error: %v", err)
	}
	provider := decodeProvider(t, out)
	expected := map[string]interface{}{"prompt": 1.0, "completion": 2.0}
	if !reflect.DeepEqual(provider["max_price"], expected) {
		t.Fatalf("expected channel max_price to replace client object, got %v", provider["max_price"])
	}
}

func TestApplyProviderRoutingSupportsThresholdDualShape(t *testing.T) {
	numeric := &shared.OpenRouterThreshold{Value: 50}
	percentile := &shared.OpenRouterThreshold{Value: 60, Percentile: "p90"}
	routing := &shared.OpenRouterRouting{
		PreferredMinThroughput: numeric,
		PreferredMaxLatency:    percentile,
	}
	body := []byte(`{"model":"openai/gpt-4o"}`)
	out, err := ApplyProviderRouting(body, newOpenRouterRoutingInfo(channelconstant.ChannelTypeOpenRouter, routing))
	if err != nil {
		t.Fatalf("ApplyProviderRouting returned error: %v", err)
	}
	provider := decodeProvider(t, out)
	if provider["preferred_min_throughput"] != 50.0 {
		t.Fatalf("expected numeric threshold form, got %v", provider["preferred_min_throughput"])
	}
	expected := map[string]interface{}{"p90": 60.0}
	if !reflect.DeepEqual(provider["preferred_max_latency"], expected) {
		t.Fatalf("expected percentile threshold form, got %v", provider["preferred_max_latency"])
	}
}

func TestApplyProviderRoutingTreatsNullClientProviderAsAbsent(t *testing.T) {
	body := []byte(`{"model":"openai/gpt-4o","provider":null}`)
	routing := &shared.OpenRouterRouting{DataCollection: "deny"}
	out, err := ApplyProviderRouting(body, newOpenRouterRoutingInfo(channelconstant.ChannelTypeOpenRouter, routing))
	if err != nil {
		t.Fatalf("ApplyProviderRouting returned error: %v", err)
	}
	if decodeProvider(t, out)["data_collection"] != "deny" {
		t.Fatalf("expected routing injected over null provider, got %s", string(out))
	}
}

func TestApplyProviderRoutingRejectsNonObjectClientProvider(t *testing.T) {
	body := []byte(`{"model":"openai/gpt-4o","provider":["openai"]}`)
	routing := &shared.OpenRouterRouting{DataCollection: "deny"}
	_, err := ApplyProviderRouting(body, newOpenRouterRoutingInfo(channelconstant.ChannelTypeOpenRouter, routing))
	if err == nil {
		t.Fatalf("expected error for non-object client provider, got nil")
	}
}

func TestApplyProviderRoutingHandlesNilInfo(t *testing.T) {
	body := []byte(`{"model":"openai/gpt-4o"}`)
	out, err := ApplyProviderRouting(body, nil)
	if err != nil {
		t.Fatalf("ApplyProviderRouting returned error: %v", err)
	}
	if string(out) != string(body) {
		t.Fatalf("expected body unchanged for nil info, got %s", string(out))
	}
}
