package common

import (
	"math"
	"testing"

	"github.com/shopspring/decimal"
)

func TestQuotaUpperLimitPositiveOnAllPlatforms(t *testing.T) {
	// 回归：int(limitUnits * QuotaPerUnit) 在 32 位平台溢出为负，
	// 使 token/user 的额度上限校验恒拒绝合法值。
	got := QuotaUpperLimit(1000000000)
	if got <= 0 {
		t.Fatalf("QuotaUpperLimit = %d, want > 0", got)
	}
	if got > math.MaxInt {
		t.Fatalf("QuotaUpperLimit = %d, want <= MaxInt(%d)", got, math.MaxInt)
	}
}

func TestQuotaUpperLimitClampsToMaxInt(t *testing.T) {
	old := QuotaPerUnit
	defer func() { QuotaPerUnit = old }()

	QuotaPerUnit = 1e18
	if got := QuotaUpperLimit(1e9); got != math.MaxInt {
		t.Fatalf("QuotaUpperLimit = %d, want MaxInt(%d)", got, math.MaxInt)
	}

	// 恰好落在 float64(MaxInt) 舍入边界上的值也必须钳制，
	// 不得进入越界 int() 转换（64 位平台窗口为 [2^63-1024, 2^63)）。
	boundary := math.Nextafter(float64(math.MaxInt), 0)
	QuotaPerUnit = boundary / 2
	if got := QuotaUpperLimit(2); got != math.MaxInt {
		t.Fatalf("QuotaUpperLimit = %d, want MaxInt(%d)", got, math.MaxInt)
	}
}

func TestQuotaFromFloat(t *testing.T) {
	cases := []struct {
		name  string
		value float64
		want  int
	}{
		{"normal value", 123456.789, 123456},
		{"negative value", -500.0, -500},
		{"zero", 0, 0},
		{"NaN maps to zero", math.NaN(), 0},
		{"positive infinity saturates", math.Inf(1), math.MaxInt32},
		{"negative infinity saturates", math.Inf(-1), math.MinInt32},
		{"above int32 saturates", 9e18, math.MaxInt32},
		{"below int32 saturates", -9e18, math.MinInt32},
		{"int32 boundary keeps", 2147483646.0, 2147483646},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := QuotaFromFloat(tc.value); got != tc.want {
				t.Fatalf("QuotaFromFloat(%v) = %d, want %d", tc.value, got, tc.want)
			}
		})
	}
}

func TestQuotaFromDecimal(t *testing.T) {
	cases := []struct {
		name  string
		value decimal.Decimal
		want  int
	}{
		{"rounds half up", decimal.NewFromFloat(1234.5), 1235},
		{"huge value saturates instead of wrapping", decimal.NewFromInt(9).Mul(decimal.NewFromInt(10).Pow(decimal.NewFromInt(18))), math.MaxInt32},
		{"huge negative saturates", decimal.NewFromInt(-9).Mul(decimal.NewFromInt(10).Pow(decimal.NewFromInt(18))), math.MinInt32},
		{"normal value", decimal.NewFromInt(4242), 4242},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := QuotaFromDecimal(tc.value); got != tc.want {
				t.Fatalf("QuotaFromDecimal(%s) = %d, want %d", tc.value.String(), got, tc.want)
			}
		})
	}
}
