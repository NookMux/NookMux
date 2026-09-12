package common

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/NookMux/NookMux/internal/common"
	"github.com/NookMux/NookMux/internal/config/model"
	"github.com/NookMux/NookMux/internal/domain/shared"
	relayconstant "github.com/NookMux/NookMux/internal/relay/constant"
	"github.com/NookMux/NookMux/internal/relay/wire/convert"

	"github.com/NookMux/NookMux/pkg/jsonx"

	"github.com/NookMux/NookMux/internal/domain/billing/contract"
	channelconstant "github.com/NookMux/NookMux/internal/domain/channel/constant"
	"github.com/NookMux/NookMux/internal/httpapi"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

const (
	LastMessageTypeNone     = "none"
	LastMessageTypeText     = "text"
	LastMessageTypeTools    = "tools"
	LastMessageTypeThinking = "thinking"
)

type ClaudeConvertInfo struct {
	LastMessagesType string
	Index            int
	Usage            *shared.Usage
	FinishReason     string
	Done             bool
	// PendingReasoningSignature buffers a Claude/OpenAI reasoning signature until
	// a thinking block is available to attach it to.
	PendingReasoningSignature string

	ToolCallBaseIndex      int
	ToolCallMaxIndexOffset int
}

type GeminiConvertInfo struct {
	ToolCallArguments map[int]map[int]string
	ToolCallNames     map[int]map[int]string
	ToolCallIDs       map[int]map[int]string
}

type RerankerInfo struct {
	Documents       []any
	ReturnDocuments bool
}

type BuildInToolInfo struct {
	ToolName          string
	CallCount         int
	SearchContextSize string
}

type ResponsesUsageInfo struct {
	BuiltInTools map[string]*BuildInToolInfo
}

type ChannelMeta struct {
	ChannelType          int
	ChannelId            int
	ChannelIsMultiKey    bool
	ChannelMultiKeyIndex int
	ChannelBaseUrl       string
	ApiType              int
	ApiVersion           string
	ApiKey               string
	Organization         string
	ChannelCreateTime    int64
	ParamOverride        map[string]interface{}
	HeadersOverride      map[string]interface{}
	ChannelSetting       shared.ChannelSettings
	ChannelOtherSettings shared.ChannelOtherSettings
	UpstreamModelName    string
	IsModelMapped        bool
	SupportStreamOptions bool // 是否支持流式选项
}

type TokenCountMeta struct {
	//promptTokens int
	estimatePromptTokens int
}

