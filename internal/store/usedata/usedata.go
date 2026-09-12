package usedatastore

import (
	"fmt"
	"github.com/NookMux/NookMux/internal/common"
	billingcontract "github.com/NookMux/NookMux/internal/domain/billing/contract"
	"github.com/NookMux/NookMux/internal/store/db"
	"gorm.io/gorm"
	"sync"
	"time"
)

// 写入层聚合粒度配置。由 option.go 的 handleConfigUpdate 在 dashboard_config 更新时同步，
// 避免 model 包反向依赖 setting/dashboard_setting（dashboard_setting 依赖 model 完成迁移）。
// 默认全部启用，保持向后兼容。
var (
	quotaDataTrackTokens  = true
	quotaDataTrackByModel = true
	quotaDataTrackByUser  = true
)

// QuotaData 柱状图数据
type QuotaData struct {
	Id        int    `json:"id"`
	UserID    int    `json:"user_id" gorm:"index"`
	Username  string `json:"username" gorm:"index:idx_qdt_model_user_name,priority:2;size:64;default:''"`
	ModelName string `json:"model_name" gorm:"index:idx_qdt_model_user_name,priority:1;size:64;default:''"`
	CreatedAt int64  `json:"created_at" gorm:"bigint;index:idx_qdt_created_at,priority:2"`
	TokenUsed int    `json:"token_used" gorm:"default:0"`
	Count     int    `json:"count" gorm:"default:0"`
	FailCount int    `json:"fail_count" gorm:"default:0"`
	Quota     int    `json:"quota" gorm:"default:0"`
}

func UpdateQuotaData() {
	lastUpdatedAt := time.Time{}
	for {
		interval := time.Duration(common.DataExportInterval) * time.Minute
		if interval < time.Minute {
			interval = time.Minute
		}
		if common.DataExportEnabled && time.Since(lastUpdatedAt) >= interval {
			common.SysLog("正在更新数据看板数据...")
			SaveQuotaDataCache()
			lastUpdatedAt = time.Now()
		}
		// When data export is disabled there is nothing to do. Sleep in larger
		// increments (10s) instead of waking every second just to re-check a
		// boolean. The previous 1-second tick forced a goroutine wake-up and
		// cache-line bounce on every core, which is pure overhead for the
		// common deployment where data export is off.
		sleepDuration := time.Second
		if !common.DataExportEnabled {
			sleepDuration = 10 * time.Second
		} else if interval > 5*time.Minute {
			// When the flush interval is long, check at most every minute so
			// that enabling DataExportEnabled at runtime is picked up without
			// waiting the full interval.
			sleepDuration = time.Minute
		}
		time.Sleep(sleepDuration)
	}
}

var CacheQuotaData = make(map[string]*QuotaData)
var CacheQuotaDataLock = sync.Mutex{}

func logQuotaDataCache(userId int, username string, modelName string, quota int, createdAt int64, tokenUsed int) {
	key := fmt.Sprintf("%d-%s-%s-%d", userId, username, modelName, createdAt)
	quotaData, ok := CacheQuotaData[key]
	if ok {
		quotaData.Count += 1
		quotaData.Quota += quota
		quotaData.TokenUsed += tokenUsed
	} else {
		quotaData = &QuotaData{
			UserID:    userId,
			Username:  username,
			ModelName: modelName,
			CreatedAt: createdAt,
			Count:     1,
			Quota:     quota,
			TokenUsed: tokenUsed,
		}
	}
	CacheQuotaData[key] = quotaData
}

// applyTrackingConfig 根据写入层聚合粒度配置调整聚合维度和 token 记录。
// 禁用 by_user 时聚合到匿名桶（userId=0/username=""），禁用 by_model 时聚合到全局模型桶（modelName=""），
// 禁用 tokens 时强制 tokenUsed=0。返回调整后的维度值和 token 数。
func applyTrackingConfig(userId int, username string, modelName string, tokenUsed int) (int, string, string, int) {
	if !quotaDataTrackByUser {
		userId = 0
		username = ""
	}
	if !quotaDataTrackByModel {
		modelName = ""
	}
	if !quotaDataTrackTokens {
		tokenUsed = 0
	}
	return userId, username, modelName, tokenUsed
}

// SyncQuotaDataTrackingConfig 由 option 同步路径调用，把 dashboard_config 的写入层配置同步到 model 包级变量。
func SyncQuotaDataTrackingConfig(trackTokens, byModel, byUser bool) {
	quotaDataTrackTokens = trackTokens
	quotaDataTrackByModel = byModel
	quotaDataTrackByUser = byUser
}

