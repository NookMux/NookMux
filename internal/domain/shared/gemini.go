package shared

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/NookMux/NookMux/internal/infra/log"

	"github.com/NookMux/NookMux/pkg/jsonx"

	"github.com/gin-gonic/gin"
)

type GeminiChatRequest struct {
	Requests           []GeminiChatRequest        `json:"requests,omitempty"` // For batch requests
	Contents           []GeminiChatContent        `json:"contents"`
	SafetySettings     []GeminiChatSafetySettings `json:"safetySettings,omitempty"`
	GenerationConfig   GeminiChatGenerationConfig `json:"generationConfig,omitempty"`
	Tools              json.RawMessage            `json:"tools,omitempty"`
	ToolConfig         *ToolConfig                `json:"toolConfig,omitempty"`
	SystemInstructions *GeminiChatContent         `json:"systemInstruction,omitempty"`
	CachedContent      string                     `json:"cachedContent,omitempty"`
}

// UnmarshalJSON allows GeminiChatRequest to accept both snake_case and camelCase fields.
func (r *GeminiChatRequest) UnmarshalJSON(data []byte) error {
	type Alias GeminiChatRequest
	var aux struct {
		Alias
		SystemInstructionSnake *GeminiChatContent `json:"system_instruction,omitempty"`
	}

	if err := jsonx.Unmarshal(data, &aux); err != nil {
		return err
	}

	*r = GeminiChatRequest(aux.Alias)

	if aux.SystemInstructionSnake != nil {
		r.SystemInstructions = aux.SystemInstructionSnake
	}

	return nil
}

type ToolConfig struct {
	FunctionCallingConfig *FunctionCallingConfig `json:"functionCallingConfig,omitempty"`
	RetrievalConfig       *RetrievalConfig       `json:"retrievalConfig,omitempty"`
}

type FunctionCallingConfig struct {
	Mode                 FunctionCallingConfigMode `json:"mode,omitempty"`
	AllowedFunctionNames []string                  `json:"allowedFunctionNames,omitempty"`
}
type FunctionCallingConfigMode string

type RetrievalConfig struct {
	LatLng       *LatLng `json:"latLng,omitempty"`
	LanguageCode string  `json:"languageCode,omitempty"`
}

type LatLng struct {
	Latitude  *float64 `json:"latitude,omitempty"`
	Longitude *float64 `json:"longitude,omitempty"`
}

// createGeminiFileSource 根据数据内容创建正确类型的 FileSource
func createGeminiFileSource(data string, mimeType string) *FileSource {
	if strings.HasPrefix(data, "http://") || strings.HasPrefix(data, "https://") {
		return NewURLFileSource(data)
	}
	return NewBase64FileSource(data, mimeType)
}

func (r *GeminiChatRequest) GetTokenCountMeta() *TokenCountMeta {
	var files []*FileMeta = make([]*FileMeta, 0)

	var maxTokens int

	if r.GenerationConfig.MaxOutputTokens > 0 {
		maxTokens = int(r.GenerationConfig.MaxOutputTokens)
	}

	var inputTexts []string
	for _, content := range r.Contents {
		for _, part := range content.Parts {
			if part.Text != "" {
				inputTexts = append(inputTexts, part.Text)
			}
			if part.InlineData != nil && part.InlineData.Data != "" {
				mimeType := part.InlineData.MimeType
				source := createGeminiFileSource(part.InlineData.Data, mimeType)
				var fileType FileType
				if strings.HasPrefix(mimeType, "image/") {
					fileType = FileTypeImage
				} else if strings.HasPrefix(mimeType, "audio/") {
					fileType = FileTypeAudio
				} else if strings.HasPrefix(mimeType, "video/") {
					fileType = FileTypeVideo
				} else {
					fileType = FileTypeFile
				}
				files = append(files, &FileMeta{
					FileType: fileType,
					Source:   source,
					MimeType: mimeType,
				})
			}
		}
	}

	inputText := strings.Join(inputTexts, "\n")
	return &TokenCountMeta{
		CombineText: inputText,
		Files:       files,
		MaxTokens:   maxTokens,
	}
}

func (r *GeminiChatRequest) IsStream(c *gin.Context) bool {
	return c.Query("alt") == "sse"
}

func (r *GeminiChatRequest) SetModelName(modelName string) {
	// GeminiChatRequest does not have a model field, so this method does nothing.
}

