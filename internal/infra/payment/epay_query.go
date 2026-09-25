package payment

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/NookMux/NookMux/internal/config/operation"
	"github.com/NookMux/NookMux/internal/infra/httpclient"
	"github.com/NookMux/NookMux/pkg/jsonx"
)

// epayQueryTimeout 是主动查询网关订单的单次请求超时。
const epayQueryTimeout = 15 * time.Second

// epayQueryMaxBodyBytes 是网关查单响应的读取上限，防止异常网关拖垮内存。
const epayQueryMaxBodyBytes = 64 << 10

// epayQueryClient 独立于 relay 共享客户端，查单超时与重定向行为不受
// RELAY_TIMEOUT、FetchSetting 等 relay 专属配置间接影响。
var epayQueryClient = httpclient.NewGatewayHttpClient(epayQueryTimeout)

// epayFlexInt 兼容部分易支付兼容网关以字符串序列化的整数字段（如 "code":"1"）。
type epayFlexInt int

func (v *epayFlexInt) UnmarshalJSON(data []byte) error {
	n, err := parseEpayIntField(string(data))
	if err != nil {
		return err
	}
	*v = epayFlexInt(n)
	return nil
}

// epayFlexString 兼容部分易支付兼容网关以数字序列化的字符串字段（如 "money":1.0）。
type epayFlexString string

func (v *epayFlexString) UnmarshalJSON(data []byte) error {
	var s string
	if err := jsonx.Unmarshal(data, &s); err == nil {
		*v = epayFlexString(s)
		return nil
	}
	var f float64
	if err := jsonx.Unmarshal(data, &f); err != nil {
		return fmt.Errorf("易支付网关字段格式无效: %w", err)
	}
	*v = epayFlexString(strconv.FormatFloat(f, 'f', -1, 64))
	return nil
}

// parseEpayIntField 解析 JSON 整数字段，接受裸数字与字符串数字两种形式。
func parseEpayIntField(raw string) (int, error) {
	if n, err := strconv.Atoi(strings.TrimSpace(raw)); err == nil {
		return n, nil
	}
	var s string
	if err := jsonx.Unmarshal([]byte(raw), &s); err != nil {
		return 0, fmt.Errorf("易支付网关整数字段格式无效: %w", err)
	}
	n, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil {
		return 0, fmt.Errorf("易支付网关整数字段格式无效 %q: %w", s, err)
	}
	return n, nil
}

// EpayOrderQueryResult 易支付 V1 查单接口（api.php?act=order）响应。
// code：1 为查询成功；status：1 为已支付、0 为未支付；money 为商品金额。
// 整数与金额字段兼容字符串/数字两种序列化形式。
type EpayOrderQueryResult struct {
	Code       epayFlexInt    `json:"code"`
	Msg        string         `json:"msg"`
	TradeNo    string         `json:"trade_no"`
	OutTradeNo string         `json:"out_trade_no"`
	Type       string         `json:"type"`
	Money      epayFlexString `json:"money"`
	Status     epayFlexInt    `json:"status"`
	Endtime    string         `json:"endtime"`
}

// redactEpayURLError 去除传输层错误（*url.Error）内嵌的完整请求 URL 中的
// 商户密钥，避免密钥随错误信息进入日志。
func redactEpayURLError(err error) error {
	var urlErr *url.Error
	if !errors.As(err, &urlErr) {
		return err
	}
	if parsed, parseErr := url.Parse(urlErr.URL); parseErr == nil {
		query := parsed.Query()
		if query.Get("key") != "" {
			query.Set("key", "REDACTED")
			parsed.RawQuery = query.Encode()
			urlErr.URL = parsed.String()
		}
	}
	return err
}

// QueryEpayOrder 向易支付网关主动查询订单支付状态（V1 协议，明文商户密钥鉴权）。
// 仅负责请求与响应解析，code/status 的业务判定由调用方完成。
// 商户密钥只出现在服务端到网关的请求中，调用方不得将其写入任何对外响应；
// 返回的错误信息已对商户密钥脱敏。
func QueryEpayOrder(outTradeNo string) (*EpayOrderQueryResult, error) {
	if outTradeNo == "" {
		return nil, errors.New("未提供订单号")
	}
	if operation.PayAddress == "" || operation.EpayId == "" || operation.EpayKey == "" {
		return nil, errors.New("易支付配置不完整")
	}
	// 防御直改库等绕过选项保存校验的配置：商户密钥明文外发前强制复验网关为 https
	if err := operation.ValidatePayAddress(operation.PayAddress); err != nil {
		return nil, err
	}

	u, err := url.Parse(operation.PayAddress)
	if err != nil {
		return nil, fmt.Errorf("易支付网关地址无效: %w", err)
	}
	// 保留网关地址自带的 query 参数（如多租户网关的租户标识），与下单 URL 行为一致
	query := u.Query()
	query.Set("act", "order")
	query.Set("pid", operation.EpayId)
	query.Set("key", operation.EpayKey)
	query.Set("out_trade_no", outTradeNo)
	u.Path = path.Join(u.Path, "api.php")
	u.RawQuery = query.Encode()

	req, err := http.NewRequest(http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("构建易支付查单请求失败: %w", err)
	}

	resp, err := epayQueryClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求易支付网关失败: %w", redactEpayURLError(err))
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
	// 查单响应无签名，out_trade_no 回显是响应与所查订单的唯一绑定关系，必须核验
	if result.Code == 1 && result.OutTradeNo != outTradeNo {
		return nil, fmt.Errorf("易支付网关响应订单号不匹配: 请求 %s 响应 %s", outTradeNo, result.OutTradeNo)
	}
	return &result, nil
}