type RelayInfo struct {
	TokenId           int
	TokenKey          string
	TokenGroup        string
	TokenQuota        int
	TokenQuotaType    int
	UserId            int
	UsingGroup        string // 使用的分组，当auto跨分组重试时，会变动
	UserGroup         string // 用户所在分组
	TokenUnlimited    bool
	StartTime         time.Time
	FirstResponseTime time.Time
	isFirstResponse   bool
	//SendLastReasoningResponse bool
	IsStream               bool
	IsGeminiBatchEmbedding bool
	UsePrice               bool
	RelayMode              int
	OriginModelName        string
	// ResponseModelName preserves the model ID the client requested for downstream responses.
	ResponseModelName     string
	RequestURLPath        string
	ShouldIncludeUsage    bool
	DisablePing           bool // 是否禁止向下游发送自定义 Ping
	ClientWs              *websocket.Conn
	TargetWs              *websocket.Conn
	InputAudioFormat      string
	OutputAudioFormat     string
	RealtimeTools         []shared.RealTimeTool
	IsFirstRequest        bool
	AudioUsage            bool
	ReasoningEffort       string
	UserSetting           shared.UserSetting
	UserEmail             string
	UserQuota             int
	RelayFormat           relayconstant.RelayFormat
	SendResponseCount     int
	ReceivedResponseCount int
	FinalPreConsumedQuota int // 最终预消耗的配额
	// WssEventConsumedQuota realtime/WSS 会话内按事件实扣的资金累计，不含
	// 请求级初始预扣（后者由 Billing 会话单独持有并在收尾按 0 释放）。
	// PostWssConsumeQuota 收尾以此为补差基准：finalQuota 与该值的差额多退
	// 少补，保证日志 Quota 等于实际净扣款。只由计量结算 goroutine 写入，
	// 收尾在 join 结算 goroutine 之后读取。
	WssEventConsumedQuota int
	// Billing 是计费会话，封装了预扣费/结算/退款的统一生命周期。
	// 免费模型和按次计费（MJ/Task）时为 nil。
	Billing BillingSettler
	// BillingSource indicates where this request is billed from.
	// Subscription billing is removed; "" or "wallet" means wallet.
	BillingSource string
	// RequestId is used for idempotent pre-consume/refund
	RequestId                 string
	IsClaudeBetaQuery         bool // /v1/messages?beta=true
	IsChannelTest             bool // channel test request
	RuntimeHeadersOverride    map[string]interface{}
	UseRuntimeHeadersOverride bool
	ParamOverrideAudit        []string

	// UpstreamRequestBodySize is set when the marshaled upstream request body
	// is wrapped in BodyStorage so DoApiRequest can preserve Content-Length.
	UpstreamRequestBodySize int64

	PriceData contract.PriceData

	Request shared.Request

	// RequestConversionChain records request format conversions in order, e.g.
	// ["openai", "openai_responses"] or ["openai", "claude"].
	RequestConversionChain []relayconstant.RelayFormat
	// OpenAIResponsesToolContext carries per-request Responses tool proxy
	// metadata across multi-hop conversions such as Responses -> Chat -> Claude
	// and Claude -> Chat -> Responses.
	OpenAIResponsesToolContext *convert.OpenAIWireToolContext
	// 最终请求到上游的格式 TODO: 当前仅设置了Claude
	FinalRequestRelayFormat relayconstant.RelayFormat

	// UsageSource 由响应解析点显式写入的上游 usage 协议族（relayconstant.UsageSource），
	// 计费归一化据此选择语义规则；禁止用 FinalRequestRelayFormat 反推。
	UsageSource relayconstant.UsageSource
	// ServiceTierEffective 是响应侧实际生效层级；请求侧 service_tier 只用于
	// 上游请求，不覆盖该快照。
	ServiceTierEffective string
	// UsageGeminiMetadata 保留 Gemini usageMetadata 原始值：转换后的 shared.Usage
	// 会把 toolUsePromptTokenCount 并入 prompt 总量与模态明细，归一化
	// （billing.BuildBillingUsage）需要原始拆分才能满足"tool-use 只审计不进计价输入"。
	UsageGeminiMetadata *shared.GeminiUsageMetadata

	TokenCountMeta
	*ClaudeConvertInfo
	*GeminiConvertInfo
	*RerankerInfo
	*ResponsesUsageInfo
	*ChannelMeta
}

// SetEffectiveServiceTier records only the response-side tier. An omitted or
// empty field in a later event never overwrites a value already observed.
func (info *RelayInfo) SetEffectiveServiceTier(tier string) {
	if info != nil && tier != "" {
		info.ServiceTierEffective = tier
	}
}