func (r *GeminiChatRequest) GetTools() []GeminiChatTool {
	var tools []GeminiChatTool
	if strings.HasPrefix(string(r.Tools), "[") {
		// is array
		if err := jsonx.Unmarshal(r.Tools, &tools); err != nil {
			log.LogError(context.Background(), "error_unmarshalling_tools: "+err.Error())
			return nil
		}
	} else if strings.HasPrefix(string(r.Tools), "{") {
		// is object
		singleTool := GeminiChatTool{}
		if err := jsonx.Unmarshal(r.Tools, &singleTool); err != nil {
			log.LogError(context.Background(), "error_unmarshalling_single_tool: "+err.Error())
			return nil
		}
		tools = []GeminiChatTool{singleTool}
	}
	return tools
}

func (r *GeminiChatRequest) SetTools(tools []GeminiChatTool) {
	if len(tools) == 0 {
		r.Tools = json.RawMessage("[]")
		return
	}

	// Marshal the tools to JSON
	data, err := jsonx.Marshal(tools)
	if err != nil {
		log.LogError(context.Background(), "error_marshalling_tools: "+err.Error())
		return
	}
	r.Tools = data
}

type GeminiThinkingConfig struct {
	IncludeThoughts bool `json:"includeThoughts,omitempty"`
	ThinkingBudget  *int `json:"thinkingBudget,omitempty"`
	// TODO Conflict with thinkingbudget.
	ThinkingLevel string `json:"thinkingLevel,omitempty"`
}

// UnmarshalJSON allows GeminiThinkingConfig to accept both snake_case and camelCase fields.
func (c *GeminiThinkingConfig) UnmarshalJSON(data []byte) error {
	type Alias GeminiThinkingConfig
	var aux struct {
		Alias
		IncludeThoughtsSnake *bool  `json:"include_thoughts,omitempty"`
		ThinkingBudgetSnake  *int   `json:"thinking_budget,omitempty"`
		ThinkingLevelSnake   string `json:"thinking_level,omitempty"`
	}

	if err := jsonx.Unmarshal(data, &aux); err != nil {
		return err
	}

	*c = GeminiThinkingConfig(aux.Alias)

	if aux.IncludeThoughtsSnake != nil {
		c.IncludeThoughts = *aux.IncludeThoughtsSnake
	}

	if aux.ThinkingBudgetSnake != nil {
		c.ThinkingBudget = aux.ThinkingBudgetSnake
	}

	if aux.ThinkingLevelSnake != "" {
		c.ThinkingLevel = aux.ThinkingLevelSnake
	}

	return nil
}

func (c *GeminiThinkingConfig) SetThinkingBudget(budget int) {
	c.ThinkingBudget = &budget
}

type GeminiInlineData struct {
	MimeType string `json:"mimeType"`
	Data     string `json:"data"`
}

// UnmarshalJSON custom unmarshaler for GeminiInlineData to support snake_case and camelCase for MimeType
func (g *GeminiInlineData) UnmarshalJSON(data []byte) error {
	type Alias GeminiInlineData // Use type alias to avoid recursion
	var aux struct {
		Alias
		MimeTypeSnake string `json:"mime_type"`
	}

	if err := jsonx.Unmarshal(data, &aux); err != nil {
		return err
	}

	*g = GeminiInlineData(aux.Alias) // Copy other fields if any in future

	// Prioritize snake_case if present
	if aux.MimeTypeSnake != "" {
		g.MimeType = aux.MimeTypeSnake
	} else if aux.MimeType != "" { // Fallback to camelCase from Alias
		g.MimeType = aux.MimeType
	}
	// g.Data would be populated by aux.Alias.Data
	return nil
}

type FunctionCall struct {
	FunctionName string `json:"name"`
	Arguments    any    `json:"args"`
}

type GeminiFunctionResponse struct {
	Name         string                 `json:"name"`
	Response     map[string]interface{} `json:"response"`
	WillContinue json.RawMessage        `json:"willContinue,omitempty"`
	Scheduling   json.RawMessage        `json:"scheduling,omitempty"`
	Parts        json.RawMessage        `json:"parts,omitempty"`
	ID           json.RawMessage        `json:"id,omitempty"`
}

