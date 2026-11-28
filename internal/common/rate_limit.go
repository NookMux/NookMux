package common

import (
	"sync"
	"time"
)

type InMemoryRateLimiter struct {
	store              map[string]*[]int64
	mutex              sync.Mutex
	expirationDuration time.Duration
	stopCh             chan struct{}
}

// Init 幂等初始化限流器并按需启动清扫协程。重复调用是 no-op。
func (l *InMemoryRateLimiter) Init(expirationDuration time.Duration) {
	l.mutex.Lock()
	defer l.mutex.Unlock()
	if l.store != nil {
		return
	}
	l.store = make(map[string]*[]int64)
	l.expirationDuration = expirationDuration
	if expirationDuration > 0 {
		stopCh := make(chan struct{})
		l.stopCh = stopCh
		go l.clearExpiredItems(stopCh, expirationDuration)
	}
}

// clearExpiredItems 周期清理过期 key。清扫周期作为参数传入，避免协程内无锁
// 读 expirationDuration 造成数据竞争；删除逻辑持锁操作 store。
func (l *InMemoryRateLimiter) clearExpiredItems(stopCh chan struct{}, duration time.Duration) {
	ticker := time.NewTicker(duration)
	defer ticker.Stop()
	expirationSeconds := int64(duration.Seconds())
	for {
		select {
		case <-stopCh:
			return
		case <-ticker.C:
			l.removeExpired(expirationSeconds)
		}
	}
}

func (l *InMemoryRateLimiter) removeExpired(expirationSeconds int64) {
	l.mutex.Lock()
	defer l.mutex.Unlock()
	now := time.Now().Unix()
	for key, queue := range l.store {
		size := len(*queue)
		if size == 0 || now-(*queue)[size-1] > expirationSeconds {
			delete(l.store, key)
		}
	}
}

// Reset 停止清扫协程并清空状态，供测试或配置重载复用同一 limiter 实例时使用。
// 调用后可通过 Init 重新启动。生产路径一般不调用（limiter 进程级单例）。
func (l *InMemoryRateLimiter) Reset() {
	l.mutex.Lock()
	defer l.mutex.Unlock()
	if l.stopCh != nil {
		close(l.stopCh)
		l.stopCh = nil
	}
	l.store = nil
	l.expirationDuration = 0
}

// Request parameter duration's unit is seconds
func (l *InMemoryRateLimiter) Request(key string, maxRequestNum int, duration int64) bool {
	l.mutex.Lock()
	defer l.mutex.Unlock()
	// [old <-- new]
	queue, ok := l.store[key]
	now := time.Now().Unix()
	if ok {
		if len(*queue) < maxRequestNum {
			*queue = append(*queue, now)
			return true
		} else {
			if now-(*queue)[0] >= duration {
				*queue = (*queue)[1:]
				*queue = append(*queue, now)
				return true
			} else {
				return false
			}
		}
	} else {
		s := make([]int64, 0, maxRequestNum)
		l.store[key] = &s
		*(l.store[key]) = append(*(l.store[key]), now)
	}
	return true
}

// Peek 只判定不记录：按 Request 的规则报告 key 此刻能否再放行一次，即窗口里的记录达到 maxRequestNum
// 且其中最旧一条距今不足 duration 秒时拒绝；判定不改变窗口里的记录。供请求处理完毕后才计入的调用方
// 在处理前预判，放行的请求处理成功后用 Record 记入。maxRequestNum 为 0 表示不限制，由调用方跳过
// 判定与记录。
func (l *InMemoryRateLimiter) Peek(key string, maxRequestNum int, duration int64) bool {
	l.mutex.Lock()
	defer l.mutex.Unlock()
	// [old <-- new]
	queue, ok := l.store[key]
	return !ok || len(*queue) < maxRequestNum || time.Now().Unix()-(*queue)[0] >= duration
}

// Record 只记录不判定：把当前时刻记入 key 的窗口，窗口已满时先丢弃最旧一条，写入的记录与 Request
// 放行时写入的相同。
func (l *InMemoryRateLimiter) Record(key string, maxRequestNum int) {
	l.mutex.Lock()
	defer l.mutex.Unlock()
	// [old <-- new]
	queue, ok := l.store[key]
	if !ok {
		s := make([]int64, 0, maxRequestNum)
		queue = &s
		l.store[key] = queue
	} else if len(*queue) >= maxRequestNum {
		*queue = (*queue)[1:]
	}
	*queue = append(*queue, time.Now().Unix())
}
