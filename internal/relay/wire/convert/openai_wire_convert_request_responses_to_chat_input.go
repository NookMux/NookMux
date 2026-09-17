package convert

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/NookMux/NookMux/internal/domain/shared"
	"github.com/NookMux/NookMux/pkg/jsonx"
)

type responsesInputTypeProbe struct {
	Type string `json:"type"`
}

type responsesFunctionCallInput struct {
	Type      string          `json:"type"`
	CallID    string          `json:"call_id"`
	Name      string          `json:"name"`
	Namespace string          `json:"namespace,omitempty"`
	Arguments json.RawMessage `json:"arguments"`
	Input     string          `json:"input"`
}

type responsesFunctionCallOutputInput struct {
	Type    string          `json:"type"`
	CallID  string          `json:"call_id"`
	Output  json.RawMessage `json:"output"`
	IsError *bool           `json:"tool_call_is_error,omitempty"`
}

type responsesReasoningInput struct {
	Type    string                        `json:"type"`
	Summary []shared.ResponsesContentPart `json:"summary,omitempty"`
}

type responsesUnsupportedCallOutputInput struct {
	Type   string          `json:"type"`
	CallID string          `json:"call_id"`
	Output json.RawMessage `json:"output"`
}

func buildChatMessagesFromResponsesInput(raw json.RawMessage) ([]shared.Message, error) {
	if len(raw) == 0 {
		return nil, fmt.Errorf("input is required")
	}

	switch jsonx.GetJsonType(raw) {
	case "string":
		var s string
		if err := jsonx.Unmarshal(raw, &s); err != nil {
			return nil, fmt.Errorf("unmarshal input string failed: %w", err)
		}
		return []shared.Message{{Role: "user", Content: s}}, nil
	case "array":
		return buildChatMessagesFromResponsesInputArray(raw)
	default:
		return nil, fmt.Errorf("unsupported input type: %s", jsonx.GetJsonType(raw))
	}
}

func buildChatMessagesFromResponsesInputArray(raw json.RawMessage) ([]shared.Message, error) {
	var items []json.RawMessage
	if err := jsonx.Unmarshal(raw, &items); err != nil {
		return nil, fmt.Errorf("unmarshal input array failed: %w", err)
	}
	if len(items) == 0 {
		return nil, fmt.Errorf("input items are empty")
	}

	out := make([]shared.Message, 0, len(items))
	pendingReasoning := ""
	pendingToolCalls := make([]shared.ToolCallResponse, 0)
	flushToolCalls := func() error {
		if len(pendingToolCalls) == 0 {
			return nil
		}
		if len(out) > 0 && strings.EqualFold(out[len(out)-1].Role, "assistant") {
			if err := appendToolCallsToChatAssistantMessage(&out[len(out)-1], pendingReasoning, pendingToolCalls); err != nil {
				return err
			}
			pendingReasoning = ""
			pendingToolCalls = pendingToolCalls[:0]
			return nil
		}
		msg, err := buildChatAssistantMessage("", pendingReasoning, pendingToolCalls)
		if err != nil {
			return err
		}
		out = append(out, msg)
		pendingReasoning = ""
		pendingToolCalls = pendingToolCalls[:0]
		return nil
	}
	for i, itemRaw := range items {
		itemType, err := probeResponsesInputItemType(itemRaw)
		if err != nil {
			return nil, fmt.Errorf("input[%d]: %w", i, err)
		}
		if itemType == "" {
			itemType = openAIResponsesInputItemTypeMessage
		}
		if isUnsupportedResponsesExecutedCallItem(itemType) {
			continue
		}
		if itemType == openAIResponsesInputItemTypeReasoning {
			reasoning, err := extractResponsesReasoningSummary(itemRaw)
			if err != nil {
				return nil, fmt.Errorf("input[%d]: %w", i, err)
			}
			if strings.TrimSpace(reasoning) != "" {
				pendingReasoning = appendReasoningSummary(pendingReasoning, reasoning)
			}
			continue
		}
		if itemType == openAIResponsesInputItemTypeFunctionCall ||
			itemType == openAIResponsesInputItemTypeCustomToolCall ||
			itemType == openAIResponsesInputItemTypeToolSearchCall {
			call, err := buildChatToolCallFromResponsesFunctionCall(itemRaw)
			if err != nil {
				return nil, fmt.Errorf("input[%d]: %w", i, err)
			}
			pendingToolCalls = append(pendingToolCalls, call)
			continue
		}
		if err := flushToolCalls(); err != nil {
			return nil, fmt.Errorf("input[%d]: %w", i, err)
		}

		msgs, err := buildChatMessagesFromResponsesInputItemByType(itemType, itemRaw)
		if err != nil {
			return nil, fmt.Errorf("input[%d]: %w", i, err)
		}
		if strings.TrimSpace(pendingReasoning) != "" {
			attachReasoningToMessages(&msgs, pendingReasoning)
			pendingReasoning = ""
		}
		out = append(out, msgs...)
	}
	if err := flushToolCalls(); err != nil {
		return nil, err
	}
	if strings.TrimSpace(pendingReasoning) != "" {
		msg := shared.Message{Role: "assistant"}
		msg.SetReasoningContent(pendingReasoning)
		out = append(out, msg)
	}
	return out, nil
}