// GetID returns Gemini's optional function response id as a plain string.
func (r *GeminiFunctionResponse) GetID() string {
	if r == nil || len(r.ID) == 0 {
		return ""
	}

	var id string
	if err := jsonx.Unmarshal(r.ID, &id); err == nil {
		return id
	}
	return strings.Trim(string(r.ID), `"`)
}

// SetID stores a function response id using Gemini's raw JSON field shape.
func (r *GeminiFunctionResponse) SetID(id string) {
	if r == nil {
		return
	}
	if strings.TrimSpace(id) == "" {
		r.ID = nil
		return
	}

	payload, err := jsonx.Marshal(id)
	if err != nil {
		return
	}
	r.ID = payload
}

type GeminiPartExecutableCode struct {
	Language string `json:"language,omitempty"`
	Code     string `json:"code,omitempty"`
}

type GeminiPartCodeExecutionResult struct {
	Outcome string `json:"outcome,omitempty"`
	Output  string `json:"output,omitempty"`
}

type GeminiFileData struct {
	MimeType string `json:"mimeType,omitempty"`
	FileUri  string `json:"fileUri,omitempty"`
}

type GeminiPart struct {
	Text             string                  `json:"text,omitempty"`
	Thought          bool                    `json:"thought,omitempty"`
	InlineData       *GeminiInlineData       `json:"inlineData,omitempty"`
	FunctionCall     *FunctionCall           `json:"functionCall,omitempty"`
	ThoughtSignature json.RawMessage         `json:"thoughtSignature,omitempty"`
	FunctionResponse *GeminiFunctionResponse `json:"functionResponse,omitempty"`
	// Optional. Media resolution for the input media.
	MediaResolution     json.RawMessage                `json:"mediaResolution,omitempty"`
	VideoMetadata       json.RawMessage                `json:"videoMetadata,omitempty"`
	FileData            *GeminiFileData                `json:"fileData,omitempty"`
	ExecutableCode      *GeminiPartExecutableCode      `json:"executableCode,omitempty"`
	CodeExecutionResult *GeminiPartCodeExecutionResult `json:"codeExecutionResult,omitempty"`
}

// GetThoughtSignature returns Gemini's optional thought signature as a plain string.
func (p GeminiPart) GetThoughtSignature() string {
	if len(p.ThoughtSignature) == 0 {
		return ""
	}

	var signature string
	if err := jsonx.Unmarshal(p.ThoughtSignature, &signature); err == nil {
		return signature
	}
	return strings.Trim(string(p.ThoughtSignature), `"`)
}

// SetThoughtSignature stores a thought signature using Gemini's raw JSON field shape.
func (p *GeminiPart) SetThoughtSignature(signature string) {
	if p == nil {
		return
	}
	if strings.TrimSpace(signature) == "" {
		p.ThoughtSignature = nil
		return
	}

	payload, err := jsonx.Marshal(signature)
	if err != nil {
		return
	}
	p.ThoughtSignature = payload
}

// UnmarshalJSON custom unmarshaler for GeminiPart to support snake_case and camelCase for InlineData
func (p *GeminiPart) UnmarshalJSON(data []byte) error {
	// Alias to avoid recursion during unmarshalling
	type Alias GeminiPart
	var aux struct {
		Alias
		InlineDataSnake *GeminiInlineData `json:"inline_data,omitempty"` // snake_case variant
	}

	if err := jsonx.Unmarshal(data, &aux); err != nil {
		return err
	}

	// Assign fields from alias
	*p = GeminiPart(aux.Alias)

	// Prioritize snake_case for InlineData if present
	if aux.InlineDataSnake != nil {
		p.InlineData = aux.InlineDataSnake
	} else if aux.InlineData != nil { // Fallback to camelCase from Alias
		p.InlineData = aux.InlineData
	}
	// Other fields like Text, FunctionCall etc. are already populated via aux.Alias

	return nil
}

type GeminiChatContent struct {
	Role  string       `json:"role,omitempty"`
	Parts []GeminiPart `json:"parts"`
}

type GeminiChatSafetySettings struct {
	Category  string `json:"category"`
	Threshold string `json:"threshold"`
}

type GeminiChatTool struct {
	GoogleSearch          any `json:"googleSearch,omitempty"`
	GoogleSearchRetrieval any `json:"googleSearchRetrieval,omitempty"`
	CodeExecution         any `json:"codeExecution,omitempty"`
	FunctionDeclarations  any `json:"functionDeclarations,omitempty"`
	URLContext            any `json:"urlContext,omitempty"`
}

