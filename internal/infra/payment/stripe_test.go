package payment

import (
	"strings"
	"testing"

	"github.com/NookMux/NookMux/pkg/jsonx"
	"github.com/stripe/stripe-go/v86"
)

func TestVerifySessionAmount(t *testing.T) {
	tests := []struct {
		name           string
		quantity       int64
		unitPrice      float64
		currency       string
		amountTotal    string
		amountDiscount string
		wantErr        string
	}{
		{
			name:        "金额一致通过",
			quantity:    10,
			unitPrice:   8.0,
			currency:    "usd",
			amountTotal: "8000",
		},
		{
			name:           "促销码折扣找平",
			quantity:       10,
			unitPrice:      8.0,
			currency:       "usd",
			amountTotal:    "7200",
			amountDiscount: "800",
		},
		{
			name:        "单价漂移拒绝入账",
			quantity:    10,
			unitPrice:   8.0,
			currency:    "usd",
			amountTotal: "5000",
			wantErr:     "不一致",
		},
		{
			name:        "零小数币种按主单位核对",
			quantity:    3,
			unitPrice:   100.0,
			currency:    "JPY",
			amountTotal: "300",
		},
		{
			name:           "零小数币种带折扣",
			quantity:       3,
			unitPrice:      100.0,
			currency:       "jpy",
			amountTotal:    "250",
			amountDiscount: "50",
		},
		{
			name:     "数量非法",
			quantity: 0,
			unitPrice: 8.0,
			currency: "usd",
			wantErr:  "数量非法",
		},
		{
			name:        "实付金额缺失",
			quantity:    3,
			unitPrice:   8.0,
			currency:    "usd",
			amountTotal: "",
			wantErr:     "缺失",
		},
		{
			name:           "促销折扣值非法",
			quantity:       3,
			unitPrice:      8.0,
			currency:       "usd",
			amountTotal:    "2400",
			amountDiscount: "abc",
			wantErr:        "非法",
		},
		{
			name:        "预期金额非整数最小单位",
			quantity:    3,
			unitPrice:   8.005,
			currency:    "usd",
			amountTotal: "2402",
			wantErr:     "非整数",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := VerifySessionAmount(tt.quantity, tt.unitPrice, tt.currency, tt.amountTotal, tt.amountDiscount)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("期望核对通过，实际报错: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("期望报错包含 %q，实际核对通过", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("错误信息未包含 %q: %v", tt.wantErr, err)
			}
		})
	}
}

func TestStripeEventAmountFieldExtraction(t *testing.T) {
	const payload = `{
		"id": "evt_test",
		"object": "event",
		"api_version": "2026-08-26.dahlia",
		"data": {
			"object": {
				"id": "cs_test",
				"object": "checkout_session",
				"client_reference_id": "ref_x",
				"amount_total": 8000,
				"currency": "usd",
				"total_details": {"amount_discount": 800}
			}
		}
	}`

	var event stripe.Event
	if err := jsonx.Unmarshal([]byte(payload), &event); err != nil {
		t.Fatalf("解析事件失败: %v", err)
	}

	if got := event.GetObjectValue("amount_total"); got != "8000" {
		t.Errorf("amount_total = %q, want 8000", got)
	}
	if got := event.GetObjectValue("currency"); got != "usd" {
		t.Errorf("currency = %q, want usd", got)
	}
	if got := event.GetObjectValue("total_details", "amount_discount"); got != "800" {
		t.Errorf("total_details.amount_discount = %q, want 800", got)
	}
	if got := event.GetObjectValue("total_details", "amount_shipping"); got != "" {
		t.Errorf("缺失的嵌套字段应返回空字符串, got %q", got)
	}
}