func buildChatMessagesFromResponsesInputItemByType(itemType string, raw json.RawMessage) ([]shared.Message, error) {
	switch itemType {
	case openAIResponsesInputItemTypeMessage:
		return buildChatMessagesFromResponsesMessageItem(raw)
	case openAIResponsesInputItemTypeReasoning:
		reasoning, err := extractResponsesReasoningSummary(raw)
		if err != nil {
			return nil, err
		}
		if strings.TrimSpace(reasoning) == "" {
			return nil, nil
		}
		msg := shared.Message{Role: "assistant"}
		msg.SetReasoningContent(reasoning)
		return []shared.Message{msg}, nil
	case openAIResponsesInputItemTypeFunctionCall, openAIResponsesInputItemTypeCustomToolCall, openAIResponsesInputItemTypeToolSearchCall:
		msg, err := buildChatToolCallMessageFromResponsesFunctionCall(raw)
		if err != nil {
			return nil, err
		}
		return []shared.Message{msg}, nil
	case openAIResponsesInputItemTypeFunctionCallOutput, openAIResponsesInputItemTypeCustomToolOutput, openAIResponsesInputItemTypeToolSearchOutput:
		msg, err := buildChatToolOutputMessageFromResponsesFunctionCallOutput(raw)
		if err != nil {
			return nil, err
		}
		return []shared.Message{msg}, nil
	case openAIResponsesInputItemTypeComputerCallOutput, openAIResponsesInputItemTypeLocalShellCallOutput:
		msg, err := buildChatMessageFromResponsesUnsupportedCallOutput(raw)
		if err != nil {
			return nil, err
		}
		if msg == nil {
			return nil, nil
		}
		return []shared.Message{*msg}, nil
	default:
		return nil, fmt.Errorf("unsupported input item type: %q", itemType)
	}
}

func isUnsupportedResponsesExecutedCallItem(itemType string) bool {
	switch itemType {
	case openAIResponsesInputItemTypeComputerCall,
		openAIResponsesInputItemTypeLocalShellCall,
		openAIResponsesInputItemTypeWebSearchCall,
		openAIResponsesInputItemTypeFileSearchCall,
		openAIResponsesInputItemTypeImageGenerationCall:
		return true
	default:
		return false
	}
}

func buildChatMessageFromResponsesUnsupportedCallOutput(raw json.RawMessage) (*shared.Message, error) {
	var item responsesUnsupportedCallOutputInput
	if err := jsonx.Unmarshal(raw, &item); err != nil {
		return nil, fmt.Errorf("unmarshal %s item failed: %w", item.Type, err)
	}
	output, ok := responsesUnsupportedCallOutputString(item.Output)
	if !ok {
		return nil, nil
	}

	prefix := strings.TrimSpace(item.Type)
	if callID := strings.TrimSpace(item.CallID); callID != "" {
		prefix += " " + callID
	}
	return &shared.Message{
		Role:    "user",
		Content: prefix + " output:\n" + output,
	}, nil
}

func responsesUnsupportedCallOutputString(raw json.RawMessage) (string, bool) {
	if len(raw) == 0 || jsonx.GetJsonType(raw) == "null" {
		return "", false
	}
	if jsonx.GetJsonType(raw) == "string" {
		var output string
		if err := jsonx.Unmarshal(raw, &output); err != nil {
			return "", false
		}
		return output, strings.TrimSpace(output) != ""
	}

	var output any
	if err := jsonx.Unmarshal(raw, &output); err != nil {
		return "", false
	}
	if isImageOnlyUnsupportedCallOutput(output) {
		return "", false
	}
	encoded, err := jsonx.Marshal(output)
	if err != nil {
		return "", false
	}
	normalized := strings.TrimSpace(string(encoded))
	return normalized, normalized != "" && normalized != "null"
}

