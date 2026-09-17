package constant

import channelconstant "github.com/NookMux/NookMux/internal/domain/channel/constant"

type RelayFormat string

const (
	RelayFormatOpenAI                    RelayFormat = "openai"
	RelayFormatClaude                    RelayFormat = "claude"
	RelayFormatGemini                    RelayFormat = "gemini"
	RelayFormatOpenAIResponses           RelayFormat = "openai_responses"
	RelayFormatOpenAIResponsesCompaction RelayFormat = "openai_responses_compaction"
	RelayFormatOpenAIAudio               RelayFormat = "openai_audio"
	RelayFormatOpenAIImage               RelayFormat = "openai_image"
	RelayFormatOpenAIRealtime            RelayFormat = "openai_realtime"
	RelayFormatRerank                    RelayFormat = "rerank"
	RelayFormatEmbedding                 RelayFormat = "embedding"
)

// RelayFormatToPreferredAPIType returns the preferred API type for a given relay format.
// Returns -1 if the format has no specific API type preference.
func RelayFormatToPreferredAPIType(format RelayFormat) int {
	switch format {
	case RelayFormatOpenAI, RelayFormatOpenAIResponses,
		RelayFormatOpenAIResponsesCompaction,
		RelayFormatOpenAIAudio, RelayFormatOpenAIImage,
		RelayFormatOpenAIRealtime, RelayFormatRerank,
		RelayFormatEmbedding:
		return channelconstant.APITypeOpenAI
	case RelayFormatClaude:
		return channelconstant.APITypeAnthropic
	case RelayFormatGemini:
		return channelconstant.APITypeGemini
	default:
		return -1
	}
}