func (info *RelayInfo) InitChannelMeta(c *gin.Context) {
	channelType := httpapi.GetContextKeyInt(c, common.ContextKeyChannelType)
	paramOverride := httpapi.GetContextKeyStringMap(c, common.ContextKeyChannelParamOverride)
	headerOverride := httpapi.GetContextKeyStringMap(c, common.ContextKeyChannelHeaderOverride)
	apiType, _ := common.ChannelType2APIType(channelType)
	channelMeta := &ChannelMeta{
		ChannelType:          channelType,
		ChannelId:            httpapi.GetContextKeyInt(c, common.ContextKeyChannelId),
		ChannelIsMultiKey:    httpapi.GetContextKeyBool(c, common.ContextKeyChannelIsMultiKey),
		ChannelMultiKeyIndex: httpapi.GetContextKeyInt(c, common.ContextKeyChannelMultiKeyIndex),
		ChannelBaseUrl:       httpapi.GetContextKeyString(c, common.ContextKeyChannelBaseUrl),
		ApiType:              apiType,
		ApiVersion:           c.GetString("api_version"),
		ApiKey:               httpapi.GetContextKeyString(c, common.ContextKeyChannelKey),
		Organization:         c.GetString("channel_organization"),
		ChannelCreateTime:    c.GetInt64("channel_create_time"),
		ParamOverride:        paramOverride,
		HeadersOverride:      headerOverride,
		UpstreamModelName:    httpapi.GetContextKeyString(c, common.ContextKeyOriginalModel),
		IsModelMapped:        false,
		SupportStreamOptions: false,
	}

	if channelType == channelconstant.ChannelTypeAzure {
		channelMeta.ApiVersion = GetAPIVersion(c)
	}
	if channelType == channelconstant.ChannelTypeVertexAi {
		channelMeta.ApiVersion = c.GetString("region")
	}

	channelSetting, ok := httpapi.GetContextKeyType[shared.ChannelSettings](c, common.ContextKeyChannelSetting)
	if ok {
		channelMeta.ChannelSetting = channelSetting
	}

	channelOtherSettings, ok := httpapi.GetContextKeyType[shared.ChannelOtherSettings](c, common.ContextKeyChannelOtherSetting)
	if ok {
		channelMeta.ChannelOtherSettings = channelOtherSettings
	}

	if streamSupportedChannels[channelMeta.ChannelType] {
		channelMeta.SupportStreamOptions = true
	}

	info.ChannelMeta = channelMeta
	if info.ResponseModelName == "" {
		info.ResponseModelName = info.OriginModelName
	}

	// reset some fields based on channel meta
	// 重置某些字段，例如模型名称等
	if info.Request != nil {
		info.Request.SetModelName(info.OriginModelName)
	}
}

func (info *RelayInfo) GetResponseModelName() string {
	if info == nil {
		return ""
	}
	if info.ResponseModelName != "" {
		return info.ResponseModelName
	}
	return info.OriginModelName
}