func isImageOnlyUnsupportedCallOutput(output any) bool {
	part, ok := output.(map[string]any)
	if !ok {
		return false
	}
	partType, _ := part["type"].(string)
	if strings.EqualFold(strings.TrimSpace(partType), openAIResponsesInputTypeImage) {
		return true
	}
	_, hasImageURL := part["image_url"]
	return hasImageURL
}

func probeResponsesInputItemType(raw json.RawMessage) (string, error) {
	var probe responsesInputTypeProbe
	if err := jsonx.Unmarshal(raw, &probe); err != nil {
		return "", fmt.Errorf("unmarshal input item type failed: %w", err)
	}
	return strings.TrimSpace(probe.Type), nil
}

func buildChatMessagesFromResponsesMessageItem(raw json.RawMessage) ([]shared.Message, error) {
	var item shared.Input
	if err := jsonx.Unmarshal(raw, &item); err != nil {
		return nil, fmt.Errorf("unmarshal message item failed: %w", err)
	}

	role := strings.TrimSpace(item.Role)
	switch role {
	case "":
		role = "user"
	case "developer":
		role = "system"
	}

	msg, err := buildChatMessageFromResponsesMessageContent(role, item.Content)
	if err != nil {
		return nil, err
	}
	return []shared.Message{msg}, nil
}

func buildChatMessageFromResponsesMessageContent(role string, raw json.RawMessage) (shared.Message, error) {
	switch jsonx.GetJsonType(raw) {
	case "string":
		var s string
		if err := jsonx.Unmarshal(raw, &s); err != nil {
			return shared.Message{}, fmt.Errorf("unmarshal content string failed: %w", err)
		}
		return shared.Message{Role: role, Content: s}, nil
	case "array":
		var parts []map[string]any
		if err := jsonx.Unmarshal(raw, &parts); err != nil {
			return shared.Message{}, fmt.Errorf("unmarshal content parts failed: %w", err)
		}
		media, err := convertResponsesContentPartsToChat(parts)
		if err != nil {
			return shared.Message{}, err
		}
		if text, ok := collapseChatMediaToString(media); ok {
			return shared.Message{Role: role, Content: text}, nil
		}
		return shared.Message{Role: role, Content: media}, nil
	default:
		return shared.Message{}, fmt.Errorf("unsupported content type: %s", jsonx.GetJsonType(raw))
	}
}

func buildChatToolCallMessageFromResponsesFunctionCall(raw json.RawMessage) (shared.Message, error) {
	call, err := buildChatToolCallFromResponsesFunctionCall(raw)
	if err != nil {
		return shared.Message{}, err
	}

	rawToolCalls, err := jsonx.Marshal([]shared.ToolCallResponse{call})
	if err != nil {
		return shared.Message{}, fmt.Errorf("marshal tool_calls failed: %w", err)
	}

	return shared.Message{
		Role:      "assistant",
		Content:   nil,
		ToolCalls: rawToolCalls,
	}, nil
}

func buildChatToolCallFromResponsesFunctionCall(raw json.RawMessage) (shared.ToolCallResponse, error) {
	var item responsesFunctionCallInput
	if err := jsonx.Unmarshal(raw, &item); err != nil {
		return shared.ToolCallResponse{}, fmt.Errorf("unmarshal function_call item failed: %w", err)
	}
	callID := strings.TrimSpace(item.CallID)
	if callID == "" {
		return shared.ToolCallResponse{}, fmt.Errorf("function_call.call_id is required")
	}
	name := strings.TrimSpace(item.Name)
	itemType := strings.TrimSpace(item.Type)
	if itemType == openAIResponsesInputItemTypeToolSearchCall {
		name = openAIResponsesToolSearchChatName
	}
	if name == "" {
		return shared.ToolCallResponse{}, fmt.Errorf("%s.name is required", item.Type)
	}
	if itemType == openAIResponsesInputItemTypeCustomToolCall {
		if strings.TrimSpace(item.Namespace) != "" {
			name = flattenOpenAIResponsesNamespaceToolName(item.Namespace, name)
		}
		arguments, err := BuildChatArgumentsForResponsesCustomToolInput(item.Input)
		if err != nil {
			return shared.ToolCallResponse{}, fmt.Errorf("marshal custom tool call arguments failed: %w", err)
		}
		return shared.ToolCallResponse{
			ID:   callID,
			Type: "function",
			Function: shared.FunctionResponse{
				Name:      name,
				Arguments: arguments,
			},
		}, nil
	}
	arguments, err := responsesArgumentsToChatString(item.Arguments)
	if err != nil {
		return shared.ToolCallResponse{}, err
	}
	if strings.TrimSpace(item.Namespace) != "" {
		name = flattenOpenAIResponsesNamespaceToolName(item.Namespace, name)
	}

	return shared.ToolCallResponse{
		ID:   callID,
		Type: "function",
		Function: shared.FunctionResponse{
			Name:      name,
			Arguments: arguments,
		},
	}, nil
}

