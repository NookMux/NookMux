package payment

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"time"

	"github.com/NookMux/NookMux/internal/config/operation"
	"github.com/NookMux/NookMux/internal/infra/httpclient"
	"github.com/NookMux/NookMux/pkg/jsonx"
)

// epayQueryTimeout 是主动查询网关订单的单次请求超时。
const epayQueryTimeout = 15 * time.Second

// epayQueryMaxBodyBytes 是网关查单响应的读取上限，防止异常网关拖垮内存。
const epayQueryMaxBodyBytes = 64 << 10

// EpayOrderQueryResult 易支付 V1 查单接口（api.php?act=order）响应。
// code：1 为查询成功；status：1 为已支付、0 为未支付；money 为商品金额字符串。
type EpayOrderQueryResult struct {
	Code       int    `json:"code"`
	Msg        string `json:"msg"`
	TradeNo    string `json:"trade_no"`
	OutTradeNo string `json:"out_trade_no"`
	Type       string `json:"type"`
	Money      string `json:"money"`
	Status     int    `json:"status"`
	Endtime    string `json:"endtime"`
}

// QueryEpayOrder 向易支付网关主动查询订单支付状态（V1 协议，明文商户密钥鉴权）。
// 仅负责请求与响应解析，code/status 的业务判定由调用方完成。
// 商户密钥只出现在服务端到网关的请求中，调用方不得将其写入任何对外响应。
func QueryEpayOrder(outTradeNo string) (*EpayOrderQueryResult, error) {
	if outTradeNo == "" {
		return nil, errors.New("未提供订单号")
	}
	if operation.PayAddress == "" || operation.EpayId == "" || operation.EpayKey == "" {
		return nil, errors.New("易支付配置不完整")
	}

	u, err := url.Parse(operation.PayAddress)
	if err != nil {
		return nil, fmt.Errorf("易支付网关地址无效: %w", err)
	}
	query := url.Values{}
	query.Set("act", "order")
	query.Set("pid", operation.EpayId)
	query.Set("key", operation.EpayKey)
	query.Set("out_trade_no", outTradeNo)
	u.Path = path.Join(u.Path, "api.php")
	u.RawQuery = query.Encode()

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("构建易支付查单请求失败: %w", err)
	}

	client := httpclient.GetHttpClient()
	if client == nil {
		return nil, errors.New("HTTP 客户端未初始化")
	}
	ctx, cancel := context.WithTimeout(req.Context(), epayQueryTimeout)
	defer cancel()
	resp, err := client.Do(req.WithContext(ctx))
	if err != nil {
		return nil, fmt.Errorf("请求易支付网关失败: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("易支付网关响应状态码异常: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, epayQueryMaxBodyBytes))
	if err != nil {
		return nil, fmt.Errorf("读取易支付网关响应失败: %w", err)
	}
	var result EpayOrderQueryResult
	if err := jsonx.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("解析易支付网关响应失败: %w", err)
	}
	return &result, nil
}
