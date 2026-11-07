package contract

import (
	"fmt"
	"math"
)

type GroupRatioInfo struct {
	GroupRatio        float64
	GroupSpecialRatio float64
	HasSpecialRatio   bool
	DynamicRatio      float64
}

type PriceData struct {
	FreeModel            bool // true when the model is genuinely free and no pre-consumption is needed
	ModelPrice           float64
	ModelRatio           float64
	CompletionRatio      float64
	CacheRatio           float64
	CacheCreationRatio   float64
	CacheCreation5mRatio float64
	CacheCreation1hRatio float64
	AudioRatio           float64
	AudioCompletionRatio float64
	OtherRatios          map[string]float64
	UsePrice             bool
	QuotaToPreConsume    int // 预消耗额度
	GroupRatioInfo       GroupRatioInfo
	ContextPricing       *ContextPricingResult
}

func (p *PriceData) AddOtherRatio(key string, ratio float64) {
	// 拒绝非正数与 NaN/Inf：NaN 的一切比较均为 false 会绕过 <=0 检查，
	// 毒化后续乘法；Inf 会让额度计算溢出回绕。
	if !(ratio > 0) || math.IsInf(ratio, 0) {
		return
	}
	if p.OtherRatios == nil {
		p.OtherRatios = make(map[string]float64)
	}
	p.OtherRatios[key] = ratio
}

type PerCallPriceData struct {
	ModelPrice     float64
	Quota          int
	GroupRatioInfo GroupRatioInfo
}

func (p *PriceData) ToSetting() string {
	return fmt.Sprintf("ModelPrice: %f, ModelRatio: %f, CompletionRatio: %f, CacheRatio: %f, GroupRatio: %f, UsePrice: %t, CacheCreationRatio: %f, CacheCreation5mRatio: %f, CacheCreation1hRatio: %f, QuotaToPreConsume: %d, AudioRatio: %f, AudioCompletionRatio: %f", p.ModelPrice, p.ModelRatio, p.CompletionRatio, p.CacheRatio, p.GroupRatioInfo.GroupRatio, p.UsePrice, p.CacheCreationRatio, p.CacheCreation5mRatio, p.CacheCreation1hRatio, p.QuotaToPreConsume, p.AudioRatio, p.AudioCompletionRatio)
}
