package common

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestRelayInfo_ResetForAttempt(t *testing.T) {
	t0 := time.Now().Add(-10 * time.Second)
	info := &RelayInfo{
		StartTime:             t0,
		FirstResponseTime:     t0.Add(500 * time.Millisecond),
		isFirstResponse:       false,
		ReceivedResponseCount: 15,
		SendResponseCount:     5,
		GeminiConvertInfo: &GeminiConvertInfo{
			ToolCallArguments: map[int]map[int]string{0: {0: "arg"}},
			ToolCallNames:     map[int]map[int]string{0: {0: "func"}},
			ToolCallIDs:       map[int]map[int]string{0: {0: "call_1"}},
		},
	}

	require.True(t, info.HasSendResponse())

	t1 := time.Now()
	info.ResetForAttempt(t1)

	require.Equal(t, t1, info.StartTime)
	require.Equal(t, t1.Add(-time.Second), info.FirstResponseTime)
	require.True(t, info.isFirstResponse)
	require.Equal(t, 0, info.ReceivedResponseCount)
	require.Equal(t, 0, info.SendResponseCount)
	require.False(t, info.HasSendResponse())
	require.Empty(t, info.GeminiConvertInfo.ToolCallArguments)
	require.Empty(t, info.GeminiConvertInfo.ToolCallNames)
	require.Empty(t, info.GeminiConvertInfo.ToolCallIDs)

	// In the new attempt, SetFirstResponseTime works again and records accurate timing.
	firstChunkTime := t1.Add(150 * time.Millisecond)
	info.FirstResponseTime = firstChunkTime
	info.isFirstResponse = false

	require.True(t, info.HasSendResponse())
	require.Equal(t, int64(150), info.FirstResponseTime.Sub(info.StartTime).Milliseconds())
}