func (info *RelayInfo) ToString() string {
	if info == nil {
		return "RelayInfo<nil>"
	}

	// Basic info
	b := &strings.Builder{}
	fmt.Fprintf(b, "RelayInfo{ ")
	fmt.Fprintf(b, "RelayFormat: %s, ", info.RelayFormat)
	fmt.Fprintf(b, "RelayMode: %d, ", info.RelayMode)
	fmt.Fprintf(b, "IsStream: %t, ", info.IsStream)
	fmt.Fprintf(b, "RequestURLPath: %q, ", info.RequestURLPath)
	fmt.Fprintf(b, "OriginModelName: %q, ", info.OriginModelName)
	fmt.Fprintf(b, "EstimatePromptTokens: %d, ", info.estimatePromptTokens)
	fmt.Fprintf(b, "ShouldIncludeUsage: %t, ", info.ShouldIncludeUsage)
	fmt.Fprintf(b, "DisablePing: %t, ", info.DisablePing)
	fmt.Fprintf(b, "SendResponseCount: %d, ", info.SendResponseCount)
	fmt.Fprintf(b, "FinalPreConsumedQuota: %d, ", info.FinalPreConsumedQuota)

	// User & token info (mask secrets)
	fmt.Fprintf(b, "User{ Id: %d, Email: %q, Group: %q, UsingGroup: %q, Quota: %d }, ",
		info.UserId, common.MaskEmail(info.UserEmail), info.UserGroup, info.UsingGroup, info.UserQuota)
	fmt.Fprintf(b, "Token{ Id: %d, Unlimited: %t, Key: ***masked*** }, ", info.TokenId, info.TokenUnlimited)

	// Time info
	latencyMs := info.FirstResponseTime.Sub(info.StartTime).Milliseconds()
	fmt.Fprintf(b, "Timing{ Start: %s, FirstResponse: %s, LatencyMs: %d }, ",
		info.StartTime.Format(time.RFC3339Nano), info.FirstResponseTime.Format(time.RFC3339Nano), latencyMs)

	// Audio / realtime
	if info.InputAudioFormat != "" || info.OutputAudioFormat != "" || len(info.RealtimeTools) > 0 || info.AudioUsage {
		fmt.Fprintf(b, "Realtime{ AudioUsage: %t, InFmt: %q, OutFmt: %q, Tools: %d }, ",
			info.AudioUsage, info.InputAudioFormat, info.OutputAudioFormat, len(info.RealtimeTools))
	}

	// Reasoning
	if info.ReasoningEffort != "" {
		fmt.Fprintf(b, "ReasoningEffort: %q, ", info.ReasoningEffort)
	}

	// Price data (non-sensitive)
	if info.PriceData.UsePrice {
		fmt.Fprintf(b, "PriceData{ %s }, ", info.PriceData.ToSetting())
	}

	// Channel metadata (mask ApiKey)
	if info.ChannelMeta != nil {
		cm := info.ChannelMeta
		fmt.Fprintf(b, "ChannelMeta{ Type: %d, Id: %d, IsMultiKey: %t, MultiKeyIndex: %d, BaseURL: %q, ApiType: %d, ApiVersion: %q, Organization: %q, CreateTime: %d, UpstreamModelName: %q, IsModelMapped: %t, SupportStreamOptions: %t, ApiKey: ***masked*** }, ",
			cm.ChannelType, cm.ChannelId, cm.ChannelIsMultiKey, cm.ChannelMultiKeyIndex, cm.ChannelBaseUrl, cm.ApiType, cm.ApiVersion, cm.Organization, cm.ChannelCreateTime, cm.UpstreamModelName, cm.IsModelMapped, cm.SupportStreamOptions)
	}

	// Responses usage info (non-sensitive)
	if info.ResponsesUsageInfo != nil && len(info.ResponsesUsageInfo.BuiltInTools) > 0 {
		fmt.Fprintf(b, "ResponsesTools{ ")
		first := true
		for name, tool := range info.ResponsesUsageInfo.BuiltInTools {
			if !first {
				fmt.Fprintf(b, ", ")
			}
			first = false
			if tool != nil {
				fmt.Fprintf(b, "%s: calls=%d", name, tool.CallCount)
			} else {
				fmt.Fprintf(b, "%s: calls=0", name)
			}
		}
		fmt.Fprintf(b, " }, ")
	}

	fmt.Fprintf(b, "}")
	return b.String()
}

// 定义支持流式选项的通道类型
var streamSupportedChannels = map[int]bool{
	channelconstant.ChannelTypeOpenAI:      true,
	channelconstant.ChannelTypeAnthropic:   true,
	channelconstant.ChannelTypeAws:         true,
	channelconstant.ChannelTypeGemini:      true,
	channelconstant.ChannelTypeAzure:       true,
	channelconstant.ChannelTypeOllama:      true,
	channelconstant.ChannelTypeDeepSeek:    true,
	channelconstant.ChannelTypeZhipu_v4:    true,
	channelconstant.ChannelTypeByteDance:   true,
	channelconstant.ChannelTypeMoonshot:    true,
	channelconstant.ChannelTypeMiniMax:     true,
	channelconstant.ChannelTypeSiliconFlow: true,
}

func GenRelayInfoWs(c *gin.Context, ws *websocket.Conn) *RelayInfo {
	info := genBaseRelayInfo(c, nil)
	info.RelayFormat = relayconstant.RelayFormatOpenAIRealtime
	info.ClientWs = ws
	info.InputAudioFormat = "pcm16"
	info.OutputAudioFormat = "pcm16"
	info.IsFirstRequest = true
	return info
}