// GetQuotaDataTrackingConfig 返回当前写入层聚合粒度配置，供测试在改写后还原。
func GetQuotaDataTrackingConfig() (trackTokens, byModel, byUser bool) {
	return quotaDataTrackTokens, quotaDataTrackByModel, quotaDataTrackByUser
}

// LogQuotaData 记录成功请求数据到内存缓存
func LogQuotaData(userId int, username string, modelName string, quota int, createdAt int64, tokenUsed int) {
	createdAt = createdAt - (createdAt % 3600)
	userId, username, modelName, tokenUsed = applyTrackingConfig(userId, username, modelName, tokenUsed)

	CacheQuotaDataLock.Lock()
	defer CacheQuotaDataLock.Unlock()
	logQuotaDataCache(userId, username, modelName, quota, createdAt, tokenUsed)
}

// LogQuotaErrorData 记录失败请求数据到内存缓存
func LogQuotaErrorData(userId int, username string, modelName string, createdAt int64) {
	createdAt = createdAt - (createdAt % 3600)
	userId, username, modelName, _ = applyTrackingConfig(userId, username, modelName, 0)

	CacheQuotaDataLock.Lock()
	defer CacheQuotaDataLock.Unlock()
	key := fmt.Sprintf("%d-%s-%s-%d", userId, username, modelName, createdAt)
	quotaData, ok := CacheQuotaData[key]
	if ok {
		quotaData.FailCount += 1
	} else {
		quotaData = &QuotaData{
			UserID:    userId,
			Username:  username,
			ModelName: modelName,
			CreatedAt: createdAt,
			Count:     0,
			Quota:     0,
			TokenUsed: 0,
			FailCount: 1,
		}
	}
	CacheQuotaData[key] = quotaData
}

func SaveQuotaDataCache() {
	CacheQuotaDataLock.Lock()
	defer CacheQuotaDataLock.Unlock()
	size := len(CacheQuotaData)
	// 如果缓存中有数据，就保存到数据库中
	// 1. 先查询数据库中是否有数据
	// 2. 如果有数据，就更新数据
	// 3. 如果没有数据，就插入数据
	for _, quotaData := range CacheQuotaData {
		quotaDataDB := &QuotaData{}
		dbstore.DB.Table("quota_data").Where("user_id = ? and username = ? and model_name = ? and created_at = ?",
			quotaData.UserID, quotaData.Username, quotaData.ModelName, quotaData.CreatedAt).First(quotaDataDB)
		if quotaDataDB.Id > 0 {
			increaseQuotaData(quotaData.UserID, quotaData.Username, quotaData.ModelName, quotaData.Count, quotaData.Quota, quotaData.CreatedAt, quotaData.TokenUsed, quotaData.FailCount)
		} else {
			dbstore.DB.Table("quota_data").Create(quotaData)
		}
	}
	CacheQuotaData = make(map[string]*QuotaData)
	common.SysLog(fmt.Sprintf("保存数据看板数据成功，共保存%d条数据", size))
}

func increaseQuotaData(userId int, username string, modelName string, count int, quota int, createdAt int64, tokenUsed int, failCount int) {
	err := dbstore.DB.Table("quota_data").Where("user_id = ? and username = ? and model_name = ? and created_at = ?",
		userId, username, modelName, createdAt).Updates(map[string]interface{}{
		"count":      gorm.Expr("count + ?", count),
		"quota":      gorm.Expr("quota + ?", quota),
		"token_used": gorm.Expr("token_used + ?", tokenUsed),
		"fail_count": gorm.Expr("fail_count + ?", failCount),
	}).Error
	if err != nil {
		common.SysLog(fmt.Sprintf("increaseQuotaData error: %s", err))
	}
}

const quotaDataAggregateSelect = "model_name, sum(count) as count, sum(quota) as quota, sum(token_used) as token_used, sum(fail_count) as fail_count, created_at"

func GetQuotaDataByUsername(username string, startTime int64, endTime int64) (quotaData []*QuotaData, err error) {
	var quotaDatas []*QuotaData
	err = dbstore.DB.Table("quota_data").
		Select(quotaDataAggregateSelect).
		Where("username = ? and created_at >= ? and created_at <= ?", username, startTime, endTime).
		Group("model_name, created_at").
		Find(&quotaDatas).Error
	return quotaDatas, err
}

