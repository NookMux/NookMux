/**
此文件为旧版支付设置文件，如需增加新的参数、变量等，请在 payment_setting.go 中添加
This file is the old version of the payment settings file. If you need to add new parameters, variables, etc., please add them in payment_setting.go
*/

package operation

import (
	"errors"
	"fmt"
	"net/url"

	"github.com/NookMux/NookMux/pkg/jsonx"
)

var PayAddress = ""
var CustomCallbackAddress = ""
var EpayId = ""
var EpayKey = ""
var Price = 7.3
var MinTopUp = 1

// ValidatePayAddress 校验易支付网关地址：非空时 scheme 必须为 https。
// 易支付 V1 协议将商户密钥拼入下单/查单请求明文外发，http 网关或被劫持链路
// 会直接泄露密钥，进而被伪造 MD5 签名回调与查单响应；空值表示未启用易支付，允许保留。
func ValidatePayAddress(address string) error {
	if address == "" {
		return nil
	}
	u, err := url.Parse(address)
	if err != nil {
		return fmt.Errorf("易支付网关地址无效: %w", err)
	}
	if u.Scheme != "https" {
		return errors.New("易支付网关地址必须使用 https 协议")
	}
	return nil
}

var PayMethods = []map[string]string{
	{
		"name":  "支付宝",
		"color": "rgba(var(--semi-blue-5), 1)",
		"type":  "alipay",
	},
	{
		"name":  "微信",
		"color": "rgba(var(--semi-green-5), 1)",
		"type":  "wxpay",
	},
	{
		"name":      "自定义1",
		"color":     "black",
		"type":      "custom1",
		"min_topup": "50",
	},
}

func UpdatePayMethodsByJsonString(jsonString string) error {
	PayMethods = make([]map[string]string, 0)
	return jsonx.Unmarshal([]byte(jsonString), &PayMethods)
}

func PayMethods2JsonString() string {
	jsonBytes, err := jsonx.Marshal(PayMethods)
	if err != nil {
		return "[]"
	}
	return string(jsonBytes)
}

func ContainsPayMethod(method string) bool {
	for _, payMethod := range PayMethods {
		if payMethod["type"] == method {
			return true
		}
	}
	return false
}
