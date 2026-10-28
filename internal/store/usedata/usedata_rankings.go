package usedatastore

import (
	"fmt"
	infradb "github.com/NookMux/NookMux/internal/infra/db"
	"github.com/NookMux/NookMux/internal/store/db"
	"gorm.io/gorm"
)

type RankingQuotaTotal struct {
	ModelName   string `json:"model_name"`
	TotalTokens int64  `json:"total_tokens"`
}

type RankingQuotaBucket struct {
	ModelName string `json:"model_name"`
	Bucket    int64  `json:"bucket"`
	Tokens    int64  `json:"tokens"`
}

type RankingPlatformTotal struct {
	Platform    string `json:"platform"`
	TotalTokens int64  `json:"total_tokens"`
}

type RankingPlatformBucket struct {
	Platform string `json:"platform"`
	Bucket   int64  `json:"bucket"`
	Tokens   int64  `json:"tokens"`
}

func GetRankingQuotaTotals(startTime int64, endTime int64) ([]RankingQuotaTotal, error) {
	var rows []RankingQuotaTotal
	query := dbstore.DB.Table("quota_data").
		Select("model_name, sum(token_used) as total_tokens").
		Where("model_name <> ''").
		Group("model_name").
		Having("sum(token_used) > 0").
		Order("total_tokens DESC")
	query = applyRankingQuotaTimeRange(query, startTime, endTime)
	err := query.Find(&rows).Error
	return rows, err
}

// GetRankingQuotaBuckets 按模型和时间段聚合 token 用量。
// bucketSize 为桶大小（秒）；dayOffset 为天级桶在 Unix 纪元上的偏移（秒），
// 用于把 86400 桶边界从 UTC 对齐到指定时区的自然日（如 +8 时区传 8*3600）。
func GetRankingQuotaBuckets(startTime int64, endTime int64, bucketSize int64, dayOffset int64) ([]RankingQuotaBucket, error) {
	if bucketSize <= 0 {
		bucketSize = 3600
	}
	bucketExpr := rankingBucketExpr(bucketSize, dayOffset)
	var rows []RankingQuotaBucket
	query := dbstore.DB.Table("quota_data").
		Select(fmt.Sprintf("model_name, %s as bucket, sum(token_used) as tokens", bucketExpr)).
		Where("model_name <> ''").
		Group(fmt.Sprintf("model_name, %s", bucketExpr)).
		Having("sum(token_used) > 0").
		Order("bucket ASC")
	query = applyRankingQuotaTimeRange(query, startTime, endTime)
	err := query.Find(&rows).Error
	return rows, err
}

func rankingBucketExpr(bucketSize int64, dayOffset int64) string { // 整数除法是否向零截断由 infradb.IntegerDivisionTruncates 集中声明：
	// MySQL/openGauss/KingbaseES 不截断（返回小数），需 FLOOR 对齐桶边界；
	// PostgreSQL/SQLite/DM 截断，可直接整除。
	useFloorDivision := !infradb.IntegerDivisionTruncates()
	if dayOffset != 0 && bucketSize%86400 == 0 {
		// 天级及以上桶按指定偏移切分，使桶边界落在目标时区的自然日 00:00。
		offsetExpr := fmt.Sprintf("(%d)", dayOffset)
		if dayOffset > 0 {
			offsetExpr = fmt.Sprintf("(+%d)", dayOffset)
		}
		if useFloorDivision {
			return fmt.Sprintf("(FLOOR((created_at + %s) / %d) * %d) - %s", offsetExpr, bucketSize, bucketSize, offsetExpr)
		}
		return fmt.Sprintf("((((created_at + %s) / %d) * %d) - %s)", offsetExpr, bucketSize, bucketSize, offsetExpr)
	}
	if useFloorDivision {
		return fmt.Sprintf("FLOOR(created_at / %d) * %d", bucketSize, bucketSize)
	}
	return fmt.Sprintf("(created_at / %d) * %d", bucketSize, bucketSize)
}

func applyRankingQuotaTimeRange(query *gorm.DB, startTime int64, endTime int64) *gorm.DB {
	if startTime > 0 {
		query = query.Where("created_at >= ?", startTime)
	}
	if endTime > 0 {
		query = query.Where("created_at <= ?", endTime)
	}
	return query
}

// GetRankingPlatformTotals 按客户端平台聚合 token 用量，供热门 Agent 排行使用。
// 空串 platform 为未识别流量，不参与排行。
func GetRankingPlatformTotals(startTime int64, endTime int64) ([]RankingPlatformTotal, error) {
	var rows []RankingPlatformTotal
	query := dbstore.DB.Table("quota_data").
		Select("platform, sum(token_used) as total_tokens").
		Where("platform <> ''").
		Group("platform").
		Having("sum(token_used) > 0").
		Order("total_tokens DESC")
	query = applyRankingQuotaTimeRange(query, startTime, endTime)
	err := query.Find(&rows).Error
	return rows, err
}

// GetRankingPlatformBuckets 按客户端平台和时间段聚合 token 用量，
// bucketSize 与 dayOffset 语义同 GetRankingQuotaBuckets。
func GetRankingPlatformBuckets(startTime int64, endTime int64, bucketSize int64, dayOffset int64) ([]RankingPlatformBucket, error) {
	if bucketSize <= 0 {
		bucketSize = 3600
	}
	bucketExpr := rankingBucketExpr(bucketSize, dayOffset)
	var rows []RankingPlatformBucket
	query := dbstore.DB.Table("quota_data").
		Select(fmt.Sprintf("platform, %s as bucket, sum(token_used) as tokens", bucketExpr)).
		Where("platform <> ''").
		Group(fmt.Sprintf("platform, %s", bucketExpr)).
		Having("sum(token_used) > 0").
		Order("bucket ASC")
	query = applyRankingQuotaTimeRange(query, startTime, endTime)
	err := query.Find(&rows).Error
	return rows, err
}
