package perfmetrics

import (
	"errors"
	"testing"
	"time"

	"github.com/NookMux/NookMux/internal/domain/shared"
	relaycommon "github.com/NookMux/NookMux/internal/relay/common"
)

func TestClassifyOutcome(t *testing.T) {
	cases := []struct {
		name string
		err  *shared.NookMuxError
		want sampleOutcome
	}{
		{"success", nil, outcomeSuccess},
		{"sensitive words", shared.NewError(errors.New("blocked"), shared.ErrorCodeSensitiveWordsDetected), outcomeIgnored},
		{"insufficient quota", shared.NewError(errors.New("no quota"), shared.ErrorCodeInsufficientUserQuota), outcomeIgnored},
		{"pre-consume failed", shared.NewError(errors.New("fail"), shared.ErrorCodePreConsumeTokenQuotaFailed), outcomeIgnored},
		{"violation fee", shared.NewError(errors.New("fee"), shared.ErrorCodeViolationFeeGrokCSAM), outcomeIgnored},
		{"invalid request", shared.NewError(errors.New("bad"), shared.ErrorCodeInvalidRequest), outcomeIgnored},
		{"count token failed", shared.NewError(errors.New("boom"), shared.ErrorCodeCountTokenFailed), outcomeIgnored},
		{"channel error", shared.NewError(errors.New("oops"), shared.ErrorCodeChannelInvalidKey), outcomeFailure},
		{"upstream status", shared.NewError(errors.New("502"), shared.ErrorCodeBadResponseStatusCode), outcomeFailure},
		{"empty response", shared.NewError(errors.New("empty"), shared.ErrorCodeEmptyResponse), outcomeFailure},
		{"unknown passthrough code", shared.NewOpenAIError(errors.New("weird"), "billing_hard_limit_reached", 429), outcomeFailure},
		{"model not found", shared.NewError(errors.New("404"), shared.ErrorCodeModelNotFound), outcomeFailure},
	}
	for _, tc := range cases {
		if got := classifyOutcome(tc.err); got != tc.want {
			t.Errorf("%s: expected %d, got %d", tc.name, tc.want, got)
		}
	}
}

func TestBuildSampleTimings(t *testing.T) {
	start := time.Unix(1700000000, 0)

	// 流式且已发出首帧：frt 有效，生成阶段 = 总耗时 - frt
	streamed := &relaycommon.RelayInfo{
		StartTime:         start,
		FirstResponseTime: start.Add(300 * time.Millisecond),
	}
	frt, gen := buildSampleTimings(streamed, 2100)
	if frt != 300 {
		t.Fatalf("expected frt 300, got %d", frt)
	}
	if gen != 1800 {
		t.Fatalf("expected generation 1800, got %d", gen)
	}

	// 非流式（首帧时间早于开始时间的哨兵值）：无 frt，生成阶段退化为总耗时
	nonStream := &relaycommon.RelayInfo{
		StartTime:         start,
		FirstResponseTime: start.Add(-time.Second),
	}
	frt, gen = buildSampleTimings(nonStream, 1500)
	if frt != -1 {
		t.Fatalf("expected frt -1 for non-stream, got %d", frt)
	}
	if gen != 1500 {
		t.Fatalf("expected generation 1500, got %d", gen)
	}

	// 首帧延迟超过总耗时（异常时钟）钳为 0，不允许负吞吐分母
	overshoot := &relaycommon.RelayInfo{
		StartTime:         start,
		FirstResponseTime: start.Add(5 * time.Second),
	}
	frt, gen = buildSampleTimings(overshoot, 1000)
	if frt != 5000 {
		t.Fatalf("expected raw frt 5000, got %d", frt)
	}
	if gen != 0 {
		t.Fatalf("expected generation clamped to 0, got %d", gen)
	}
}