func GenRelayInfoClaude(c *gin.Context, request shared.Request) *RelayInfo {
	info := genBaseRelayInfo(c, request)
	info.RelayFormat = relayconstant.RelayFormatClaude
	info.ShouldIncludeUsage = false
	info.ClaudeConvertInfo = &ClaudeConvertInfo{
		LastMessagesType: LastMessageTypeNone,
	}
	info.IsClaudeBetaQuery = c.Query("beta") == "true" || isClaudeBetaForced(c)
	return info
}

func isClaudeBetaForced(c *gin.Context) bool {
	channelOtherSettings, ok := httpapi.GetContextKeyType[shared.ChannelOtherSettings](c, common.ContextKeyChannelOtherSetting)
	return ok && channelOtherSettings.ClaudeBetaQuery
}

func GenRelayInfoRerank(c *gin.Context, request *shared.RerankRequest) *RelayInfo {
	info := genBaseRelayInfo(c, request)
	info.RelayMode = relayconstant.RelayModeRerank
	info.RelayFormat = relayconstant.RelayFormatRerank
	info.RerankerInfo = &RerankerInfo{
		Documents:       request.Documents,
		ReturnDocuments: request.GetReturnDocuments(),
	}
	return info
}

func GenRelayInfoOpenAIAudio(c *gin.Context, request shared.Request) *RelayInfo {
	info := genBaseRelayInfo(c, request)
	info.RelayFormat = relayconstant.RelayFormatOpenAIAudio
	return info
}

func GenRelayInfoEmbedding(c *gin.Context, request shared.Request) *RelayInfo {
	info := genBaseRelayInfo(c, request)
	info.RelayFormat = relayconstant.RelayFormatEmbedding
	return info
}

func GenRelayInfoResponses(c *gin.Context, request *shared.OpenAIResponsesRequest) *RelayInfo {
	info := genBaseRelayInfo(c, request)
	info.RelayMode = relayconstant.RelayModeResponses
	info.RelayFormat = relayconstant.RelayFormatOpenAIResponses

	info.ResponsesUsageInfo = &ResponsesUsageInfo{
		BuiltInTools: make(map[string]*BuildInToolInfo),
	}
	if len(request.Tools) > 0 {
		for _, tool := range request.GetToolsMap() {
			toolType := common.Interface2String(tool["type"])
			info.ResponsesUsageInfo.BuiltInTools[toolType] = &BuildInToolInfo{
				ToolName:  toolType,
				CallCount: 0,
			}
			switch toolType {
			case shared.BuildInToolWebSearchPreview:
				searchContextSize := common.Interface2String(tool["search_context_size"])
				if searchContextSize == "" {
					searchContextSize = "medium"
				}
				info.ResponsesUsageInfo.BuiltInTools[toolType].SearchContextSize = searchContextSize
			}
		}
	}
	return info
}

func GenRelayInfoGemini(c *gin.Context, request shared.Request) *RelayInfo {
	info := genBaseRelayInfo(c, request)
	info.RelayFormat = relayconstant.RelayFormatGemini
	info.ShouldIncludeUsage = false

	return info
}

func GenRelayInfoImage(c *gin.Context, request shared.Request) *RelayInfo {
	info := genBaseRelayInfo(c, request)
	info.RelayFormat = relayconstant.RelayFormatOpenAIImage
	return info
}

func GenRelayInfoOpenAI(c *gin.Context, request shared.Request) *RelayInfo {
	info := genBaseRelayInfo(c, request)
	info.RelayFormat = relayconstant.RelayFormatOpenAI
	return info
}

