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
	_, allowed := l.admit(key, maxRequestNum, duration, time.Now().Unix())
	return allowed
}

// RateLimitReservation 是 Reserve 在窗口里占下的一条记录，对应一个开始处理的请求。
type RateLimitReservation struct {
	limiter *InMemoryRateLimiter
	key     string
	stamp   int64 // 占位时写入窗口的时刻（秒）
	settled bool
}

// Reserve parameter duration's unit is seconds
// Reserve 按 Request 的规则判定并记录一次请求，返回占下的记录；窗口已满时返回 nil。供请求处理完毕后
// 才知道成败的限额使用：处理前占位，处理完毕后由 Complete 保留或撤回。
func (l *InMemoryRateLimiter) Reserve(key string, maxRequestNum int, duration int64) *RateLimitReservation {
	l.mutex.Lock()
	defer l.mutex.Unlock()
	stamp, allowed := l.admit(key, maxRequestNum, duration, time.Now().Unix())
	if !allowed {
		return nil
	}
	return &RateLimitReservation{limiter: l, key: key, stamp: stamp}
}

// Complete 了结占位，只生效一次：succeeded 为 true 时记录留在窗口里，否则把这条记录撤回，失败的请求
// 不占额度。记录已离开窗口时没有可撤回的记录。nil 占位的 Complete 不做任何事。
func (r *RateLimitReservation) Complete(succeeded bool) {
	if r == nil || r.settled {
		return
	}
	r.settled = true
	if succeeded {
		return
	}
	r.limiter.withdraw(r.key, r.stamp)
}

// admit 把一次放行记入 key 的窗口并报告窗口是否放得下：记录未满时放行，记录已满时最旧一条离开窗口后
// 才再放行。调用方持有 l.mutex。
func (l *InMemoryRateLimiter) admit(key string, maxRequestNum int, duration int64, now int64) (stamp int64, allowed bool) {
	// [old <-- new]
	queue, ok := l.store[key]
	if !ok {
		records := make([]int64, 0, maxRequestNum)
		l.store[key] = &records
		queue = &records
	} else if len(*queue) >= maxRequestNum {
		if now-(*queue)[0] < duration {
			return 0, false
		}
		*queue = (*queue)[1:]
	}
	*queue = append(*queue, now)
	return now, true
}

// withdraw 撤回窗口里值为 stamp 的一条记录；记录已不在窗口里时不做任何事。
func (l *InMemoryRateLimiter) withdraw(key string, stamp int64) {
	l.mutex.Lock()
	defer l.mutex.Unlock()
	queue, ok := l.store[key]
	if !ok {
		return
	}
	for i, record := range *queue {
		if record == stamp {
			*queue = append((*queue)[:i], (*queue)[i+1:]...)
			return
		}
	}
}
