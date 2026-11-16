package perfmetricsstore

import (
	"testing"
	"time"
)

func resetPerfMetricsState(t *testing.T) {
	t.Helper()
	hotMu.Lock()
	hotBucket = map[bucketKey]*bucketCounters{}
	hotMu.Unlock()
	t.Cleanup(func() {
		hotMu.Lock()
		hotBucket = map[bucketKey]*bucketCounters{}
		hotMu.Unlock()
	})
}

func TestRecordBucketsByModelGroupAndAlignedBucketStart(t *testing.T) {
	resetPerfMetricsState(t)

	ts := time.Unix(1700000000, 0) // 任意非桶边界时刻
	Record("model-a", "vip", ts.Add(10*time.Second), Sample{Success: true, LatencyMs: 100, FirstTokenMs: 20, OutputTokens: 50, GenerationMs: 80})
	Record("model-a", "vip", ts.Add(20*time.Second), Sample{Success: false, LatencyMs: 300, FirstTokenMs: -1, OutputTokens: 0, GenerationMs: 300})
	Record("model-a", "default", ts.Add(30*time.Second), Sample{Success: true, LatencyMs: 50, FirstTokenMs: 10, OutputTokens: 10, GenerationMs: 40})
	Record("model-b", "vip", ts.Add(40*time.Second), Sample{Success: true, LatencyMs: 10, FirstTokenMs: -1, OutputTokens: 0, GenerationMs: 10})

	bucketStart := ts.Unix() - ts.Unix()%bucketSeconds()

	hotMu.Lock()
	defer hotMu.Unlock()
	if len(hotBucket) != 3 {
		t.Fatalf("expected 3 buckets (model×group), got %d", len(hotBucket))
	}
	vip := hotBucket[bucketKey{model: "model-a", group: "vip", bucketStart: bucketStart}]
	if vip == nil {
		t.Fatal("model-a/vip bucket missing")
	}
	if vip.requestCount != 2 || vip.successCount != 1 {
		t.Fatalf("unexpected counts: request=%d success=%d", vip.requestCount, vip.successCount)
	}
	if vip.totalLatencyMs != 400 {
		t.Fatalf("expected total latency 400, got %d", vip.totalLatencyMs)
	}
	if vip.totalFrtMs != 20 || vip.frtCount != 1 {
		t.Fatalf("expected frt sum 20 / count 1, got %d/%d", vip.totalFrtMs, vip.frtCount)
	}
	if vip.outputTokens != 50 || vip.generationMs != 380 {
		t.Fatalf("unexpected tokens/generation: %d/%d", vip.outputTokens, vip.generationMs)
	}
	if _, ok := hotBucket[bucketKey{model: "model-a", group: DefaultGroup, bucketStart: bucketStart}]; !ok {
		t.Fatal("empty group should fall back to default bucket")
	}
}

func TestRecordDropsDisabledOrEmptyModel(t *testing.T) {
	resetPerfMetricsState(t)

	SyncRuntimeConfig(false, 5, 3600, 30)
	t.Cleanup(func() { SyncRuntimeConfig(true, 5, 3600, 30) })

	Record("model-a", "vip", time.Now(), Sample{Success: true})
	if len(hotBucket) != 0 {
		t.Fatalf("disabled sampling should drop samples, got %d buckets", len(hotBucket))
	}

	SyncRuntimeConfig(true, 5, 3600, 30)
	Record("", "vip", time.Now(), Sample{Success: true})
	if len(hotBucket) != 0 {
		t.Fatalf("empty model name should drop samples, got %d buckets", len(hotBucket))
	}
}

func TestSyncRuntimeConfigClampsLowerBounds(t *testing.T) {
	SyncRuntimeConfig(true, 0, 0, -1)
	t.Cleanup(func() { SyncRuntimeConfig(true, 5, 3600, 30) })

	if got := flushInterval(); got != time.Minute {
		t.Fatalf("flush interval should clamp to 1 minute, got %s", got)
	}
	if got := bucketSeconds(); got != 60 {
		t.Fatalf("bucket seconds should clamp to 60, got %d", got)
	}
	if got := retentionDays(); got != -1 {
		t.Fatalf("retention should pass through negative sentinel, got %d", got)
	}
}

func TestBuildPerfSummary(t *testing.T) {
	if got := BuildPerfSummary(0, 0, 0, 0, 0); got != nil {
		t.Fatalf("no requests should yield nil summary, got %+v", got)
	}

	summary := BuildPerfSummary(3, 2, 900, 120, 2000)
	if summary == nil {
		t.Fatal("summary unexpectedly nil")
	}
	if summary.AvgLatencyMs != 300 {
		t.Fatalf("expected avg latency 300, got %d", summary.AvgLatencyMs)
	}
	// 2/3 = 66.666...% → 66.67
	if summary.SuccessRate != 66.67 {
		t.Fatalf("expected success rate 66.67, got %v", summary.SuccessRate)
	}
	// 120 tokens / 2s = 60 t/s
	if summary.AvgTps != 60 {
		t.Fatalf("expected avg tps 60, got %v", summary.AvgTps)
	}

	// 无生成耗时时吞吐为零值而不是除零
	zeroGen := BuildPerfSummary(1, 1, 100, 50, 0)
	if zeroGen == nil || zeroGen.AvgTps != 0 {
		t.Fatalf("expected zero tps without generation time, got %+v", zeroGen)
	}
}