type GeminiChatGenerationConfig struct {
	Temperature        *float64              `json:"temperature,omitempty"`
	TopP               float64               `json:"topP,omitempty"`
	TopK               float64               `json:"topK,omitempty"`
	MaxOutputTokens    uint                  `json:"maxOutputTokens,omitempty"`
	CandidateCount     int                   `json:"candidateCount,omitempty"`
	StopSequences      []string              `json:"stopSequences,omitempty"`
	ResponseMimeType   string                `json:"responseMimeType,omitempty"`
	ResponseSchema     any                   `json:"responseSchema,omitempty"`
	ResponseJsonSchema json.RawMessage       `json:"responseJsonSchema,omitempty"`
	PresencePenalty    *float32              `json:"presencePenalty,omitempty"`
	FrequencyPenalty   *float32              `json:"frequencyPenalty,omitempty"`
	ResponseLogprobs   bool                  `json:"responseLogprobs,omitempty"`
	Logprobs           *int32                `json:"logprobs,omitempty"`
	MediaResolution    MediaResolution       `json:"mediaResolution,omitempty"`
	Seed               int64                 `json:"seed,omitempty"`
	ResponseModalities []string              `json:"responseModalities,omitempty"`
	ThinkingConfig     *GeminiThinkingConfig `json:"thinkingConfig,omitempty"`
	SpeechConfig       json.RawMessage       `json:"speechConfig,omitempty"` // RawMessage to allow flexible speech config
	ImageConfig        json.RawMessage       `json:"imageConfig,omitempty"`  // RawMessage to allow flexible image config
}

// UnmarshalJSON allows GeminiChatGenerationConfig to accept both snake_case and camelCase fields.
func (c *GeminiChatGenerationConfig) UnmarshalJSON(data []byte) error {
	type Alias GeminiChatGenerationConfig
	var aux struct {
		Alias
		TopPSnake               float64               `json:"top_p,omitempty"`
		TopKSnake               float64               `json:"top_k,omitempty"`
		MaxOutputTokensSnake    uint                  `json:"max_output_tokens,omitempty"`
		CandidateCountSnake     int                   `json:"candidate_count,omitempty"`
		StopSequencesSnake      []string              `json:"stop_sequences,omitempty"`
		ResponseMimeTypeSnake   string                `json:"response_mime_type,omitempty"`
		ResponseSchemaSnake     any                   `json:"response_schema,omitempty"`
		ResponseJsonSchemaSnake json.RawMessage       `json:"response_json_schema,omitempty"`
		PresencePenaltySnake    *float32              `json:"presence_penalty,omitempty"`
		FrequencyPenaltySnake   *float32              `json:"frequency_penalty,omitempty"`
		ResponseLogprobsSnake   bool                  `json:"response_logprobs,omitempty"`
		MediaResolutionSnake    MediaResolution       `json:"media_resolution,omitempty"`
		ResponseModalitiesSnake []string              `json:"response_modalities,omitempty"`
		ThinkingConfigSnake     *GeminiThinkingConfig `json:"thinking_config,omitempty"`
		SpeechConfigSnake       json.RawMessage       `json:"speech_config,omitempty"`
		ImageConfigSnake        json.RawMessage       `json:"image_config,omitempty"`
	}

	if err := jsonx.Unmarshal(data, &aux); err != nil {
		return err
	}

	*c = GeminiChatGenerationConfig(aux.Alias)

	// Prioritize snake_case if present
	if aux.TopPSnake != 0 {
		c.TopP = aux.TopPSnake
	}
	if aux.TopKSnake != 0 {
		c.TopK = aux.TopKSnake
	}
	if aux.MaxOutputTokensSnake != 0 {
		c.MaxOutputTokens = aux.MaxOutputTokensSnake
	}
	if aux.CandidateCountSnake != 0 {
		c.CandidateCount = aux.CandidateCountSnake
	}
	if len(aux.StopSequencesSnake) > 0 {
		c.StopSequences = aux.StopSequencesSnake
	}
	if aux.ResponseMimeTypeSnake != "" {
		c.ResponseMimeType = aux.ResponseMimeTypeSnake
	}
	if aux.ResponseSchemaSnake != nil {
		c.ResponseSchema = aux.ResponseSchemaSnake
	}
	if len(aux.ResponseJsonSchemaSnake) > 0 {
		c.ResponseJsonSchema = aux.ResponseJsonSchemaSnake
	}
	if aux.PresencePenaltySnake != nil {
		c.PresencePenalty = aux.PresencePenaltySnake
	}
	if aux.FrequencyPenaltySnake != nil {
		c.FrequencyPenalty = aux.FrequencyPenaltySnake
	}
	if aux.ResponseLogprobsSnake {
		c.ResponseLogprobs = aux.ResponseLogprobsSnake
	}
	if aux.MediaResolutionSnake != "" {
		c.MediaResolution = aux.MediaResolutionSnake
	}
	if len(aux.ResponseModalitiesSnake) > 0 {
		c.ResponseModalities = aux.ResponseModalitiesSnake
	}
	if aux.ThinkingConfigSnake != nil {
		c.ThinkingConfig = aux.ThinkingConfigSnake
	}
	if len(aux.SpeechConfigSnake) > 0 {
		c.SpeechConfig = aux.SpeechConfigSnake
	}
	if len(aux.ImageConfigSnake) > 0 {
		c.ImageConfig = aux.ImageConfigSnake
	}

	return nil
}

