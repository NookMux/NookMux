package dbstore

import (
	"fmt"

	"github.com/NookMux/NookMux/internal/common"
)

// CheckNegativeQuotaBalances 启动时扫描余额与令牌额度的负值存量。
// 准入型扣减（用户余额、令牌剩余额度、窗口/周期额度）均走条件 UPDATE，
// 正常运行不会把额度扣成负数；此处告警用于暴露历史版本遗留的透支数据
// 或绕过扣减接口的异常写入，避免超卖问题静默存在。
func CheckNegativeQuotaBalances() {
	type negativeRow struct {
		Id    int
		Value int
	}

	var users []negativeRow
	if err := DB.Table("users").Select("id AS id, quota AS value").Where("quota < 0").Find(&users).Error; err != nil {
		common.SysError("check negative user quota failed: " + err.Error())
		return
	}
	for _, u := range users {
		common.SysError(fmt.Sprintf("negative user quota detected: user_id=%d quota=%d, please correct manually", u.Id, u.Value))
	}
	if len(users) > 0 {
		common.SysError(fmt.Sprintf("total %d user(s) with negative quota", len(users)))
	}

	var tokens []negativeRow
	if err := DB.Table("tokens").
		Select("id AS id, remain_quota AS value").
		Where("remain_quota < 0 OR window_used_quota > window_quota OR cycle_used_quota > cycle_quota").
		Find(&tokens).Error; err != nil {
		common.SysError("check negative token quota failed: " + err.Error())
		return
	}
	for _, tk := range tokens {
		common.SysError(fmt.Sprintf("negative token quota detected: token_id=%d remain_quota=%d, please correct manually", tk.Id, tk.Value))
	}
	if len(tokens) > 0 {
		common.SysError(fmt.Sprintf("total %d token(s) with negative quota", len(tokens)))
	}
}
