package customvoice

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/NookMux/NookMux/internal/common"
	configmodel "github.com/NookMux/NookMux/internal/config/model"
	"github.com/NookMux/NookMux/internal/config/ratio"
	httpclient "github.com/NookMux/NookMux/internal/infra/httpclient"
	"github.com/NookMux/NookMux/internal/store/channel"
	"github.com/NookMux/NookMux/internal/store/log"
	"github.com/NookMux/NookMux/internal/store/minimax_voice"
	"github.com/NookMux/NookMux/internal/store/user"
	"github.com/NookMux/NookMux/pkg/jsonx"
	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// 定制音色流程的常量与校验规则。
const (
	customVoiceMaxFileSize    = 20 << 20 // 20MB，与旧版一致
	customVoicePreviewTextMax = 2000
	customVoicePreviewTimeout = 90 * time.Second
	// customVoicePreviewTTL 试听记录保留时长：超过该时长未确认的“试听中”记录将被自动清理。
	// 业务规则：7 天内未确认的试听视为放弃，直接删除记录（不写审计日志）。
	customVoicePreviewTTL = 7 * 24 * time.Hour
)

var (
	customVoiceIDPattern = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_-]*[^-_]$`)

	// 允许上传的音频扩展名。旧版仅做前端 accept 限制，这里服务端强制校验。
	customVoiceAllowedExts = map[string]struct{}{
		".mp3": {},
		".m4a": {},
		".wav": {},
	}
)

// CustomVoicePreviewRequest 试听（上传并克隆）请求参数。
type CustomVoicePreviewRequest struct {
	Model                   string // 客户端选择的 TTS 模型，用于按所选模型计费
	VoiceId                 string // 用户填写的音色 ID
	PreviewText             string // 可选试听文本
	NeedNoiseReduction      bool
	NeedVolumeNormalization bool
}

// CustomVoicePreviewResult 试听返回结果。
type CustomVoicePreviewResult struct {
	VoiceId   string `json:"voice_id"`
	DemoAudio string `json:"demo_audio"` // 试听音频 URL（上游返回）
	FileId    string `json:"file_id"`    // 上游文件 ID（便于调试，不回传敏感信息）
	RecordId  int64  `json:"record_id"`  // 写入的试听中记录 ID
}

// CustomVoiceConfirmResult 确认定制返回结果。
type CustomVoiceConfirmResult struct {
	VoiceId string `json:"voice_id"`
	Status  string `json:"status"`
}

// CustomVoiceConfirmQuoteResult 确认定制报价返回结果。
type CustomVoiceConfirmQuoteResult struct {
	VoiceId   string `json:"voice_id"`
	QuotaCost int    `json:"quota_cost"`
}

type customVoiceConfirmContext struct {
	voiceId      string
	group        string
	billingModel string
	voice        *minimaxvoicestore.MiniMaxVoice
}

// customVoiceFileID preserves the upstream JSON type while exposing a safe
// string form for local records and frontend responses. MiniMax may return a
// numeric file_id from /files/upload, and voice_clone expects that ID to be
// sent back without changing its JSON type.
type customVoiceFileID struct {
	Display string
	Raw     json.RawMessage
}

func (id *customVoiceFileID) UnmarshalJSON(data []byte) error {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		id.Display = ""
		id.Raw = nil
		return nil
	}

	id.Raw = append(id.Raw[:0], trimmed...)
	if jsonx.GetJsonType(trimmed) == "string" {
		var value string
		if err := jsonx.Unmarshal(trimmed, &value); err != nil {
			return err
		}
		id.Display = strings.TrimSpace(value)
		return nil
	}
	if jsonx.GetJsonType(trimmed) == "number" {
		id.Display = string(trimmed)
		return nil
	}
	return fmt.Errorf("unsupported file_id json type: %s", jsonx.GetJsonType(trimmed))
}

func (id customVoiceFileID) String() string {
	return id.Display
}

func (id customVoiceFileID) IsZero() bool {
	return strings.TrimSpace(id.Display) == ""
}

func (id customVoiceFileID) upstreamValue() interface{} {
	if len(id.Raw) > 0 {
		return id.Raw
	}
	return id.Display
}

func resolveCustomVoiceCloneModel(modelName string) string {
	modelName = strings.TrimSpace(modelName)
	if modelName == "" {
		return ""
	}

	redirected := modelName
	_ = configmodel.WithMiniMaxSettingsReadLock(func() error {
		redirected = configmodel.ApplyMiniMaxModelRedirect(modelName, configmodel.GetMiniMaxSettings())
		return nil
	})
	redirected = strings.TrimSpace(redirected)
	if redirected == "" {
		redirected = modelName
	}
	return redirected
}

// validateCustomVoiceID 校验音色 ID：长度 8-256，字母开头，字母数字/下划线/连字符，不能以 _ - 结尾。
func validateCustomVoiceID(voiceId string) error {
	voiceId = strings.TrimSpace(voiceId)
	if len(voiceId) < 8 || len(voiceId) > 256 {
		return errors.New("音色ID不合规")
	}
	if !customVoiceIDPattern.MatchString(voiceId) {
		return errors.New("音色ID不合规")
	}
	return nil
}

// validateCustomVoiceFile 校验上传文件大小与扩展名。
func validateCustomVoiceFile(header *multipart.FileHeader) error {
	if header == nil {
		return errors.New("请上传音频文件")
	}
	if header.Size <= 0 {
		return errors.New("音频文件为空")
	}
	if header.Size > customVoiceMaxFileSize {
		return errors.New("音频文件过大，请压缩到 20MB 以内")
	}
	ext := strings.ToLower(filepath.Ext(header.Filename))
	if _, ok := customVoiceAllowedExts[ext]; !ok {
		return errors.New("仅支持 mp3、m4a、wav 格式")
	}
	return nil
}

// minimaxUpstream 是从渠道解析出的上游调用所需信息。
type minimaxUpstream struct {
	baseURL string
	apiKey  string
	groupId string
	channel *channelstore.Channel
}

// resolveMiniMaxUpstream 解析定制音色分组的上游 MiniMax 渠道信息。
// group 来自系统设置 CustomVoiceGroup；GroupId 从渠道 Other 字段读取（管理员填写）。
func resolveMiniMaxUpstream(group string) (*minimaxUpstream, error) {
	group = strings.TrimSpace(group)
	ch, err := minimaxvoicestore.GetEnabledMiniMaxChannelForGroup(group)
	if err != nil || ch == nil {
		return nil, errors.New("未找到可用的渠道，请联系管理员")
	}
	baseURL := ""
	if ch.BaseURL != nil {
		baseURL = strings.TrimRight(*ch.BaseURL, "/")
	}
	if baseURL == "" {
		baseURL = "https://api.minimaxi.com/v1"
	}
	// MiniMax GroupId 存放在渠道 Other 字段（管理员填写）。
	groupId := strings.TrimSpace(ch.Other)
	keys := ch.GetKeys()
	if len(keys) == 0 || keys[0] == "" {
		return nil, errors.New("渠道凭证缺失，请联系管理员")
	}
	return &minimaxUpstream{
		baseURL: baseURL,
		apiKey:  keys[0],
		groupId: groupId,
		channel: ch,
	}, nil
}

// doUpstreamRequest 执行上游 HTTP 请求并返回状态码与响应体。
// 必须走渠道配置的代理（GetHttpClientWithProxy），redirect 由受控 client
// 的 checkRedirect 复查，不允许裸 http.Client 出站。
// 通过 request context 保留 90s 超时（relay client 全局超时可能为 0）。
func doUpstreamRequest(up *minimaxUpstream, url string, contentType string, body io.Reader, apiKey string) (int, []byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), customVoicePreviewTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, body)
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	client, err := httpclient.GetHttpClientWithProxy(up.channel.GetSetting().Proxy)
	if err != nil {
		return 0, nil, errors.New("上游服务暂不可用，请稍后重试")
	}
	resp, err := client.Do(req)
	if err != nil {
		return 0, nil, errors.New("上游服务暂不可用，请稍后重试")
	}
	defer resp.Body.Close()
	data, readErr := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if readErr != nil {
		return resp.StatusCode, nil, errors.New("读取上游响应失败")
	}
	return resp.StatusCode, data, nil
}

// upstreamBaseResp 兼容 MiniMax 返回的 base_resp 结构，用于判定成功/失败。
type upstreamBaseResp struct {
	BaseResp struct {
		StatusCode int    `json:"status_code"`
		StatusMsg  string `json:"status_msg"`
	} `json:"base_resp"`
}

// normalizeUpstreamError 把上游错误转成面向用户的普通业务提示，不暴露渠道名。
// status 为 HTTP 状态码，rawBody 为上游响应体。
func normalizeUpstreamError(status int, rawBody []byte) error {
	msg := "音色处理失败，请稍后重试"
	if status >= 500 {
		return errors.New("上游服务繁忙，请稍后重试")
	}
	if status == http.StatusUnauthorized || status == http.StatusForbidden {
		return errors.New("服务凭证无效，请联系管理员")
	}
	if status == http.StatusTooManyRequests {
		return errors.New("请求过于频繁，请稍后重试")
	}
	// 尝试解析上游 base_resp，进一步精简提示，但绝不回传完整原始信息。
	var br upstreamBaseResp
	if err := jsonx.Unmarshal(rawBody, &br); err == nil {
		if br.BaseResp.StatusCode != 0 {
			inner := strings.TrimSpace(br.BaseResp.StatusMsg)
			if inner != "" {
				// 仅保留简短摘要，避免泄露内部细节。
				if len([]rune(inner)) > 40 {
					inner = string([]rune(inner)[:40]) + "..."
				}
				msg = "音色处理失败：" + inner
			}
		}
	}
	return errors.New(msg)
}

// calculateModelOnceQuota 计算确认定制阶段的按次扣费额度，但不扣费。
func calculateModelOnceQuota(modelName, group string) (int, error) {
	modelName = strings.TrimSpace(modelName)
	if modelName == "" {
		return 0, errors.New("计费模型未配置，请联系管理员")
	}
	groupRatio := resolveCustomVoiceGroupRatio(group, modelName)

	price, ok := ratio.GetModelPrice(modelName, false)
	usePrice := ok && price > 0
	quota := 0
	if usePrice {
		quota = int(price * common.QuotaPerUnit * groupRatio)
	} else {
		ratio, ratioOk, _ := ratio.GetModelRatio(modelName)
		if !ratioOk {
			// fail-closed：找不到计费配置时直接拒绝，避免无扣费使用。
			return 0, errors.New("所选计费模型暂不可用，请联系管理员")
		}
		// 按 1 千 token 计算一次调用的基准 quota（ratio 为每千 token 倍率）。
		quota = int(ratio * common.QuotaPerUnit * groupRatio)
	}
	if quota <= 0 {
		// 价格为 0 视为未正确配置，避免免费滥用。
		return 0, errors.New("所选计费模型价格无效，请联系管理员")
	}
	return quota, nil
}

// chargeModelOnce 按模型 ID 扣费一次（按次计费，ModelPrice 优先，乘以分组倍率）。
// 仅用于确认定制阶段：按系统设置的“音色定制”计费模型按次扣费。
//
// 计费原则（防越权/防漏扣）：
//   - ModelPrice 优先：按价格 * 分组倍率扣费。
//   - 否则尝试 ModelRatio：按每千 token 倍率 * 分组倍率折算一次基准 quota。
//   - 解析失败或价格为 0 则 fail closed，绝不“无扣费成功”。
//   - 仅扣减用户钱包额度（custom voice 流程无 token 上下文），并记录消费日志。
//   - userId/tokenName/channelId 仅用于日志展示。
//
// 返回扣减的 quota。
func chargeModelOnce(c *gin.Context, userId int, channelId int, modelName, group string) (int, error) {
	quota, err := calculateModelOnceQuota(modelName, group)
	if err != nil {
		return 0, err
	}

	// 预检用户余额，避免无效的上游调用与负数额度。
	remain, err := userstore.GetUserQuota(userId, true)
	if err != nil {
		return 0, errors.New("额度查询失败，请稍后重试")
	}
	if remain < quota {
		return 0, errors.New("额度不足，请充值后再试")
	}

	if err := userstore.DecreaseUserQuota(userId, quota); err != nil {
		return 0, errors.New("扣费失败，请稍后重试")
	}
	userstore.UpdateUserUsedQuotaAndRequestCount(userId, quota)
	if channelId > 0 {
		channelstore.UpdateChannelUsedQuota(channelId, quota)
	}

	// 记录消费日志（按 token 0 的方式记录一次调用）。
	tokenName := c.GetString("token_name")
	logstore.RecordConsumeLog(c, userId, logstore.RecordConsumeLogParams{
		ChannelId:        channelId,
		PromptTokens:     0,
		CompletionTokens: 0,
		ModelName:        modelName,
		TokenName:        tokenName,
		Quota:            quota,
		Content:          "音色定制确认消费",
		UseTimeMs:        0,
		IsStream:         false,
		Group:            group,
	})
	return quota, nil
}

// chargePreviewTTS 按正常 TTS 口径对试听文本计费：
//   - 按 ModelPrice 优先（按次）或 ModelRatio（按字符 usage）结算到所选试听模型；
//   - 字符 usage 同时映射到输入文本 token 与音频输出 token，与 relay 层 MiniMax TTS 一致，
//     让音频倍率计费分支同时计入文本成本和音频输出成本；
//   - 真正乘以 custom_voice_group 分组倍率与动态倍率；
//   - 失败时不落消费日志，由调用方决定是否退款。
//
// 用于定制音色试听阶段：试听模型（如 tts-3-turbo）只用于生成 demo_audio，
// 不应被记录为“音色定制”扣费。
//
// 返回扣减的 quota；返回 error 时不会扣减额度，也不会落消费日志。
func chargePreviewTTS(c *gin.Context, userId, channelId int, modelName, group, previewText string) (int, error) {
	modelName = strings.TrimSpace(modelName)
	if modelName == "" {
		return 0, errors.New("试听模型未配置，请联系管理员")
	}
	previewText = strings.TrimSpace(previewText)
	// 没有试听文本时不产生 TTS usage，不扣费（voice_clone 仍可只克隆不合成 demo）。
	if previewText == "" {
		return 0, nil
	}

	groupRatio := resolveCustomVoiceGroupRatio(group, modelName)

	price, ok := ratio.GetModelPrice(modelName, false)
	usePrice := ok && price > 0

	// usage_characters：试听合成的字符数，与 MiniMax TTS 计费口径一致。
	usageCharacters := len([]rune(previewText))

	var quota int
	if usePrice {
		quota = int(price * common.QuotaPerUnit * groupRatio)
	} else {
		modelRatio, ratioOk, _ := ratio.GetModelRatio(modelName)
		if !ratioOk {
			return 0, errors.New("试听计费模型暂不可用，请联系管理员")
		}
		audioRatio := ratio.GetAudioRatio(modelName)
		audioCompletionRatio := ratio.GetAudioCompletionRatio(modelName)

		inputTextTokens := decimal.NewFromInt(int64(usageCharacters))
		outputAudioTokens := decimal.NewFromInt(int64(usageCharacters))

		ratio := decimal.NewFromFloat(modelRatio * groupRatio)
		sum := decimal.Zero
		sum = sum.Add(inputTextTokens)
		sum = sum.Add(outputAudioTokens.Mul(decimal.NewFromFloat(audioRatio)).Mul(decimal.NewFromFloat(audioCompletionRatio)))
		quotaVal := sum.Mul(ratio)
		if !ratio.IsZero() && quotaVal.LessThanOrEqual(decimal.Zero) {
			quotaVal = decimal.NewFromInt(1)
		}
		quota = int(quotaVal.Round(0).IntPart())
	}
	if quota <= 0 {
		return 0, errors.New("试听计费模型价格无效，请联系管理员")
	}

	remain, err := userstore.GetUserQuota(userId, true)
	if err != nil {
		return 0, errors.New("额度查询失败，请稍后重试")
	}
	if remain < quota {
		return 0, errors.New("额度不足，请充值后再试")
	}

	if err := userstore.DecreaseUserQuota(userId, quota); err != nil {
		return 0, errors.New("扣费失败，请稍后重试")
	}
	userstore.UpdateUserUsedQuotaAndRequestCount(userId, quota)
	if channelId > 0 {
		channelstore.UpdateChannelUsedQuota(channelId, quota)
	}

	tokenName := c.GetString("token_name")
	logContent := fmt.Sprintf("定制音色试听，字符数 %d，分组倍率 %.4f", usageCharacters, groupRatio)
	if usePrice {
		logContent = fmt.Sprintf("定制音色试听，模型价格 %.4f，分组倍率 %.4f", price, groupRatio)
	}
	// 定制音色按字符计费，没有 Token 用量：不传伪 token 聚合（历史版本曾把
	// 字符数填进聚合列），保持 billing_details 与统计口径的“无 Token 用量”语义。
	logstore.RecordConsumeLog(c, userId, logstore.RecordConsumeLogParams{
		ChannelId: channelId,
		ModelName: modelName,
		TokenName: tokenName,
		Quota:     quota,
		Content:   logContent,
		UseTimeMs: 0,
		IsStream:  false,
		Group:     group,
	})
	return quota, nil
}

// resolveCustomVoiceGroupRatio 解析定制音色分组的最终倍率：
// 分组倍率 * 动态倍率（若配置）。与 relay 层 HandleGroupRatio 的口径保持一致，
// 但定制音色走 UserAuth（无 token/用户分组上下文），因此只按 custom_voice_group 解析。
func resolveCustomVoiceGroupRatio(group, modelName string) float64 {
	groupRatio := ratio.GetGroupRatio(group)
	if dynamicRatio := channelstore.GetMatchedDynamicRatio(group, modelName); dynamicRatio > 0 {
		groupRatio *= dynamicRatio
	}
	return groupRatio
}

// refundQuota 退还定制音色流程中已扣减的钱包额度。
// 仅在试听上游失败时作为尽力而为的补偿，不抛错给用户。
// 注意：已用额度/渠道统计是批量异步累加的，没有对称的回滚接口；
// 这里只恢复钱包余额，保证用户不会因上游失败白白损失额度。
func refundQuota(userId, quota int) {
	if quota <= 0 {
		return
	}
	if err := userstore.IncreaseUserQuota(userId, quota, false); err != nil {
		common.SysError(fmt.Sprintf("custom voice refund increase quota failed: userId=%d quota=%d err=%s", userId, quota, err.Error()))
	}
}

// CustomVoicePreview 执行“上传音频 + 生成试听”流程：
//  1. 校验文件、音色 ID；
//  2. 查重音色 ID（已存在则返回泛化的“不合规”提示）；
//  3. 上传文件到上游、调用 voice_clone 获取 demo_audio；
//  4. 试听成功后，按所选 TTS 模型按正常 TTS 口径计费（仅在有试听文本时）；
//  5. 写入“试听中”音色记录。
//
// 计费时序说明：
//   - 先调用上游拿到 demo_audio，再按试听文本字符数结算到试听模型；
//   - 上游失败时不扣费、不落消费日志；
//   - 试听扣费失败（余额不足等）时，已生成的 demo_audio 不返回，并向用户报错。
//
// 该函数不写审计日志（用户创建音色不审计）。
func CustomVoicePreview(c *gin.Context, userId int, req CustomVoicePreviewRequest, fileHeader *multipart.FileHeader) (*CustomVoicePreviewResult, error) {
	if err := validateCustomVoiceID(req.VoiceId); err != nil {
		return nil, err
	}
	if err := validateCustomVoiceFile(fileHeader); err != nil {
		return nil, err
	}
	if req.PreviewText != "" && len([]rune(req.PreviewText)) > customVoicePreviewTextMax {
		return nil, fmt.Errorf("试听文本过长，最多 %d 字", customVoicePreviewTextMax)
	}

	// 区域开关与渠道解析。
	if !isCustomVoiceConfigReady() {
		return nil, errors.New("定制音色功能未开启，请联系管理员")
	}
	group, _ := getCustomVoiceGroupAndBilling()
	upstream, err := resolveMiniMaxUpstream(group)
	if err != nil {
		return nil, err
	}

	// 先清理超过 7 天未确认的试听记录，确保过期音色 ID 可被重新使用（系统自动清理，不写审计）。
	if err := cleanupExpiredCustomVoicePreviews(); err != nil {
		return nil, err
	}

	// 查重：已存在则提示不合规（不暴露“重复”）。
	exists, err := minimaxvoicestore.IsMiniMaxVoiceIdExists(req.VoiceId)
	if err != nil {
		return nil, errors.New("音色校验失败，请稍后重试")
	}
	if exists {
		return nil, errors.New("音色ID不合规")
	}

	// 上传文件到上游。
	fileId, err := uploadFileUpstream(c, upstream, fileHeader)
	if err != nil {
		return nil, err
	}
	// 调用 voice_clone 生成 demo_audio。
	demoAudio, err := cloneVoiceUpstream(upstream, fileId, req)
	if err != nil {
		return nil, err
	}

	// 试听阶段按所选 TTS 模型按正常 TTS 口径计费（不是“音色定制”按次扣费）。
	// 没有试听文本时不产生 TTS usage，不扣费。
	previewQuota, err := chargePreviewTTS(c, userId, upstream.channel.Id, req.Model, group, req.PreviewText)
	if err != nil {
		return nil, err
	}

	// 写入“试听中”记录（用户创建，不审计）。
	voice := &minimaxvoicestore.MiniMaxVoice{
		Type:         minimaxvoicestore.MiniMaxVoiceTypePreview,
		OperatorId:   userId,
		OperatorKind: "user",
		VoiceId:      req.VoiceId,
		Allowed:      false,
	}
	if err := minimaxvoicestore.InsertMiniMaxVoice(voice); err != nil {
		// 唯一索引冲突也归一为“不合规”，避免暴露重复。
		if isDuplicateKeyErr(err) {
			// 记录写入失败时退还试听扣费，避免用户已付费却拿不到试听记录。
			refundQuota(userId, previewQuota)
			return nil, errors.New("音色ID不合规")
		}
		refundQuota(userId, previewQuota)
		return nil, errors.New("音色记录保存失败，请稍后重试")
	}

	return &CustomVoicePreviewResult{
		VoiceId:   req.VoiceId,
		DemoAudio: demoAudio,
		FileId:    fileId.String(),
		RecordId:  voice.Id,
	}, nil
}

// uploadFileUpstream 把用户上传的音频转发到上游文件接口，返回上游 file_id。
func uploadFileUpstream(c *gin.Context, up *minimaxUpstream, header *multipart.FileHeader) (customVoiceFileID, error) {
	src, err := header.Open()
	if err != nil {
		return customVoiceFileID{}, errors.New("音频文件读取失败")
	}
	defer src.Close()

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	// purpose=voice_clone
	if werr := writer.WriteField("purpose", "voice_clone"); werr != nil {
		return customVoiceFileID{}, errors.New("音频上传准备失败")
	}
	part, err := writer.CreateFormFile("file", header.Filename)
	if err != nil {
		return customVoiceFileID{}, errors.New("音频上传准备失败")
	}
	if _, err := io.Copy(part, src); err != nil {
		return customVoiceFileID{}, errors.New("音频上传准备失败")
	}
	if err := writer.Close(); err != nil {
		return customVoiceFileID{}, errors.New("音频上传准备失败")
	}

	url := up.baseURL + "/files/upload"
	if up.groupId != "" {
		url += "?" + buildGroupIdQuery(up.groupId)
	}
	status, body, err := doUpstreamRequest(up, url, writer.FormDataContentType(), &buf, up.apiKey)
	if err != nil {
		return customVoiceFileID{}, err
	}
	if status != http.StatusOK {
		return customVoiceFileID{}, normalizeUpstreamError(status, body)
	}
	var resp struct {
		File struct {
			FileId customVoiceFileID `json:"file_id"`
		} `json:"file"`
		upstreamBaseResp
	}
	if err := jsonx.Unmarshal(body, &resp); err != nil {
		return customVoiceFileID{}, errors.New("上游响应解析失败")
	}
	if resp.BaseResp.StatusCode != 0 || resp.File.FileId.IsZero() {
		return customVoiceFileID{}, normalizeUpstreamError(status, body)
	}
	return resp.File.FileId, nil
}

// cloneVoiceUpstream 调用上游 voice_clone 接口生成试听音频，返回 demo_audio URL。
func cloneVoiceUpstream(up *minimaxUpstream, fileId customVoiceFileID, req CustomVoicePreviewRequest) (string, error) {
	payload := buildVoiceClonePayload(fileId, req)
	bodyBytes, err := jsonx.Marshal(payload)
	if err != nil {
		return "", errors.New("请求构建失败")
	}

	url := up.baseURL + "/voice_clone"
	if up.groupId != "" {
		url += "?" + buildGroupIdQuery(up.groupId)
	}
	status, respBody, err := doUpstreamRequest(up, url, "application/json", bytes.NewReader(bodyBytes), up.apiKey)
	if err != nil {
		return "", err
	}
	if status != http.StatusOK {
		return "", normalizeUpstreamError(status, respBody)
	}
	var resp struct {
		DemoAudio string `json:"demo_audio"`
		upstreamBaseResp
	}
	if err := jsonx.Unmarshal(respBody, &resp); err != nil {
		return "", errors.New("上游响应解析失败")
	}
	if resp.BaseResp.StatusCode != 0 {
		return "", normalizeUpstreamError(status, respBody)
	}
	return resp.DemoAudio, nil
}

// buildVoiceClonePayload 构造发给 MiniMax voice_clone 接口的 payload。
//
// 当试听文本非空时，复用 MiniMax TTS 增强策略：
//   - 文本：移除情绪标签、替换语气词标签后的文本（policy.Text）。
//   - 模型：按模型重定向表映射后的上游模型名（policy.Model）。
//   - 情绪：识别到的情绪标签经 redirect 映射后的上游 emotion 值（policy.Emotion）。
//
// 用户看到和输入的始终是 redirect map 的 key（源标签）；
// 发给上游的是现有策略处理后的 value。策略未启用时原样透传用户输入。
func buildVoiceClonePayload(fileId interface{}, req CustomVoicePreviewRequest) map[string]interface{} {
	upstreamFileID := fileId
	if id, ok := fileId.(customVoiceFileID); ok {
		upstreamFileID = id.upstreamValue()
	}
	payload := map[string]interface{}{
		"file_id":                   upstreamFileID,
		"voice_id":                  req.VoiceId,
		"need_noise_reduction":      req.NeedNoiseReduction,
		"need_volume_normalization": req.NeedVolumeNormalization,
	}
	previewText := strings.TrimSpace(req.PreviewText)
	if previewText == "" {
		return payload
	}

	policy := configmodel.ApplyMiniMaxTTSPolicy(req.Model, "", previewText, "")
	payload["text"] = policy.Text
	// voice_clone 的 model 必须由管理员配置映射到 MiniMax 原生 speech-* ID。
	// 这里始终应用管理员重定向，但不内置兜底别名，避免配置不可控。
	payload["model"] = resolveCustomVoiceCloneModel(policy.Model)
	if policy.Emotion != "" {
		payload["emotion"] = policy.Emotion
	}
	return payload
}

func prepareCustomVoiceConfirm(userId int, voiceId string) (*customVoiceConfirmContext, error) {
	voiceId = strings.TrimSpace(voiceId)
	if voiceId == "" {
		return nil, errors.New("音色ID不能为空")
	}
	if !isCustomVoiceConfigReady() {
		return nil, errors.New("定制音色功能未开启，请联系管理员")
	}
	group, billingModel := getCustomVoiceGroupAndBilling()
	if billingModel == "" {
		return nil, errors.New("计费模型未配置，请联系管理员")
	}

	// 先清理超过 7 天未确认的试听记录：过期试听不能再确认（系统自动清理，不写审计）。
	if err := cleanupExpiredCustomVoicePreviews(); err != nil {
		return nil, err
	}

	// 必须命中本用户的试听中记录，防止越权报价或确认他人音色。
	voice, err := minimaxvoicestore.GetMiniMaxVoiceByVoiceId(voiceId)
	if err != nil || voice == nil {
		return nil, errors.New("音色ID不合规")
	}
	if voice.Type != minimaxvoicestore.MiniMaxVoiceTypePreview {
		return nil, errors.New("该音色无需确认或已处理")
	}
	if voice.OperatorId != userId {
		return nil, errors.New("无权操作该音色")
	}

	return &customVoiceConfirmContext{
		voiceId:      voiceId,
		group:        group,
		billingModel: billingModel,
		voice:        voice,
	}, nil
}

// CustomVoiceConfirmQuote 查询确认定制阶段应扣额度，只报价不扣费、不激活音色。
func CustomVoiceConfirmQuote(c *gin.Context, userId int, voiceId string) (*CustomVoiceConfirmQuoteResult, error) {
	confirmContext, err := prepareCustomVoiceConfirm(userId, voiceId)
	if err != nil {
		return nil, err
	}

	quotaCost, err := calculateModelOnceQuota(confirmContext.billingModel, confirmContext.group)
	if err != nil {
		return nil, err
	}

	return &CustomVoiceConfirmQuoteResult{
		VoiceId:   confirmContext.voiceId,
		QuotaCost: quotaCost,
	}, nil
}

// CustomVoiceConfirm 确认定制：按配置的扣费模型 ID 扣费，成功后把记录从试听中转为已创建。
//
// 安全要点：
//   - 必须命中本用户的“试听中”记录，防止越权确认他人音色。
//   - 一个“试听中”音色 ID 只能确认成功一次：通过条件更新（type=preview -> created）实现幂等。
//   - 扣费失败即中断，记录不会变为已创建。
//   - 扣费与状态流转使用一次性条件更新，避免旧实现里 DB.Save(voice) 把内存中残留的
//     preview 状态回写到数据库。
func CustomVoiceConfirm(c *gin.Context, userId int, voiceId string) (*CustomVoiceConfirmResult, error) {
	confirmContext, err := prepareCustomVoiceConfirm(userId, voiceId)
	if err != nil {
		return nil, err
	}

	// 确认定制阶段按配置的扣费模型 ID 扣费。失败即中断。
	quota, err := chargeModelOnce(c, userId, 0, confirmContext.billingModel, confirmContext.group)
	if err != nil {
		return nil, err
	}

	// 原子地把 preview -> created 并写入扣费额度，杜绝状态回滚风险。
	ok, err := minimaxvoicestore.ConfirmMiniMaxVoice(confirmContext.voice.Id, userId, quota)
	if err != nil {
		// 状态更新失败：尽力退还额度，避免无音色却扣费。
		refundQuota(userId, quota)
		return nil, errors.New("音色激活失败，已退还费用")
	}
	if !ok {
		// 并发场景下记录已不再是 preview（被他人确认或清理），退还本次扣费。
		refundQuota(userId, quota)
		return nil, errors.New("该音色无需确认或已处理")
	}

	return &CustomVoiceConfirmResult{
		VoiceId: confirmContext.voiceId,
		Status:  minimaxvoicestore.MiniMaxVoiceTypeCreated,
	}, nil
}

// isCustomVoiceConfigReady 检查定制音色区域是否开启。
// 分组允许为空（按默认分组路由），但开关必须打开。
func isCustomVoiceConfigReady() bool {
	return configmodel.IsCustomVoiceEnabled()
}

// cleanupExpiredCustomVoicePreviews 清理超过试听保留时长的“试听中”记录。
// 业务规则：试听阶段超过 customVoicePreviewTTL 仍未确认的记录视为放弃，直接删除。
// 该清理是系统自动行为，不走 controller 删除路径，因此不写审计日志。
// 清理失败时显式返回错误，避免后续查重/确认逻辑基于脏数据做出错误决策。
func cleanupExpiredCustomVoicePreviews() error {
	cutoff := time.Now().Add(-customVoicePreviewTTL).Unix()
	if _, err := minimaxvoicestore.DeleteExpiredMiniMaxVoicePreviews(cutoff); err != nil {
		return errors.New("音色校验失败，请稍后重试")
	}
	return nil
}

// getCustomVoiceGroupAndBilling 返回定制音色分组与扣费模型 ID。
func getCustomVoiceGroupAndBilling() (string, string) {
	group, billing := configmodel.GetCustomVoiceConfig()
	return strings.TrimSpace(group), strings.TrimSpace(billing)
}

// buildGroupIdQuery 构造上游 GroupId 查询串（已 URL 编码）。
// groupId 来自渠道 Other 字段（管理员填写），含 &/# 等字符时未编码会污染
// 上游请求参数，因此统一经 url.Values 编码后输出。
func buildGroupIdQuery(groupId string) string {
	q := url.Values{}
	q.Set("GroupId", groupId)
	return q.Encode()
}

// isDuplicateKeyErr 判断是否为唯一键冲突错误（跨数据库兼容）。
func isDuplicateKeyErr(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "duplicate") || strings.Contains(msg, "unique constraint")
}
