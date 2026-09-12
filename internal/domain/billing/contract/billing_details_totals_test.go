package contract

import "testing"

// totalsFixture 构造带缓存与多模态明细的 payload：
// 普通输入 = text 100 + image 200 = 300；输入侧总量 = 300 + read 300 + write 400 = 1000；
// 输出总量 = text 500 + audio 60 = 560；处理总量 = 1560。
// reasoning 50 是 text_output 子集、5m/1h 是 write_cache 子集，均不重复相加。
func totalsFixture() BillingDetailsPayload {
	return BillingDetailsPayload{
		SchemaVersion: BillingDetailsSchemaVersion,
		Tokens: BillingTokensDetail{
			Input: BillingInputTokens{
				TextInput:  100,
				ImageInput: 200,
			},
			Output: BillingOutputTokens{
				TextOutput:      500,
				AudioOutput:     60,
				ReasoningOutput: 50,
			},
			Cache: BillingCacheTokens{
				ReadCache:    300,
				WriteCache:   400,
				WriteCache5m: 400,
			},
		},
	}
}

func TestBillingDetailsTotalsFormula(t *testing.T) {
	payload := totalsFixture()

	ordinary, err := payload.OrdinaryInputTotal()
	if err != nil {
		t.Fatalf("OrdinaryInputTotal: %v", err)
	}
	if ordinary != 300 {
		t.Fatalf("ordinary input = %d, want 300", ordinary)
	}
	inputSide, err := payload.InputSideTotal()
	if err != nil {
		t.Fatalf("InputSideTotal: %v", err)
	}
	if inputSide != 1000 {
		t.Fatalf("input side total = %d, want 1000", inputSide)
	}
	output, err := payload.OutputTotal()
	if err != nil {
		t.Fatalf("OutputTotal: %v", err)
	}
	if output != 560 {
		t.Fatalf("output total = %d, want 560", output)
	}
	processed, err := payload.ProcessedTotal()
	if err != nil {
		t.Fatalf("ProcessedTotal: %v", err)
	}
	if processed != 1560 {
		t.Fatalf("processed total = %d, want 1560", processed)
	}
}

func TestBillingDetailsTotalsZeroPayload(t *testing.T) {
	payload := BillingDetailsPayload{SchemaVersion: BillingDetailsSchemaVersion}
	processed, err := payload.ProcessedTotal()
	if err != nil {
		t.Fatalf("ProcessedTotal on zero payload: %v", err)
	}
	if processed != 0 {
		t.Fatalf("processed total = %d, want 0", processed)
	}
}

func TestParseBillingDetailsJSONRejectsInvalid(t *testing.T) {
	cases := []struct {
		name string
		raw  string
	}{
		{name: "unknown schema version", raw: `{"schema_version":2,"tokens":{}}`},
		{name: "unknown field", raw: `{"schema_version":1,"tokens":{"input":{},"output":{},"cache":{},"extra":1}}`},
		{name: "missing group", raw: `{"schema_version":1,"tokens":{"input":{"text_input":1},"output":{}}}`},
		{name: "missing required field", raw: `{"schema_version":1,"tokens":{"input":{"text_input":1},"output":{},"cache":{}}}`},
		{name: "negative value", raw: `{"schema_version":1,"tokens":{"input":{"text_input":-1,"image_input":0,"audio_input":0,"video_input":0,"document_input":0},"output":{"text_output":0,"audio_output":0,"image_output":0,"reasoning_output":0,"accepted_prediction":0,"rejected_prediction":0},"cache":{"read_cache":0,"write_cache":0,"write_cache_5m":0,"write_cache_1h":0}}}`},
		{name: "tiered cache exceeds total", raw: `{"schema_version":1,"tokens":{"input":{"text_input":0,"image_input":0,"audio_input":0,"video_input":0,"document_input":0},"output":{"text_output":0,"audio_output":0,"image_output":0,"reasoning_output":0,"accepted_prediction":0,"rejected_prediction":0},"cache":{"read_cache":0,"write_cache":10,"write_cache_5m":20,"write_cache_1h":0}}}`},
		{name: "corrupt json", raw: `{`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if _, err := ParseBillingDetailsJSON(c.raw); err == nil {
				t.Fatalf("ParseBillingDetailsJSON(%s) succeeded, want explicit error", c.name)
			}
		})
	}
}

func TestParseBillingDetailsJSONCanonicalRoundTrip(t *testing.T) {
	payload := totalsFixture()
	raw := `{"schema_version":1,"tokens":{"input":{"text_input":100,"image_input":200,"audio_input":0,"video_input":0,"document_input":0},"output":{"text_output":500,"audio_output":60,"image_output":0,"reasoning_output":50,"accepted_prediction":0,"rejected_prediction":0},"cache":{"read_cache":300,"write_cache":400,"write_cache_5m":400,"write_cache_1h":0}}}`
	parsed, err := ParseBillingDetailsJSON(raw)
	if err != nil {
		t.Fatalf("parse canonical json: %v", err)
	}
	if parsed.Tokens != payload.Tokens {
		t.Fatalf("parsed tokens = %+v, want %+v", parsed.Tokens, payload.Tokens)
	}
}