func GetQuotaDataByUserId(userId int, startTime int64, endTime int64) (quotaData []*QuotaData, err error) {
	var quotaDatas []*QuotaData
	err = dbstore.DB.Table("quota_data").
		Select(quotaDataAggregateSelect).
		Where("user_id = ? and created_at >= ? and created_at <= ?", userId, startTime, endTime).
		Group("model_name, created_at").
		Find(&quotaDatas).Error
	return quotaDatas, err
}

func GetQuotaDataGroupByUser(startTime int64, endTime int64) (quotaData []*QuotaData, err error) {
	var quotaDatas []*QuotaData
	err = dbstore.DB.Table("quota_data").
		Select("username, created_at, sum(count) as count, sum(quota) as quota, sum(token_used) as token_used, sum(fail_count) as fail_count").
		Where("created_at >= ? and created_at <= ?", startTime, endTime).
		Group("username, created_at").
		Find(&quotaDatas).Error
	return quotaDatas, err
}

func GetAllQuotaDates(startTime int64, endTime int64, username string) (quotaData []*QuotaData, err error) {
	if username != "" {
		return GetQuotaDataByUsername(username, startTime, endTime)
	}
	var quotaDatas []*QuotaData
	// 从quota_data表中查询数据
	// only select model_name, sum(count) as count, sum(quota) as quota, model_name, created_at from quota_data group by model_name, created_at;
	//err = DB.Table("quota_data").Where("created_at >= ? and created_at <= ?", startTime, endTime).Find(&quotaDatas).Error
	err = dbstore.DB.Table("quota_data").Select(quotaDataAggregateSelect).Where("created_at >= ? and created_at <= ?", startTime, endTime).Group("model_name, created_at").Find(&quotaDatas).Error
	return quotaDatas, err
}

// QuotaStat 预聚合的统计数据
type QuotaStat struct {
	Quota        int `json:"quota"`
	Tpm          int `json:"tpm"`
	SuccessCount int `json:"success_count"`
	FailCount    int `json:"fail_count"`
}

// GetQuotaStatByUserId 从 quota_data 表查询指定用户的预聚合成功/失败次数
func GetQuotaStatByUserId(userId int, startTime int64, endTime int64) (QuotaStat, error) {
	var stat QuotaStat
	err := dbstore.DB.Table("quota_data").Select("coalesce(sum(quota), 0) as quota, coalesce(sum(token_used), 0) as tpm, coalesce(sum(count), 0) as success_count, coalesce(sum(fail_count), 0) as fail_count").
		Where("user_id = ? and created_at >= ? and created_at <= ?", userId, startTime, endTime).
		Scan(&stat).Error
	return stat, err
}

// GetQuotaStatByUsername 从 quota_data 表查询指定用户的预聚合成功/失败次数
func GetQuotaStatByUsername(username string, startTime int64, endTime int64) (QuotaStat, error) {
	var stat QuotaStat
	err := dbstore.DB.Table("quota_data").Select("coalesce(sum(quota), 0) as quota, coalesce(sum(token_used), 0) as tpm, coalesce(sum(count), 0) as success_count, coalesce(sum(fail_count), 0) as fail_count").
		Where("username = ? and created_at >= ? and created_at <= ?", username, startTime, endTime).
		Scan(&stat).Error
	return stat, err
}

// GetAllQuotaStat 从 quota_data 表查询所有用户的预聚合成功/失败次数
func GetAllQuotaStat(startTime int64, endTime int64) (QuotaStat, error) {
	var stat QuotaStat
	err := dbstore.DB.Table("quota_data").Select("coalesce(sum(quota), 0) as quota, coalesce(sum(token_used), 0) as tpm, coalesce(sum(count), 0) as success_count, coalesce(sum(fail_count), 0) as fail_count").
		Where("created_at >= ? and created_at <= ?", startTime, endTime).
		Scan(&stat).Error
	return stat, err
}

