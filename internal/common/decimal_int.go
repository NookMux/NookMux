package common

import (
	"fmt"
	"math"

	"github.com/shopspring/decimal"
)

// DecimalToInt 将 decimal 值转换为 int，超出当前平台 int 表示范围时返回错误。
//
// 直接写 int(d.IntPart()) 在 32 位构建（GOARCH=386/arm 等，int 为 32 位）会把
// 超过 2^31 的额度按低 32 位回绕：例如单笔充值 $8599 × QuotaPerUnit(500000)
// = 4,299,500,000 会静默入账 4,532,704（少入账）；超过 2^63 的乘积连 IntPart()
// 自身的 int64 都会回绕。因此必须先用 decimal 精确比较（IntPart 之前）拦截，
// 调用方收到错误后应立即失败，不得截断或兜底继续。
func DecimalToInt(d decimal.Decimal) (int, error) {
	if d.GreaterThan(decimal.NewFromInt(math.MaxInt)) || d.LessThan(decimal.NewFromInt(math.MinInt)) {
		return 0, fmt.Errorf("decimal value %s out of int range [%d, %d]", d.String(), math.MinInt, math.MaxInt)
	}
	return int(d.IntPart()), nil
}

// QuotaFromDecimal 将 decimal 额度终值饱和换算为 int：Round 到整数后按
// QuotaFromFloat 语义钳制到 32 位配额范围，超界值饱和到边界而非回绕。
func QuotaFromDecimal(d decimal.Decimal) int {
	return QuotaFromFloat(d.Round(0).InexactFloat64())
}