func buildChatToolOutputMessageFromResponsesFunctionCallOutput(raw json.RawMessage) (shared.Message, error) {
	var item responsesFunctionCallOutputInput
	if err := jsonx.Unmarshal(raw, &item); err != nil {
		return shared.Message{}, fmt.Errorf("unmarshal function_call_output item failed: %w", err)
	}
	callID := strings.TrimSpace(item.CallID)
	if callID == "" {
		return shared.Message{}, fmt.Errorf("function_call_output.call_id is required")
	}
	var output any
	var err error
	if strings.TrimSpace(item.Type) == openAIResponsesInputItemTypeToolSearchOutput {
		output = string(raw)
	} else {
		output, err = responsesFunctionCallOutputToChatContent(item.Output)
		if err != nil {
			return shared.Message{}, err
		}
	}

	return shared.Message{
		Role:            "tool",
		Content:         output,
		ToolCallId:      callID,
		ToolCallIsError: item.IsError,
	}, nil
}

func appendToolCallsToChatAssistantMessage(msg *shared.Message, reasoning string, toolCalls []shared.ToolCallResponse) error {
	if msg == nil || len(toolCalls) == 0 {
		return nil
	}
	if strings.TrimSpace(reasoning) != "" {
		msg.SetReasoningContent(appendReasoningSummary(msg.GetReasoningContent(), reasoning))
	}
	existing := make([]shared.ToolCallResponse, 0)
	if len(msg.ToolCalls) > 0 && jsonx.GetJsonType(msg.ToolCalls) != "null" {
		if err := jsonx.Unmarshal(msg.ToolCalls, &existing); err != nil {
			return fmt.Errorf("unmarshal existing tool_calls failed: %w", err)
		}
	}
	existing = append(existing, toolCalls...)
	raw, err := jsonx.Marshal(existing)
	if err != nil {
		return fmt.Errorf("marshal tool_calls failed: %w", err)
	}
	msg.ToolCalls = raw
	return nil
}

func responsesFunctionCallOutputToChatContent(raw json.RawMessage) (any, error) {
	if len(raw) == 0 || jsonx.GetJsonType(raw) == "null" {
		return "", nil
	}
	if jsonx.GetJsonType(raw) == "string" {
		var output string
		if err := jsonx.Unmarshal(raw, &output); err != nil {
			return nil, fmt.Errorf("unmarshal function_call_output.output failed: %w", err)
		}
		return output, nil
	}
	if jsonx.GetJsonType(raw) != "array" {
		return nil, fmt.Errorf("function_call_output.output must be a string or content array, got %s", jsonx.GetJsonType(raw))
	}

	var parts []map[string]any
	if err := jsonx.Unmarshal(raw, &parts); err != nil {
		return nil, fmt.Errorf("unmarshal function_call_output.output failed: %w", err)
	}
	media, err := convertResponsesContentPartsToChat(parts)
	if err != nil {
		return nil, err
	}
	text, ok := collapseChatMediaToString(media)
	if !ok {
		return nil, fmt.Errorf("function_call_output.output only supports text parts for chat.completions conversion")
	}
	return text, nil
}

func responsesArgumentsToChatString(raw json.RawMessage) (string, error) {
	if len(raw) == 0 || jsonx.GetJsonType(raw) == "null" {
		return "{}", nil
	}
	if jsonx.GetJsonType(raw) == "string" {
		var arguments string
		if err := jsonx.Unmarshal(raw, &arguments); err != nil {
			return "", fmt.Errorf("unmarshal function_call.arguments failed: %w", err)
		}
		return arguments, nil
	}
	var arguments any
	if err := jsonx.Unmarshal(raw, &arguments); err != nil {
		return "", fmt.Errorf("unmarshal function_call.arguments failed: %w", err)
	}
	encoded, err := jsonx.Marshal(arguments)
	if err != nil {
		return "", fmt.Errorf("marshal function_call.arguments failed: %w", err)
	}
	return string(encoded), nil
}