type MediaResolution string

type GeminiChatCandidate struct {
	Content       GeminiChatContent        `json:"content"`
	FinishReason  *string                  `json:"finishReason"`
	Index         int64                    `json:"index"`
	SafetyRatings []GeminiChatSafetyRating `json:"safetyRatings"`
}

// UnmarshalJSON allows GeminiChatCandidate to accept both snake_case and camelCase fields.
func (c *GeminiChatCandidate) UnmarshalJSON(data []byte) error {
	type Alias GeminiChatCandidate
	var aux struct {
		Alias
		FinishReasonSnake  *string                  `json:"finish_reason,omitempty"`
		SafetyRatingsSnake []GeminiChatSafetyRating `json:"safety_ratings,omitempty"`
	}

	if err := jsonx.Unmarshal(data, &aux); err != nil {
		return err
	}

	*c = GeminiChatCandidate(aux.Alias)
	if aux.FinishReasonSnake != nil {
		c.FinishReason = aux.FinishReasonSnake
	}
	if len(aux.SafetyRatingsSnake) > 0 {
		c.SafetyRatings = aux.SafetyRatingsSnake
	}
	return nil
}

type GeminiChatSafetyRating struct {
	Category    string `json:"category"`
	Probability string `json:"probability"`
}

type GeminiChatPromptFeedback struct {
	SafetyRatings []GeminiChatSafetyRating `json:"safetyRatings"`
	BlockReason   *string                  `json:"blockReason,omitempty"`
}

// UnmarshalJSON allows GeminiChatPromptFeedback to accept both snake_case and camelCase fields.
func (f *GeminiChatPromptFeedback) UnmarshalJSON(data []byte) error {
	type Alias GeminiChatPromptFeedback
	var aux struct {
		Alias
		SafetyRatingsSnake []GeminiChatSafetyRating `json:"safety_ratings,omitempty"`
		BlockReasonSnake   *string                  `json:"block_reason,omitempty"`
	}

	if err := jsonx.Unmarshal(data, &aux); err != nil {
		return err
	}

	*f = GeminiChatPromptFeedback(aux.Alias)
	if len(aux.SafetyRatingsSnake) > 0 {
		f.SafetyRatings = aux.SafetyRatingsSnake
	}
	if aux.BlockReasonSnake != nil {
		f.BlockReason = aux.BlockReasonSnake
	}
	return nil
}

type GeminiChatResponse struct {
	Candidates     []GeminiChatCandidate     `json:"candidates"`
	PromptFeedback *GeminiChatPromptFeedback `json:"promptFeedback,omitempty"`
	UsageMetadata  GeminiUsageMetadata       `json:"usageMetadata"`
	// Error 保留上游在 HTTP 200 中携带的错误载荷（Gemini 及部分中间网关会把
	// 429/5xx 转成 200 + {"error":{...}} 下发），用于 handler 识别真实上游错误。
	Error *GeminiErrorResponse `json:"error,omitempty"`
}

// GeminiErrorResponse 是 Gemini API 的标准错误载荷。
type GeminiErrorResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Status  string `json:"status"`
}