func genBaseRelayInfo(c *gin.Context, request shared.Request) *RelayInfo {

	//channelType := httpapi.GetContextKeyInt(c, common.ContextKeyChannelType)
	//channelId := httpapi.GetContextKeyInt(c, common.ContextKeyChannelId)
	//paramOverride := httpapi.GetContextKeyStringMap(c, common.ContextKeyChannelParamOverride)

	tokenGroup := httpapi.GetContextKeyString(c, common.ContextKeyTokenGroup)
	// 当令牌分组为空时，表示使用用户分组
	if tokenGroup == "" {
		tokenGroup = httpapi.GetContextKeyString(c, common.ContextKeyUserGroup)
	}

	startTime := httpapi.GetContextKeyTime(c, common.ContextKeyRequestStartTime)
	if startTime.IsZero() {
		startTime = time.Now()
	}

	isStream := false

	if request != nil {
		isStream = request.IsStream(c)
	}

	// firstResponseTime = time.Now() - 1 second

	reqId := httpapi.GetContextKeyString(c, common.RequestIdKey)
	if reqId == "" {
		reqId = common.GetTimeString() + common.GetRandomString(8)
	}
	info := &RelayInfo{
		Request: request,

		RequestId:  reqId,
		UserId:     httpapi.GetContextKeyInt(c, common.ContextKeyUserId),
		UsingGroup: httpapi.GetContextKeyString(c, common.ContextKeyUsingGroup),
		UserGroup:  httpapi.GetContextKeyString(c, common.ContextKeyUserGroup),
		UserQuota:  httpapi.GetContextKeyInt(c, common.ContextKeyUserQuota),
		UserEmail:  httpapi.GetContextKeyString(c, common.ContextKeyUserEmail),

		OriginModelName:   httpapi.GetContextKeyString(c, common.ContextKeyOriginalModel),
		ResponseModelName: httpapi.GetContextKeyString(c, common.ContextKeyOriginalModel),

		TokenId:        httpapi.GetContextKeyInt(c, common.ContextKeyTokenId),
		TokenKey:       httpapi.GetContextKeyString(c, common.ContextKeyTokenKey),
		TokenQuota:     httpapi.GetContextKeyInt(c, common.ContextKeyTokenQuota),
		TokenQuotaType: httpapi.GetContextKeyInt(c, common.ContextKeyTokenQuotaType),
		TokenUnlimited: httpapi.GetContextKeyBool(c, common.ContextKeyTokenUnlimited),
		TokenGroup:     tokenGroup,

		isFirstResponse: true,
		RelayMode:       relayconstant.Path2RelayMode(c.Request.URL.Path),
		RequestURLPath:  c.Request.URL.String(),
		IsStream:        isStream,

		StartTime:         startTime,
		FirstResponseTime: startTime.Add(-time.Second),
		GeminiConvertInfo: &GeminiConvertInfo{
			ToolCallArguments: make(map[int]map[int]string),
			ToolCallNames:     make(map[int]map[int]string),
			ToolCallIDs:       make(map[int]map[int]string),
		},
		TokenCountMeta: TokenCountMeta{
			//promptTokens: httpapi.GetContextKeyInt(c, common.ContextKeyPromptTokens),
			estimatePromptTokens: httpapi.GetContextKeyInt(c, common.ContextKeyEstimatedTokens),
		},
	}

	if info.RelayMode == relayconstant.RelayModeUnknown {
		info.RelayMode = c.GetInt("relay_mode")
	}

	userSetting, ok := httpapi.GetContextKeyType[shared.UserSetting](c, common.ContextKeyUserSetting)
	if ok {
		info.UserSetting = userSetting
	}

	return info
}

