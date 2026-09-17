package runtime

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestBoundedRelayPoolAppliesBackpressure(t *testing.T) {
	p := newBoundedRelayPool(1, 1)

	release := make(chan struct{})
	p.CtxGo(context.Background(), func() {
		<-release
	})
	p.CtxGo(context.Background(), func() {})

	submitted := make(chan struct{})
	go func() {
		p.CtxGo(context.Background(), func() {})
		close(submitted)
	}()

	select {
	case <-submitted:
		t.Fatal("third task submitted while worker and queue were full")
	case <-time.After(20 * time.Millisecond):
	}

	close(release)
	<-submitted
}

func TestBoundedRelayPoolRunsAllSubmittedTasks(t *testing.T) {
	const taskCount = 32
	p := newBoundedRelayPool(4, taskCount)

	var ran atomic.Int32
	var wg sync.WaitGroup
	wg.Add(taskCount)
	for i := 0; i < taskCount; i++ {
		p.CtxGo(context.Background(), func() {
			ran.Add(1)
			wg.Done()
		})
	}
	wg.Wait()

	if got := ran.Load(); got != taskCount {
		t.Fatalf("ran %d tasks, want %d", got, taskCount)
	}
}

func TestBoundedRelayPoolStartsWorkersLazily(t *testing.T) {
	p := newBoundedRelayPoolWithIdleTimeout(4, 4, 50*time.Millisecond)
	if got := p.activeWorkers(); got != 0 {
		t.Fatalf("activeWorkers = %d before submit, want 0", got)
	}

	done := make(chan struct{})
	p.CtxGo(context.Background(), func() {
		close(done)
	})
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("submitted task did not run")
	}
	if got := p.activeWorkers(); got == 0 {
		t.Fatal("worker exited before idle timeout")
	}
}

func TestBoundedRelayPoolWorkersExitWhenIdle(t *testing.T) {
	p := newBoundedRelayPoolWithIdleTimeout(2, 2, 10*time.Millisecond)

	done := make(chan struct{})
	p.CtxGo(context.Background(), func() {
		close(done)
	})
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("submitted task did not run")
	}

	deadline := time.After(time.Second)
	for {
		if got := p.activeWorkers(); got == 0 {
			return
		}
		select {
		case <-deadline:
			t.Fatalf("workers did not exit after idle timeout, active=%d", p.activeWorkers())
		case <-time.After(5 * time.Millisecond):
		}
	}
}