func convertResponsesContentPartsToChat(parts []map[string]any) ([]shared.MediaContent, error) {
	out := make([]shared.MediaContent, 0, len(parts))
	for _, part := range parts {
		typ, _ := part["type"].(string)
		typ = strings.TrimSpace(typ)
		switch typ {
		case openAIResponsesInputTypeText, openAIResponsesOutputTypeText:
			text, _ := part["text"].(string)
			out = append(out, shared.MediaContent{Type: shared.ContentTypeText, Text: text})
		case openAIResponsesInputTypeImage:
			image := extractResponsesImageURL(part["image_url"])
			if image == nil || strings.TrimSpace(image.Url) == "" {
				return nil, fmt.Errorf("invalid input_image content")
			}
			out = append(out, shared.MediaContent{Type: shared.ContentTypeImageURL, ImageUrl: image})
		case openAIResponsesInputTypeFile:
			file := extractResponsesFile(part)
			if file == nil {
				return nil, fmt.Errorf("invalid input_file content")
			}
			out = append(out, shared.MediaContent{Type: shared.ContentTypeFile, File: file})
		default:
			return nil, fmt.Errorf("unsupported responses content type: %q", typ)
		}
	}
	return out, nil
}

func extractResponsesImageURL(v any) *shared.MessageImageUrl {
	switch val := v.(type) {
	case string:
		return &shared.MessageImageUrl{Url: val, Detail: "high"}
	case map[string]any:
		u, _ := val["url"].(string)
		detail, _ := val["detail"].(string)
		if strings.TrimSpace(detail) == "" {
			detail = "high"
		}
		return &shared.MessageImageUrl{Url: u, Detail: detail}
	default:
		return nil
	}
}

// extractResponsesReasoningSummary flattens a Responses reasoning item into the
// assistant reasoning_content field used by ChatCompletions-compatible clients.
func appendReasoningSummary(existing string, next string) string {
	existing = strings.TrimSpace(existing)
	next = strings.TrimSpace(next)
	if existing == "" {
		return next
	}
	if next == "" {
		return existing
	}
	return existing + "\n" + next
}

func extractResponsesReasoningSummary(raw json.RawMessage) (string, error) {
	var item responsesReasoningInput
	if err := jsonx.Unmarshal(raw, &item); err != nil {
		return "", fmt.Errorf("unmarshal reasoning item failed: %w", err)
	}
	parts := make([]string, 0, len(item.Summary))
	for _, summary := range item.Summary {
		if strings.TrimSpace(summary.Text) != "" {
			parts = append(parts, summary.Text)
		}
	}
	return strings.Join(parts, "\n"), nil
}

func attachReasoningToMessages(msgs *[]shared.Message, reasoning string) {
	if msgs == nil || len(*msgs) == 0 {
		return
	}
	for i := range *msgs {
		if strings.EqualFold((*msgs)[i].Role, "assistant") {
			(*msgs)[i].SetReasoningContent(reasoning)
			return
		}
	}
	*msgs = append([]shared.Message{{Role: "assistant"}}, (*msgs)...)
	msg := &(*msgs)[0]
	msg.SetReasoningContent(reasoning)
}

func extractResponsesFile(part map[string]any) *shared.MessageFile {
	if part == nil {
		return nil
	}
	file := &shared.MessageFile{}
	if fileID, _ := part["file_id"].(string); strings.TrimSpace(fileID) != "" {
		file.FileId = fileID
		return file
	}
	if fileData, _ := part["file_data"].(string); strings.TrimSpace(fileData) != "" {
		file.FileData = fileData
		if fileName, _ := part["filename"].(string); strings.TrimSpace(fileName) != "" {
			file.FileName = fileName
		}
		return file
	}
	if fileURL, _ := part["file_url"].(string); strings.TrimSpace(fileURL) != "" {
		file.FileData = fileURL
		if fileName, _ := part["filename"].(string); strings.TrimSpace(fileName) != "" {
			file.FileName = fileName
		}
		return file
	}
	return nil
}

func collapseChatMediaToString(media []shared.MediaContent) (string, bool) {
	if len(media) == 0 {
		return "", true
	}
	var builder strings.Builder
	for _, part := range media {
		if part.Type != shared.ContentTypeText {
			return "", false
		}
		builder.WriteString(part.Text)
	}
	return builder.String(), true
}