func GenRelayInfo(c *gin.Context, relayFormat relayconstant.RelayFormat, request shared.Request, ws *websocket.Conn) (*RelayInfo, error) {
	var info *RelayInfo
	var err error
	switch relayFormat {
	case relayconstant.RelayFormatOpenAI:
		info = GenRelayInfoOpenAI(c, request)
	case relayconstant.RelayFormatOpenAIAudio:
		info = GenRelayInfoOpenAIAudio(c, request)
	case relayconstant.RelayFormatOpenAIImage:
		info = GenRelayInfoImage(c, request)
	case relayconstant.RelayFormatOpenAIRealtime:
		info = GenRelayInfoWs(c, ws)
	case relayconstant.RelayFormatClaude:
		info = GenRelayInfoClaude(c, request)
	case relayconstant.RelayFormatRerank:
		if request, ok := request.(*shared.RerankRequest); ok {
			info = GenRelayInfoRerank(c, request)
			break
		}
		err = errors.New("request is not a RerankRequest")
	case relayconstant.RelayFormatGemini:
		info = GenRelayInfoGemini(c, request)
	case relayconstant.RelayFormatEmbedding:
		info = GenRelayInfoEmbedding(c, request)
	case relayconstant.RelayFormatOpenAIResponses:
		if request, ok := request.(*shared.OpenAIResponsesRequest); ok {
			info = GenRelayInfoResponses(c, request)
			break
		}
		err = errors.New("request is not a OpenAIResponsesRequest")
	case relayconstant.RelayFormatOpenAIResponsesCompaction:
		if request, ok := request.(*shared.OpenAIResponsesCompactionRequest); ok {
			return GenRelayInfoResponsesCompaction(c, request), nil
		}
		return nil, errors.New("request is not a OpenAIResponsesCompactionRequest")
	default:
		err = errors.New("invalid relay format")
	}

	if err != nil {
		return nil, err
	}
	if info == nil {
		return nil, errors.New("failed to build relay info")
	}

	info.InitRequestConversionChain()
	return info, nil
}

func (info *RelayInfo) InitRequestConversionChain() {
	if info == nil {
		return
	}
	if len(info.RequestConversionChain) > 0 {
		return
	}
	if info.RelayFormat == "" {
		return
	}
	info.RequestConversionChain = []relayconstant.RelayFormat{info.RelayFormat}
}

func (info *RelayInfo) AppendRequestConversion(format relayconstant.RelayFormat) {
	if info == nil {
		return
	}
	if format == "" {
		return
	}
	if len(info.RequestConversionChain) == 0 {
		info.RequestConversionChain = []relayconstant.RelayFormat{format}
		return
	}
	last := info.RequestConversionChain[len(info.RequestConversionChain)-1]
	if last == format {
		return
	}
	info.RequestConversionChain = append(info.RequestConversionChain, format)
}

func GenRelayInfoResponsesCompaction(c *gin.Context, request *shared.OpenAIResponsesCompactionRequest) *RelayInfo {
	info := genBaseRelayInfo(c, request)
	if info.RelayMode == relayconstant.RelayModeUnknown {
		info.RelayMode = relayconstant.RelayModeResponsesCompact
	}
	info.RelayFormat = relayconstant.RelayFormatOpenAIResponsesCompaction
	return info
}

//func (info *RelayInfo) SetPromptTokens(promptTokens int) {
//	info.promptTokens = promptTokens
//}

func (info *RelayInfo) SetEstimatePromptTokens(promptTokens int) {
	info.estimatePromptTokens = promptTokens
}

func (info *RelayInfo) GetEstimatePromptTokens() int {
	return info.estimatePromptTokens
}

func (info *RelayInfo) SetFirstResponseTime() {
	if info.isFirstResponse {
		info.FirstResponseTime = time.Now()
		info.isFirstResponse = false
	}
}

func (info *RelayInfo) HasSendResponse() bool {
	return info.FirstResponseTime.After(info.StartTime)
}

// RemoveDisabledFields 从请求 JSON 数据中移除渠道设置中禁用的字段
// service_tier: 服务层级字段，可能导致额外计费（OpenAI、Claude、Responses API 支持）
// store: 数据存储授权字段，涉及用户隐私（仅 OpenAI、Responses API 支持，默认允许透传，禁用后可能导致 Codex 无法使用）
// safety_identifier: 安全标识符，用于向 OpenAI 报告违规用户（仅 OpenAI 支持，涉及用户隐私）
func RemoveDisabledFields(jsonData []byte, channelOtherSettings shared.ChannelOtherSettings) ([]byte, error) {
	var data map[string]interface{}
	if err := jsonx.Unmarshal(jsonData, &data); err != nil {
		common.SysError("RemoveDisabledFields Unmarshal error :" + err.Error())
		return jsonData, nil
	}

	// 默认移除 service_tier，除非明确允许（避免额外计费风险）
	if !channelOtherSettings.AllowServiceTier {
		delete(data, "service_tier")
	}

	// 默认允许 store 透传，除非明确禁用（禁用可能影响 Codex 使用）
	if channelOtherSettings.DisableStore {
		delete(data, "store")
	}

	// 默认移除 safety_identifier，除非明确允许（保护用户隐私，避免向 OpenAI 报告用户信息）
	if !channelOtherSettings.AllowSafetyIdentifier {
		delete(data, "safety_identifier")
	}

	jsonDataAfter, err := jsonx.Marshal(data)
	if err != nil {
		common.SysError("RemoveDisabledFields Marshal error :" + err.Error())
		return jsonData, nil
	}
	return jsonDataAfter, nil
}