// RecalculateQuotaData 从 logs 表重新聚合指定时间范围的 quota_data（先删后插）
func RecalculateQuotaData(startTime int64, endTime int64) error {
	// 对齐到小时桶边界，避免部分小时数据不一致
	startTime = startTime - (startTime % 3600)
	endTime = endTime - (endTime % 3600) + 3599

	// 先从日志聚合数据（可失败的操作放在事务外）。
	// token_used 的生成来源是 billing_details（持久化 Token 用量的唯一权威
	// 来源），按唯一汇总公式还原每条消费记录的处理总量；列不存在（无用量）
	// 计 0，损坏 JSON 显式报错。
	type logRow struct {
		Id             int
		UserId         int
		Username       string
		ModelName      string
		CreatedAt      int64
		BillingDetails *string
		Quota          int
	}

	// 成功请求（type = 2）
	var successLogs []logRow
	err := dbstore.LOG_DB.Table("logs").
		Select("id, user_id, username, model_name, created_at, billing_details, quota").
		Where("type = 2 and created_at >= ? and created_at <= ?", startTime, endTime).
		Find(&successLogs).Error
	if err != nil {
		return fmt.Errorf("查询成功日志失败: %w", err)
	}
	successTokens := make(map[int]int, len(successLogs))
	for _, r := range successLogs {
		if r.BillingDetails == nil || *r.BillingDetails == "" {
			continue
		}
		payload, err := billingcontract.ParseBillingDetailsJSON(*r.BillingDetails)
		if err != nil {
			return fmt.Errorf("log id=%d: %w", r.Id, err)
		}
		processed, err := payload.ProcessedTotal()
		if err != nil {
			return fmt.Errorf("log id=%d: %w", r.Id, err)
		}
		successTokens[r.Id] = processed
	}

	// 失败请求（type = 5）
	type failRow struct {
		UserId    int
		Username  string
		ModelName string
		CreatedAt int64
	}
	var failLogs []failRow
	err = dbstore.LOG_DB.Table("logs").
		Select("user_id, username, model_name, created_at").
		Where("type = 5 and created_at >= ? and created_at <= ?", startTime, endTime).
		Find(&failLogs).Error
	if err != nil {
		return fmt.Errorf("查询失败日志失败: %w", err)
	}

	// 按 (userId, username, modelName, hourStart) 聚合
	type aggKey struct {
		UserId    int
		Username  string
		ModelName string
		HourStart int64
	}
	type aggVal struct {
		Count     int
		FailCount int
		Quota     int
		TokenUsed int
	}
	merged := make(map[aggKey]*aggVal)

	// 聚合成功日志
	for _, r := range successLogs {
		hourStart := r.CreatedAt - (r.CreatedAt % 3600)
		// 根据写入层聚合粒度配置调整 key 维度和 token 记录
		userId, username, modelName, tokenUsed := applyTrackingConfig(r.UserId, r.Username, r.ModelName, successTokens[r.Id])
		key := aggKey{UserId: userId, Username: username, ModelName: modelName, HourStart: hourStart}
		v, ok := merged[key]
		if !ok {
			v = &aggVal{}
			merged[key] = v
		}
		v.Count++
		v.Quota += r.Quota
		v.TokenUsed += tokenUsed
	}

	// 聚合失败日志
	for _, r := range failLogs {
		hourStart := r.CreatedAt - (r.CreatedAt % 3600)
		// 失败日志按同一配置聚合 key 维度（不记录 token）
		userId, username, modelName, _ := applyTrackingConfig(r.UserId, r.Username, r.ModelName, 0)
		key := aggKey{UserId: userId, Username: username, ModelName: modelName, HourStart: hourStart}
		v, ok := merged[key]
		if !ok {
			v = &aggVal{}
			merged[key] = v
		}
		v.FailCount++
	}

	// 构建批量插入数据
	batch := make([]*QuotaData, 0, len(merged))
	for k, v := range merged {
		batch = append(batch, &QuotaData{
			UserID:    k.UserId,
			Username:  k.Username,
			ModelName: k.ModelName,
			CreatedAt: k.HourStart,
			Count:     v.Count,
			FailCount: v.FailCount,
			Quota:     v.Quota,
			TokenUsed: v.TokenUsed,
		})
	}

	// 事务保护：先删后插
	err = dbstore.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Table("quota_data").Where("created_at >= ? and created_at <= ?", startTime, endTime).Delete(nil).Error; err != nil {
			return fmt.Errorf("删除旧 quota_data 失败: %w", err)
		}
		if len(batch) > 0 {
			if err := tx.Table("quota_data").CreateInBatches(batch, 100).Error; err != nil {
				return fmt.Errorf("插入 quota_data 失败: %w", err)
			}
		}
		return nil
	})
	if err != nil {
		return err
	}

	common.SysLog(fmt.Sprintf("重新计算数据看板完成，时间范围 %d~%d，共 %d 条记录", startTime, endTime, len(batch)))
	return nil
}
