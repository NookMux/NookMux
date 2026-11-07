package common

import "math"

func GetTrustQuota() int {
	return int(10 * QuotaPerUnit)
}

// QuotaFromFloat 将 float64 额度值饱和换算为 int：NaN 归零，其余钳制到
// [MinInt32, MaxInt32]。配额列（users.quota、logs.quota 等 gorm type:int）
// 为 32 位，直接裸转在溢出时会回绕为负数并沿负差额入账路径铸造余额；
// 饱和到上界的值会沿现有"余额不足"路径失败。
func QuotaFromFloat(value float64) int {
	if math.IsNaN(value) {
		return 0
	}
	if value >= math.MaxInt32 {
		return math.MaxInt32
	}
	if value <= math.MinInt32 {
		return math.MinInt32
	}
	return int(value)
}

// QuotaUpperLimit 返回 limitUnits 单位（美元）对应的额度上限。
// 乘积先按 float64 计算并钳制到当前平台 int 最大值：直接写
// int(limitUnits * QuotaPerUnit) 在 32 位平台（int 为 32 位）会溢出为
// 负数，使"超过上限"校验恒成立、合法额度被全量拒绝。
func QuotaUpperLimit(limitUnits float64) int {
	limit := limitUnits * QuotaPerUnit
	// float64(math.MaxInt) 在 64 位平台舍入为 2^63（已超出 MaxInt），
	// 用其下最近可表示值作钳制边界，避免 [2^63-1024, 2^63) 区间漏检。
	if limit >= math.Nextafter(float64(math.MaxInt), 0) {
		return math.MaxInt
	}
	return int(limit)
}
