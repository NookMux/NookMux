package uptimekumacontroller

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/NookMux/NookMux/internal/config/console"
	"github.com/NookMux/NookMux/internal/config/dashboard"
	"github.com/NookMux/NookMux/pkg/jsonx"

	"github.com/gin-gonic/gin"
	"golang.org/x/sync/errgroup"
)

const (
	requestTimeout   = 30 * time.Second
	httpTimeout      = 10 * time.Second
	uptimeKeySuffix  = "_24"
	apiStatusPath    = "/api/status-page/"
	apiHeartbeatPath = "/api/status-page/heartbeat/"
)

type Monitor struct {
	Name   string  `json:"name"`
	Uptime float64 `json:"uptime"`
	Status int     `json:"status"`
	Group  string  `json:"group,omitempty"`
}

type UptimeGroupResult struct {
	CategoryName string    `json:"categoryName"`
	Monitors     []Monitor `json:"monitors"`
}

func getAndDecode(ctx context.Context, client *http.Client, url string, dest interface{}) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return errors.New("non-200 status")
	}

	return jsonx.DecodeJson(resp.Body, dest)
}

func fetchGroupData(ctx context.Context, client *http.Client, groupConfig map[string]interface{}) UptimeGroupResult {
	url, _ := groupConfig["url"].(string)
	slug, _ := groupConfig["slug"].(string)
	categoryName, _ := groupConfig["categoryName"].(string)

	result := UptimeGroupResult{
		CategoryName: categoryName,
		Monitors:     []Monitor{},
	}

	if url == "" || slug == "" {
		return result
	}

	baseURL := strings.TrimSuffix(url, "/")

	var statusData struct {
		PublicGroupList []struct {
			ID          int    `json:"id"`
			Name        string `json:"name"`
			MonitorList []struct {
				ID   int    `json:"id"`
				Name string `json:"name"`
			} `json:"monitorList"`
		} `json:"publicGroupList"`
	}

	var heartbeatData struct {
		HeartbeatList map[string][]struct {
			Status int `json:"status"`
		} `json:"heartbeatList"`
		UptimeList map[string]float64 `json:"uptimeList"`
	}

	g, gCtx := errgroup.WithContext(ctx)
	g.Go(func() error {
		return getAndDecode(gCtx, client, baseURL+apiStatusPath+slug, &statusData)
	})
	g.Go(func() error {
		return getAndDecode(gCtx, client, baseURL+apiHeartbeatPath+slug, &heartbeatData)
	})

	if g.Wait() != nil {
		return result
	}

	for _, pg := range statusData.PublicGroupList {
		if len(pg.MonitorList) == 0 {
			continue
		}

		for _, m := range pg.MonitorList {
			monitor := Monitor{
				Name:  m.Name,
				Group: pg.Name,
			}

			monitorID := strconv.Itoa(m.ID)

			if uptime, exists := heartbeatData.UptimeList[monitorID+uptimeKeySuffix]; exists {
				monitor.Uptime = uptime
			}

			if heartbeats, exists := heartbeatData.HeartbeatList[monitorID]; exists && len(heartbeats) > 0 {
				monitor.Status = heartbeats[0].Status
			}

			result.Monitors = append(result.Monitors, monitor)
		}
	}

	return result
}

// 运行状态快照缓存。TTL 来自仪表台配置 uptime_kuma_refresh_interval（秒），
// 下限 30s 与配置校验一致；监控分组变更时由 option 更新路径显式失效。
var uptimeCache = struct {
	sync.Mutex
	data      []UptimeGroupResult
	expiresAt time.Time
}{}

func uptimeCacheTTL() time.Duration {
	sec := max(dashboard.GetDashboardConfig().UptimeKumaRefreshInterval, 30)
	return time.Duration(sec) * time.Second
}

// ClearUptimeKumaCache 清空运行状态缓存，供监控分组或刷新间隔配置变更后失效调用。
func ClearUptimeKumaCache() {
	uptimeCache.Lock()
	uptimeCache.data = nil
	uptimeCache.expiresAt = time.Time{}
	uptimeCache.Unlock()
}

func GetUptimeKumaStatus(c *gin.Context) {
	uptimeCache.Lock()
	if uptimeCache.data != nil && time.Now().Before(uptimeCache.expiresAt) {
		data := uptimeCache.data
		uptimeCache.Unlock()
		c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": data})
		return
	}
	uptimeCache.Unlock()

	groups := console.GetUptimeKumaGroups()
	if len(groups) == 0 {
		ClearUptimeKumaCache()
		c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": []UptimeGroupResult{}})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), requestTimeout)
	defer cancel()

	client := &http.Client{Timeout: httpTimeout}
	results := make([]UptimeGroupResult, len(groups))

	g, gCtx := errgroup.WithContext(ctx)
	for i, group := range groups {
		g.Go(func() error {
			results[i] = fetchGroupData(gCtx, client, group)
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		// errgroup 中所有 goroutine 均显式 return nil，理论上不会到达；
		// 保留显式处理以备未来扩展并避免吞错。
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
		return
	}

	uptimeCache.Lock()
	uptimeCache.data = results
	uptimeCache.expiresAt = time.Now().Add(uptimeCacheTTL())
	uptimeCache.Unlock()

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": results})
}
