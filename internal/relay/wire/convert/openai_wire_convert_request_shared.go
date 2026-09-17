package convert

const (
	openAIResponsesInputTypeText   = "input_text"
	openAIResponsesOutputTypeText  = "output_text"
	openAIResponsesInputTypeImage  = "input_image"
	openAIResponsesInputTypeFile   = "input_file"
	openAIResponsesSummaryTextType = "summary_text"
)

const (
	openAIResponsesInputItemTypeMessage            = "message"
	openAIResponsesInputItemTypeReasoning          = "reasoning"
	openAIResponsesInputItemTypeFunctionCall       = "function_call"
	openAIResponsesInputItemTypeFunctionCallOutput = "function_call_output"
	openAIResponsesInputItemTypeCustomToolCall     = "custom_tool_call"
	openAIResponsesInputItemTypeCustomToolOutput   = "custom_tool_call_output"
	openAIResponsesInputItemTypeToolSearchCall     = "tool_search_call"
	openAIResponsesInputItemTypeToolSearchOutput   = "tool_search_output"
	openAIResponsesInputItemTypeComputerCall       = "computer_call"
	openAIResponsesInputItemTypeComputerCallOutput = "computer_call_output"
	openAIResponsesInputItemTypeLocalShellCall     = "local_shell_call"

	openAIResponsesInputItemTypeLocalShellCallOutput = "local_shell_call_output"
	openAIResponsesInputItemTypeWebSearchCall        = "web_search_call"
	openAIResponsesInputItemTypeFileSearchCall       = "file_search_call"
	openAIResponsesInputItemTypeImageGenerationCall  = "image_generation_call"
)

const (
	openAIResponsesToolTypeFunction   = "function"
	openAIResponsesToolTypeCustom     = "custom"
	openAIResponsesToolTypeNamespace  = "namespace"
	openAIResponsesToolTypeToolSearch = "tool_search"

	// Responses 服务端内置工具。这些工具由 OpenAI 在 Responses 端点执行，
	// Chat Completions 上游无法识别，因此在 Responses → Chat 请求转换时丢弃。
	// Chat → Responses 方向无需处理：Chat 客户端无法声明这些内置工具。
	openAIResponsesToolTypeWebSearch        = "web_search"
	openAIResponsesToolTypeWebSearchPreview = "web_search_preview" // 早期预览版别名
	openAIResponsesToolTypeImageGeneration  = "image_generation"

	openAIResponsesToolSearchChatName = "tool_search"
	openAIResponsesCustomInputField   = "input"
)