// RemoveClaudeDisabledFields applies the common field passthrough policy plus
// Claude-specific controls for cache_control and speed.
func RemoveClaudeDisabledFields(jsonData []byte, channelOtherSettings shared.ChannelOtherSettings) ([]byte, error) {
	jsonData, err := RemoveDisabledFields(jsonData, channelOtherSettings)
	if err != nil {
		return jsonData, err
	}

	if channelOtherSettings.AllowCacheControl && channelOtherSettings.AllowSpeed {
		return jsonData, nil
	}

	var data any
	if err := jsonx.Unmarshal(jsonData, &data); err != nil {
		common.SysError("RemoveClaudeDisabledFields Unmarshal error :" + err.Error())
		return jsonData, nil
	}

	if root, ok := data.(map[string]interface{}); ok && !channelOtherSettings.AllowSpeed {
		delete(root, "speed")
	}

	if !channelOtherSettings.AllowCacheControl {
		removeJSONFieldRecursive(data, "cache_control")
	}

	jsonDataAfter, err := jsonx.Marshal(data)
	if err != nil {
		common.SysError("RemoveClaudeDisabledFields Marshal error :" + err.Error())
		return jsonData, nil
	}
	return jsonDataAfter, nil
}

func removeJSONFieldRecursive(value any, fieldName string) {
	switch typed := value.(type) {
	case map[string]interface{}:
		delete(typed, fieldName)
		for _, child := range typed {
			removeJSONFieldRecursive(child, fieldName)
		}
	case []interface{}:
		for _, child := range typed {
			removeJSONFieldRecursive(child, fieldName)
		}
	}
}

// RemoveGeminiDisabledFields removes disabled fields from Gemini request JSON data
// Currently supports removing functionResponse.id field which Vertex AI does not support
func RemoveGeminiDisabledFields(jsonData []byte) ([]byte, error) {
	if !model.GetGeminiSettings().RemoveFunctionResponseIdEnabled {
		return jsonData, nil
	}

	var data map[string]interface{}
	if err := jsonx.Unmarshal(jsonData, &data); err != nil {
		common.SysError("RemoveGeminiDisabledFields Unmarshal error: " + err.Error())
		return jsonData, nil
	}

	// Process contents array
	// Handle both camelCase (functionResponse) and snake_case (function_response)
	if contents, ok := data["contents"].([]interface{}); ok {
		for _, content := range contents {
			if contentMap, ok := content.(map[string]interface{}); ok {
				if parts, ok := contentMap["parts"].([]interface{}); ok {
					for _, part := range parts {
						if partMap, ok := part.(map[string]interface{}); ok {
							// Check functionResponse (camelCase)
							if funcResp, ok := partMap["functionResponse"].(map[string]interface{}); ok {
								delete(funcResp, "id")
							}
							// Check function_response (snake_case)
							if funcResp, ok := partMap["function_response"].(map[string]interface{}); ok {
								delete(funcResp, "id")
							}
						}
					}
				}
			}
		}
	}

	jsonDataAfter, err := jsonx.Marshal(data)
	if err != nil {
		common.SysError("RemoveGeminiDisabledFields Marshal error: " + err.Error())
		return jsonData, nil
	}
	return jsonDataAfter, nil
}