// UnmarshalJSON allows GeminiChatResponse to accept both snake_case and camelCase fields.
func (r *GeminiChatResponse) UnmarshalJSON(data []byte) error {
	type Alias GeminiChatResponse
	var aux struct {
		Alias
		PromptFeedbackSnake *GeminiChatPromptFeedback `json:"prompt_feedback,omitempty"`
		UsageMetadataSnake  *GeminiUsageMetadata      `json:"usage_metadata,omitempty"`
	}

	if err := jsonx.Unmarshal(data, &aux); err != nil {
		return err
	}

	*r = GeminiChatResponse(aux.Alias)
	if aux.PromptFeedbackSnake != nil {
		r.PromptFeedback = aux.PromptFeedbackSnake
	}
	if aux.UsageMetadataSnake != nil {
		r.UsageMetadata = *aux.UsageMetadataSnake
	}
	return nil
}

type GeminiUsageMetadata struct {
	PromptTokenCount           int                         `json:"promptTokenCount"`
	ToolUsePromptTokenCount    int                         `json:"toolUsePromptTokenCount"`
	CandidatesTokenCount       int                         `json:"candidatesTokenCount"`
	TotalTokenCount            int                         `json:"totalTokenCount"`
	ThoughtsTokenCount         int                         `json:"thoughtsTokenCount"`
	CachedContentTokenCount    int                         `json:"cachedContentTokenCount"`
	PromptTokensDetails        []GeminiPromptTokensDetails `json:"promptTokensDetails"`
	ToolUsePromptTokensDetails []GeminiPromptTokensDetails `json:"toolUsePromptTokensDetails"`
	CandidatesTokensDetails    []GeminiPromptTokensDetails `json:"candidatesTokensDetails"`
}

// UnmarshalJSON allows GeminiUsageMetadata to accept both snake_case and camelCase fields.
func (m *GeminiUsageMetadata) UnmarshalJSON(data []byte) error {
	type Alias GeminiUsageMetadata
	var aux struct {
		Alias
		PromptTokenCountSnake           int                         `json:"prompt_token_count,omitempty"`
		ToolUsePromptTokenCountSnake    int                         `json:"tool_use_prompt_token_count,omitempty"`
		CandidatesTokenCountSnake       int                         `json:"candidates_token_count,omitempty"`
		TotalTokenCountSnake            int                         `json:"total_token_count,omitempty"`
		ThoughtsTokenCountSnake         int                         `json:"thoughts_token_count,omitempty"`
		CachedContentTokenCountSnake    int                         `json:"cached_content_token_count,omitempty"`
		PromptTokensDetailsSnake        []GeminiPromptTokensDetails `json:"prompt_tokens_details,omitempty"`
		ToolUsePromptTokensDetailsSnake []GeminiPromptTokensDetails `json:"tool_use_prompt_tokens_details,omitempty"`
		CandidatesTokensDetailsSnake    []GeminiPromptTokensDetails `json:"candidates_tokens_details,omitempty"`
	}

	if err := jsonx.Unmarshal(data, &aux); err != nil {
		return err
	}

	*m = GeminiUsageMetadata(aux.Alias)
	if aux.PromptTokenCountSnake != 0 {
		m.PromptTokenCount = aux.PromptTokenCountSnake
	}
	if aux.ToolUsePromptTokenCountSnake != 0 {
		m.ToolUsePromptTokenCount = aux.ToolUsePromptTokenCountSnake
	}
	if aux.CandidatesTokenCountSnake != 0 {
		m.CandidatesTokenCount = aux.CandidatesTokenCountSnake
	}
	if aux.TotalTokenCountSnake != 0 {
		m.TotalTokenCount = aux.TotalTokenCountSnake
	}
	if aux.ThoughtsTokenCountSnake != 0 {
		m.ThoughtsTokenCount = aux.ThoughtsTokenCountSnake
	}
	if aux.CachedContentTokenCountSnake != 0 {
		m.CachedContentTokenCount = aux.CachedContentTokenCountSnake
	}
	if len(aux.PromptTokensDetailsSnake) > 0 {
		m.PromptTokensDetails = aux.PromptTokensDetailsSnake
	}
	if len(aux.ToolUsePromptTokensDetailsSnake) > 0 {
		m.ToolUsePromptTokensDetails = aux.ToolUsePromptTokensDetailsSnake
	}
	if len(aux.CandidatesTokensDetailsSnake) > 0 {
		m.CandidatesTokensDetails = aux.CandidatesTokensDetailsSnake
	}
	return nil
}

type GeminiPromptTokensDetails struct {
	Modality   string `json:"modality"`
	TokenCount int    `json:"tokenCount"`
}

// UnmarshalJSON allows GeminiPromptTokensDetails to accept both snake_case and camelCase fields.
func (d *GeminiPromptTokensDetails) UnmarshalJSON(data []byte) error {
	type Alias GeminiPromptTokensDetails
	var aux struct {
		Alias
		TokenCountSnake int `json:"token_count,omitempty"`
	}

	if err := jsonx.Unmarshal(data, &aux); err != nil {
		return err
	}

	*d = GeminiPromptTokensDetails(aux.Alias)
	if aux.TokenCountSnake != 0 {
		d.TokenCount = aux.TokenCountSnake
	}
	return nil
}

// Imagen related structs
type GeminiImageRequest struct {
	Instances  []GeminiImageInstance `json:"instances"`
	Parameters GeminiImageParameters `json:"parameters"`
}

type GeminiImageInstance struct {
	Prompt string `json:"prompt"`
}

type GeminiImageParameters struct {
	SampleCount      int    `json:"sampleCount,omitempty"`
	AspectRatio      string `json:"aspectRatio,omitempty"`
	PersonGeneration string `json:"personGeneration,omitempty"`
	ImageSize        string `json:"imageSize,omitempty"`
}

type GeminiImageResponse struct {
	Predictions []GeminiImagePrediction `json:"predictions"`
}

type GeminiImagePrediction struct {
	MimeType           string `json:"mimeType"`
	BytesBase64Encoded string `json:"bytesBase64Encoded"`
	RaiFilteredReason  string `json:"raiFilteredReason,omitempty"`
	SafetyAttributes   any    `json:"safetyAttributes,omitempty"`
}

// Embedding related structs
type GeminiEmbeddingRequest struct {
	Model                string            `json:"model,omitempty"`
	Content              GeminiChatContent `json:"content"`
	TaskType             string            `json:"taskType,omitempty"`
	Title                string            `json:"title,omitempty"`
	OutputDimensionality int               `json:"outputDimensionality,omitempty"`
}

func (r *GeminiEmbeddingRequest) IsStream(c *gin.Context) bool {
	// Gemini embedding requests are not streamed
	return false
}

func (r *GeminiEmbeddingRequest) GetTokenCountMeta() *TokenCountMeta {
	var inputTexts []string
	for _, part := range r.Content.Parts {
		if part.Text != "" {
			inputTexts = append(inputTexts, part.Text)
		}
	}
	inputText := strings.Join(inputTexts, "\n")
	return &TokenCountMeta{
		CombineText: inputText,
	}
}

func (r *GeminiEmbeddingRequest) SetModelName(modelName string) {
	if modelName != "" {
		r.Model = modelName
	}
}

type GeminiBatchEmbeddingRequest struct {
	Requests []*GeminiEmbeddingRequest `json:"requests"`
}

func (r *GeminiBatchEmbeddingRequest) IsStream(c *gin.Context) bool {
	// Gemini batch embedding requests are not streamed
	return false
}

func (r *GeminiBatchEmbeddingRequest) GetTokenCountMeta() *TokenCountMeta {
	var inputTexts []string
	for _, request := range r.Requests {
		meta := request.GetTokenCountMeta()
		if meta != nil && meta.CombineText != "" {
			inputTexts = append(inputTexts, meta.CombineText)
		}
	}
	inputText := strings.Join(inputTexts, "\n")
	return &TokenCountMeta{
		CombineText: inputText,
	}
}

func (r *GeminiBatchEmbeddingRequest) SetModelName(modelName string) {
	if modelName != "" {
		for _, req := range r.Requests {
			req.SetModelName(modelName)
		}
	}
}

type GeminiEmbeddingResponse struct {
	Embedding ContentEmbedding `json:"embedding"`
}

type GeminiBatchEmbeddingResponse struct {
	Embeddings []*ContentEmbedding `json:"embeddings"`
}

type ContentEmbedding struct {
	Values []float64 `json:"values"`
}
