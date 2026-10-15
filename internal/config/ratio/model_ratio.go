package ratio

import (
	"strings"

	"github.com/NookMux/NookMux/internal/common"
	"github.com/NookMux/NookMux/internal/domain/shared"
	"github.com/NookMux/NookMux/pkg/jsonx"
)

// from songquanpeng/one-api
const (
	USD2RMB = 7.3 // 暂定 1 USD = 7.3 RMB
	USD     = 500 // $0.002 = 1 -> $1 = 500
	RMB     = USD / USD2RMB
)

// modelRatio
// https://platform.openai.com/docs/models/model-endpoint-compatibility
// https://cloud.baidu.com/doc/WENXINWORKSHOP/s/Blfmc9dlf
// https://openai.com/pricing
// TODO: when a new api is enabled, check the pricing here
// 1 === $0.002 / 1K tokens
// 1 === ￥0.014 / 1k tokens

var defaultModelRatio = map[string]float64{
	"360GPT_S2_V9":                              0.8572,
	"360gpt-pro":                                0.8572,
	"360gpt-turbo":                              0.0858,
	"360gpt-turbo-responsibility-8k":            0.8572,
	"360gpt2-pro":                               0.8572,
	"BLOOMZ-7B":                                 0.004,
	"ERNIE-3.5-4K-0205":                         0.012,
	"ERNIE-3.5-8K":                              0.012,
	"ERNIE-3.5-8K-0205":                         0.024,
	"ERNIE-3.5-8K-1222":                         0.012,
	"ERNIE-4.0-8K":                              0.12,
	"ERNIE-Bot-8K":                              0.024,
	"ERNIE-Lite-8K-0308":                        0.003,
	"ERNIE-Lite-8K-0922":                        0.008,
	"ERNIE-Speed-128K":                          0.004,
	"ERNIE-Speed-8K":                            0.004,
	"ERNIE-Tiny-8K":                             0.001,
	"Embedding-V1":                              0.002,
	"MiniMax-M2":                                0.1275,
	"MiniMax-M2.1":                              0.15,
	"MiniMax-M2.5":                              0.075,
	"MiniMax-M2.7":                              0.125,
	"NousResearch/Hermes-4-405B-FP8":            0.8,
	"ada":                                       10,
	"babbage":                                   10,
	"babbage-002":                               0.2,
	"bge-large-en":                              0.002,
	"bge-large-zh":                              0.002,
	"bytedance/ui-tars-1.5-7b":                  0.05,
	"chatglm_lite":                              0.1429,
	"chatglm_pro":                               0.7143,
	"chatglm_std":                               0.3572,
	"chatglm_turbo":                             0.3572,
	"claude-3-7-sonnet-20250219-thinking":       1.5,
	"claude-3-haiku":                            0.125,
	"claude-3-haiku-20240307":                   0.125,
	"claude-fable-5":                            5,
	"claude-haiku-4.5":                          0.5,
	"claude-opus-4":                             7.5,
	"claude-opus-4-20250514":                    7.5,
	"claude-opus-4-6-high":                      2.5,
	"claude-opus-4-6-low":                       2.5,
	"claude-opus-4-6-max":                       2.5,
	"claude-opus-4-6-medium":                    2.5,
	"claude-opus-4.1":                           7.5,
	"claude-opus-4.5":                           2.5,
	"claude-opus-4.6":                           2.5,
	"claude-opus-4.7":                           2.5,
	"claude-opus-4.7-fast":                      15,
	"claude-opus-4.8":                           2.5,
	"claude-opus-4.8-fast":                      5,
	"claude-sonnet-4":                           1.5,
	"claude-sonnet-4-20250514":                  1.5,
	"claude-sonnet-4.5":                         1.5,
	"claude-sonnet-4.6":                         1.5,
	"claude-sonnet-5":                           1,
	"code-davinci-edit-001":                     10,
	"curie":                                     10,
	"davinci":                                   10,
	"davinci-002":                               1,
	"deepseek-ai/DeepSeek-R1":                   0.8,
	"deepseek-ai/DeepSeek-R1-0528":              0.8,
	"deepseek-ai/DeepSeek-V3-0324":              0.8,
	"deepseek-ai/DeepSeek-V3.1":                 0.8,
	"deepseek-chat":                             0.1001,
	"deepseek-chat-v3-0324":                     0.135,
	"deepseek-chat-v3.1":                        0.125,
	"deepseek-coder":                            0.27,
	"deepseek-r1":                               0.35,
	"deepseek-r1-0528":                          0.25,
	"deepseek-r1-distill-llama-70b":             0.4,
	"deepseek-v3.1-terminus":                    0.135,
	"deepseek-v3.2":                             0.1345,
	"deepseek-v3.2-exp":                         0.135,
	"deepseek-v4-flash":                         0.07,
	"deepseek-v4-pro":                           0.2175,
	"embedding-bert-512-v1":                     0.0715,
	"embedding_s1_v1":                           0.0715,
	"gemini-2.5-flash":                          0.15,
	"gemini-2.5-flash-image":                    0.15,
	"gemini-2.5-flash-lite":                     0.05,
	"gemini-2.5-flash-lite-preview-06-17":       0.05,
	"gemini-2.5-flash-lite-preview-thinking-*":  0.05,
	"gemini-2.5-flash-preview-04-17":            0.075,
	"gemini-2.5-flash-preview-04-17-nothinking": 0.075,
	"gemini-2.5-flash-preview-04-17-thinking":   0.075,
	"gemini-2.5-flash-preview-05-20":            0.075,
	"gemini-2.5-flash-preview-05-20-nothinking": 0.075,
	"gemini-2.5-flash-preview-05-20-thinking":   0.075,
	"gemini-2.5-flash-thinking-*":               0.075,
	"gemini-2.5-pro":                            0.625,
	"gemini-2.5-pro-preview":                    0.625,
	"gemini-2.5-pro-preview-05-06":              0.625,
	"gemini-2.5-pro-thinking-*":                 0.625,
	"gemini-3-flash-preview":                    0.25,
	"gemini-3-pro-image":                        1,
	"gemini-3-pro-image-preview":                1,
	"gemini-3.1-flash-image":                    0.25,
	"gemini-3.1-flash-image-preview":            0.25,
	"gemini-3.1-flash-lite":                     0.125,
	"gemini-3.1-flash-lite-image":               0.125,
	"gemini-3.1-flash-lite-preview":             0.125,
	"gemini-3.1-pro-preview":                    1,
	"gemini-3.1-pro-preview-customtools":        1,
	"gemini-3.5-flash":                          0.75,
	"gemini-3.5-flash-lite":                     0.15,
	"gemini-3.6-flash":                          0.375,
	"gemini-3.8-flash":                          0.375,
	"gemini-4-argon":                            1,
	"gemini-embedding-001":                      0.075,
	"gemini-embedding-2":                        0.1,
	"gemini-nano-banana-2.1":                    0.75,
	"gemini-robotics-er-1.5-preview":            0.15,
	"gemma-2-27b-it":                            0.325,
	"gemma-3-12b-it":                            0.025,
	"gemma-3-27b-it":                            0.05,
	"gemma-3-4b-it":                             0.025,
	"gemma-3n-e4b-it":                           0.03,
	"gemma-4-26b-a4b-it":                        0.05,
	"gemma-4-31b-it":                            0.11,
	"glm-4.5":                                   0.3,
	"glm-4.5-air":                               0.1,
	"glm-4.5v":                                  0.3,
	"glm-4.6":                                   0.3,
	"glm-4.6v":                                  0.15,
	"glm-4.7":                                   0.3,
	"glm-4.7-flash":                             0.03025,
	"glm-5":                                     0.475,
	"glm-5-turbo":                               0.6,
	"glm-5.1":                                   0.7,
	"glm-5.2":                                   0.7,
	"glm-5v-turbo":                              0.6,
	"gpt-3.5-turbo":                             0.25,
	"gpt-3.5-turbo-0125":                        0.25,
	"gpt-3.5-turbo-0613":                        0.5,
	"gpt-3.5-turbo-1106":                        0.5,
	"gpt-3.5-turbo-16k":                         1.5,
	"gpt-3.5-turbo-16k-0613":                    1.5,
	"gpt-3.5-turbo-instruct":                    0.75,
	"gpt-4":                                     15,
	"gpt-4-0125-preview":                        5,
	"gpt-4-0613":                                15,
	"gpt-4-1106-preview":                        5,
	"gpt-4-1106-vision-preview":                 5,
	"gpt-4-32k":                                 30,
	"gpt-4-32k-0613":                            30,
	"gpt-4-all":                                 15,
	"gpt-4-gizmo-*":                             15,
	"gpt-4-turbo":                               5,
	"gpt-4-turbo-2024-04-09":                    5,
	"gpt-4-turbo-preview":                       5,
	"gpt-4-vision-preview":                      5,
	"gpt-4.1":                                   1,
	"gpt-4.1-2025-04-14":                        1,
	"gpt-4.1-mini":                              0.2,
	"gpt-4.1-mini-2025-04-14":                   0.2,
	"gpt-4.1-nano":                              0.05,
	"gpt-4.1-nano-2025-04-14":                   0.05,
	"gpt-4.5-preview":                           37.5,
	"gpt-4.5-preview-2025-02-27":                37.5,
	"gpt-4o":                                    1.25,
	"gpt-4o-2024-05-13":                         2.5,
	"gpt-4o-2024-08-06":                         1.25,
	"gpt-4o-2024-11-20":                         1.25,
	"gpt-4o-all":                                15,
	"gpt-4o-audio-preview":                      1.25,
	"gpt-4o-audio-preview-2024-10-01":           1.25,
	"gpt-4o-gizmo-*":                            2.5,
	"gpt-4o-mini":                               0.075,
	"gpt-4o-mini-2024-07-18":                    0.075,
	"gpt-4o-mini-realtime-preview":              0.3,
	"gpt-4o-mini-realtime-preview-2024-12-17":   0.3,
	"gpt-4o-mini-search-preview":                0.075,
	"gpt-4o-realtime-preview":                   2.5,
	"gpt-4o-realtime-preview-2024-10-01":        2.5,
	"gpt-4o-realtime-preview-2024-12-17":        2.5,
	"gpt-4o-search-preview":                     1.25,
	"gpt-5":                                     0.625,
	"gpt-5-2025-08-07":                          0.625,
	"gpt-5-chat":                                0.625,
	"gpt-5-codex":                               0.625,
	"gpt-5-image":                               5,
	"gpt-5-image-mini":                          1.25,
	"gpt-5-mini":                                0.125,
	"gpt-5-mini-2025-08-07":                     0.125,
	"gpt-5-nano":                                0.025,
	"gpt-5-nano-2025-08-07":                     0.025,
	"gpt-5-pro":                                 7.5,
	"gpt-5-pro-2025-10-06":                      7.5,
	"gpt-5.1":                                   0.625,
	"gpt-5.1-2025-11-13":                        0.625,
	"gpt-5.1-chat":                              0.625,
	"gpt-5.1-codex":                             0.625,
	"gpt-5.1-codex-max":                         0.625,
	"gpt-5.1-codex-mini":                        0.125,
	"gpt-5.2":                                   0.875,
	"gpt-5.2-2025-12-11":                        0.875,
	"gpt-5.2-chat":                              0.875,
	"gpt-5.2-codex":                             0.875,
	"gpt-5.2-pro":                               10.5,
	"gpt-5.2-pro-2025-12-11":                    10.5,
	"gpt-5.3-chat":                              0.875,
	"gpt-5.3-codex":                             0.875,
	"gpt-5.4":                                   1.25,
	"gpt-5.4-2026-03-05":                        1.25,
	"gpt-5.4-image-2":                           4,
	"gpt-5.4-mini":                              0.375,
	"gpt-5.4-nano":                              0.1,
	"gpt-5.4-pro":                               15,
	"gpt-5.4-pro-2026-03-05":                    15,
	"gpt-5.5":                                   2.5,
	"gpt-5.5-pro":                               15,
	"gpt-5.6-luna":                              0.5,
	"gpt-5.6-luna-pro":                          0.5,
	"gpt-5.6-sol":                               2.5,
	"gpt-5.6-sol-pro":                           2.5,
	"gpt-5.6-terra":                             1.25,
	"gpt-5.6-terra-pro":                         1.25,
	"gpt-audio":                                 1.25,
	"gpt-audio-2025-08-28":                      1.25,
	"gpt-audio-mini":                            0.3,
	"gpt-audio-mini-2025-10-06":                 0.3,
	"gpt-audio-mini-2025-12-15":                 0.3,
	"gpt-chat-latest":                           2.5,
	"gpt-oss-120b":                              0.0185,
	"gpt-oss-20b":                               0.015,
	"gpt-oss-safeguard-20b":                     0.0375,
	"hunyuan-a13b-instruct":                     0.07,
	"hy3-preview":                               0.033,
	"kimi-k2":                                   0.285,
	"kimi-k2-0905":                              0.3,
	"kimi-k2-thinking":                          0.3,
	"kimi-k2.5":                                 0.285,
	"kimi-k2.6":                                 0.475,
	"kimi-k2.7-code":                            0.475,
	"kimi-k3":                                   1.5,
	"llama-3-sonar-large-32k-chat":              1,
	"llama-3-sonar-large-32k-online":            1,
	"llama-3-sonar-small-32k-chat":              0.2,
	"llama-3-sonar-small-32k-online":            0.2,
	"lyria-3-clip-preview":                      0,
	"lyria-3-pro-preview":                       0,
	"meta-llama/llama-3.1-70b-instruct":         0.2,
	"llama-3-1-70b-instruct":                    0.2,
	"meta-llama/llama-3.1-8b-instruct":          0.025,
	"meta-llama/llama-3.2-1b-instruct":          0.0135,
	"meta-llama/llama-3.2-3b-instruct":          0.02545,
	"meta-llama/llama-3.3-70b-instruct":         0.065,
	"meta-llama/llama-4-maverick":               0.1,
	"meta-llama/llama-4-scout":                  0.05,
	"meta-llama/llama-guard-4-12b":              0.09,
	"meta/muse-spark-1.1":                       0.625,
	"mimo-v2.5":                                 0.07,
	"mimo-v2.5-pro":                             0.2175,
	"minimax-01":                                0.1,
	"minimax-m1":                                0.2,
	"minimax-m2":                                0.1275,
	"minimax-m2-her":                            0.15,
	"minimax-m2.1":                              0.15,
	"minimax-m2.5":                              0.15,
	"minimax-m2.7":                              0.15,
	"minimax-m3":                                0.15,
	"mistralai/codestral-2508":                  0.15,
	"mistralai/devstral-2512":                   0.2,
	"mistralai/ministral-14b-2512":              0.1,
	"mistralai/ministral-3b-2512":               0.05,
	"mistralai/ministral-8b-2512":               0.075,
	"mistralai/mistral-large":                   1,
	"mistralai/mistral-large-2407":              1,
	"mistralai/mistral-large-2512":              0.25,
	"mistralai/mistral-medium-3":                0.2,
	"mistralai/mistral-medium-3-5":              0.75,
	"mistralai/mistral-medium-3.1":              0.2,
	"mistralai/mistral-nemo":                    0.0095,
	"mistralai/mistral-saba":                    0.1,
	"mistralai/mistral-small-24b-instruct-2501": 0.025,
	"mistralai/mistral-small-2603":              0.075,
	"mistralai/mistral-small-3.1-24b-instruct":  0.1755,
	"mistralai/mistral-small-3.2-24b-instruct":  0.05,
	"mistralai/mixtral-8x22b-instruct":          1,
	"mistralai/voxtral-small-24b-2507":          0.05,
	"nex-n2-pro":                                0.25,
	"nova-2-lite-v1:0":                          0.15,
	"nova-lite-v1:0":                            0.03,
	"nova-micro-v1:0":                           0.0175,
	"nova-premier-v1:0":                         1.25,
	"nova-pro-v1:0":                             0.4,
	"nousresearch/hermes-3-llama-3.1-405b":      0.5,
	"nousresearch/hermes-3-llama-3.1-70b":       0.35,
	"nousresearch/hermes-4-405b":                0.5,
	"nousresearch/hermes-4-70b":                 0.065,
	"o1":                                        7.5,
	"o1-2024-12-17":                             7.5,
	"o1-mini":                                   0.55,
	"o1-mini-2024-09-12":                        0.55,
	"o1-preview":                                7.5,
	"o1-preview-2024-09-12":                     7.5,
	"o1-pro":                                    75,
	"o1-pro-2025-03-19":                         75,
	"o3":                                        1,
	"o3-2025-04-16":                             1,
	"o3-deep-research":                          5,
	"o3-deep-research-2025-06-26":               5,
	"o3-mini":                                   0.55,
	"o3-mini-2025-01-31":                        0.55,
	"o3-mini-2025-01-31-high":                   0.55,
	"o3-mini-2025-01-31-low":                    0.55,
	"o3-mini-2025-01-31-medium":                 0.55,
	"o3-mini-high":                              0.55,
	"o3-mini-low":                               0.55,
	"o3-mini-medium":                            0.55,
	"o3-pro":                                    10,
	"o3-pro-2025-06-10":                         10,
	"o4-mini":                                   0.55,
	"o4-mini-2025-04-16":                        0.55,
	"o4-mini-deep-research":                     1,
	"o4-mini-deep-research-2025-06-26":          1,
	"o4-mini-high":                              0.55,
	"openai/gpt-oss-120b":                       0.5,
	"qwen/qwen-2.5-72b-instruct":                0.18,
	"qwen/qwen-2.5-7b-instruct":                 0.02,
	"qwen/qwen-2.5-coder-32b-instruct":          0.33,
	"qwen/qwen-plus":                            0.13,
	"qwen/qwen-plus-2025-07-28":                 0.13,
	"qwen/qwen2.5-vl-72b-instruct":              0.4,
	"qwen/qwen3-14b":                            0.05,
	"qwen/qwen3-235b-a22b":                      0.2275,
	"qwen/qwen3-235b-a22b-2507":                 0.045,
	"qwen/qwen3-235b-a22b-thinking-2507":        0.07475,
	"qwen/qwen3-30b-a3b":                        0.06,
	"qwen/qwen3-30b-a3b-instruct-2507":          0.05,
	"qwen/qwen3-30b-a3b-thinking-2507":          0.065,
	"qwen/qwen3-32b":                            0.04,
	"qwen/qwen3-8b":                             0.0585,
	"qwen/qwen3-coder":                          0.15,
	"qwen/qwen3-coder-30b-a3b-instruct":         0.035,
	"qwen3-coder-30b-a3b-instruct":              0.075,
	"qwen/qwen3-coder-flash":                    0.0975,
	"qwen/qwen3-coder-next":                     0.055,
	"qwen/qwen3-coder-plus":                     0.325,
	"qwen/qwen3-max":                            0.39,
	"qwen/qwen3-max-thinking":                   0.39,
	"qwen/qwen3-next-80b-a3b-instruct":          0.05,
	"qwen/qwen3-next-80b-a3b-thinking":          0.04875,
	"qwen/qwen3-vl-235b-a22b-instruct":          0.105,
	"qwen/qwen3-vl-235b-a22b-thinking":          0.13,
	"qwen/qwen3-vl-30b-a3b-instruct":            0.065,
	"qwen/qwen3-vl-30b-a3b-thinking":            0.065,
	"qwen/qwen3-vl-32b-instruct":                0.052,
	"qwen/qwen3-vl-8b-instruct":                 0.0585,
	"qwen/qwen3-vl-8b-thinking":                 0.0585,
	"qwen/qwen3.5-122b-a10b":                    0.13,
	"qwen/qwen3.5-27b":                          0.0975,
	"qwen/qwen3.5-35b-a3b":                      0.07,
	"qwen/qwen3.5-397b-a17b":                    0.195,
	"qwen/qwen3.5-9b":                           0.05,
	"qwen/qwen3.5-flash-02-23":                  0.0325,
	"qwen/qwen3.5-plus-02-15":                   0.13,
	"qwen/qwen3.5-plus-20260420":                0.15,
	"qwen/qwen3.6-27b":                          0.225,
	"qwen/qwen3.6-35b-a3b":                      0.07,
	"qwen/qwen3.6-flash":                        0.09375,
	"qwen/qwen3.6-max-preview":                  0.52,
	"qwen/qwen3.6-plus":                         0.1625,
	"qwen/qwen3.7-max":                          0.7375,
	"qwen/qwen3.7-plus":                         0.16,
	"seed-1.6":                                  0.125,
	"seed-1.6-flash":                            0.0375,
	"seed-2.0-lite":                             0.125,
	"seed-2.0-mini":                             0.05,
	"step-3.5-flash":                            0.05,
	"semantic_similarity_s1_v1":                 0.0715,
	"tao-8k":                                    0.002,
	"text-ada-001":                              0.2,
	"text-babbage-001":                          0.25,
	"text-curie-001":                            1,
	"text-davinci-edit-001":                     10,
	"text-moderation-latest":                    0.1,
	"text-moderation-stable":                    0.1,
	"text-search-ada-doc-001":                   10,
	"writer/palmyra-x5":                         0.3,
	"palmyra-x5":                                0.3,
	"zai-org/GLM-4.5-FP8":                       0.8,
}

var defaultModelPrice = map[string]float64{
	"imagen-3.0-generate-002": 0.03,
	"gpt-4o-mini-tts":         0.3,
}

var defaultAudioRatio = map[string]float64{
	"gemini-2.5-flash":                   3.333333,
	"gemini-2.5-flash-image":             3.333333,
	"gemini-2.5-flash-lite":              3,
	"gemini-2.5-pro":                     1,
	"gemini-2.5-pro-preview":             1,
	"gemini-2.5-pro-preview-05-06":       1,
	"gemini-3-flash-preview":             2,
	"gemini-3-pro-image":                 1,
	"gemini-3-pro-image-preview":         1,
	"gemini-3.1-flash-lite":              2,
	"gemini-3.1-flash-lite-preview":      2,
	"gemini-3.1-pro-preview":             1,
	"gemini-3.1-pro-preview-customtools": 1,
	"gemini-3.5-flash":                   2,
	"gpt-4o-audio-preview":               16,
	"gpt-4o-mini-audio-preview":          66.67,
	"gpt-4o-mini-realtime-preview":       16.67,
	"gpt-4o-realtime-preview":            8,
	"gpt-audio":                          12.8,
	"gpt-audio-2025-08-28":               12.8,
	"gpt-audio-mini":                     1,
	"gpt-audio-mini-2025-10-06":          1,
	"gpt-audio-mini-2025-12-15":          1,
	"mistralai/voxtral-small-24b-2507":   1000,
}

var defaultAudioCompletionRatio = map[string]float64{
	"gpt-4o-mini-realtime":      2,
	"gpt-4o-realtime":           2,
	"gpt-audio":                 2,
	"gpt-audio-2025-08-28":      2,
	"gpt-audio-mini":            4,
	"gpt-audio-mini-2025-10-06": 4,
	"gpt-audio-mini-2025-12-15": 4,
}

var modelPriceMap = shared.NewRWMap[string, float64]()
var modelRatioMap = shared.NewRWMap[string, float64]()
var completionRatioMap = shared.NewRWMap[string, float64]()

var defaultCompletionRatio = map[string]float64{
	"MiniMax-M2":                                4,
	"MiniMax-M2.1":                              4,
	"MiniMax-M2.5":                              6,
	"MiniMax-M2.7":                              4,
	"bytedance/ui-tars-1.5-7b":                  2,
	"claude-3-haiku":                            5,
	"claude-3-haiku-20240307":                   5,
	"claude-fable-5":                            5,
	"claude-haiku-4.5":                          5,
	"claude-opus-4":                             5,
	"claude-opus-4-20250514":                    5,
	"claude-opus-4.1":                           5,
	"claude-opus-4.5":                           5,
	"claude-opus-4.6":                           5,
	"claude-opus-4.7":                           5,
	"claude-opus-4.7-fast":                      5,
	"claude-opus-4.8":                           5,
	"claude-opus-4.8-fast":                      5,
	"claude-sonnet-4":                           5,
	"claude-sonnet-4-20250514":                  5,
	"claude-sonnet-4.5":                         5,
	"claude-sonnet-4.6":                         5,
	"claude-sonnet-5":                           5,
	"deepseek-chat":                             3.996503,
	"deepseek-chat-v3-0324":                     4.148148,
	"deepseek-chat-v3.1":                        3.8,
	"deepseek-r1":                               3.571429,
	"deepseek-r1-0528":                          4.3,
	"deepseek-r1-distill-llama-70b":             1,
	"deepseek-v3.1-terminus":                    3.703704,
	"deepseek-v3.2":                             1.486989,
	"deepseek-v3.2-exp":                         1.518519,
	"deepseek-v4-flash":                         2,
	"deepseek-v4-pro":                           2,
	"gemini-2.5-flash":                          8.333333,
	"gemini-2.5-flash-image":                    8.333333,
	"gemini-2.5-flash-lite":                     4,
	"gemini-2.5-pro":                            8,
	"gemini-2.5-pro-preview":                    8,
	"gemini-2.5-pro-preview-05-06":              8,
	"gemini-3-flash-preview":                    6,
	"gemini-3-pro-image":                        6,
	"gemini-3-pro-image-preview":                6,
	"gemini-3.1-flash-image":                    6,
	"gemini-3.1-flash-image-preview":            6,
	"gemini-3.1-flash-lite":                     6,
	"gemini-3.1-flash-lite-image":               6,
	"gemini-3.1-flash-lite-preview":             6,
	"gemini-3.1-pro-preview":                    6,
	"gemini-3.1-pro-preview-customtools":        6,
	"gemini-3.5-flash":                          6,
	"gemma-2-27b-it":                            1,
	"gemma-3-12b-it":                            3,
	"gemma-3-27b-it":                            3,
	"gemma-3-4b-it":                             2,
	"gemma-3n-e4b-it":                           2,
	"gemma-4-26b-a4b-it":                        3,
	"gemma-4-31b-it":                            2.5,
	"glm-4.5":                                   3.666667,
	"glm-4.5-air":                               5.5,
	"glm-4.5v":                                  3,
	"glm-4.6":                                   4,
	"glm-4.6v":                                  3,
	"glm-4.7":                                   3.666667,
	"glm-4.7-flash":                             6.61157,
	"glm-5":                                     3.315789,
	"glm-5-turbo":                               3.333333,
	"glm-5.1":                                   3.142857,
	"glm-5.2":                                   3.142857,
	"glm-5v-turbo":                              3.333333,
	"gpt-3.5-turbo":                             3,
	"gpt-3.5-turbo-0613":                        2,
	"gpt-3.5-turbo-16k":                         1.333333,
	"gpt-3.5-turbo-instruct":                    1.333333,
	"gpt-4":                                     2,
	"gpt-4-all":                                 2,
	"gpt-4-gizmo-*":                             2,
	"gpt-4-turbo":                               3,
	"gpt-4-turbo-preview":                       3,
	"gpt-4.1":                                   4,
	"gpt-4.1-2025-04-14":                        4,
	"gpt-4.1-mini":                              4,
	"gpt-4.1-mini-2025-04-14":                   4,
	"gpt-4.1-nano":                              4,
	"gpt-4.1-nano-2025-04-14":                   4,
	"gpt-4o":                                    4,
	"gpt-4o-2024-05-13":                         3,
	"gpt-4o-2024-08-06":                         4,
	"gpt-4o-2024-11-20":                         4,
	"gpt-4o-gizmo-*":                            3,
	"gpt-4o-mini":                               4,
	"gpt-4o-mini-2024-07-18":                    4,
	"gpt-4o-mini-search-preview":                4,
	"gpt-4o-search-preview":                     4,
	"gpt-5":                                     8,
	"gpt-5-2025-08-07":                          8,
	"gpt-5-chat":                                8,
	"gpt-5-codex":                               8,
	"gpt-5-image":                               1,
	"gpt-5-image-mini":                          0.8,
	"gpt-5-mini":                                8,
	"gpt-5-mini-2025-08-07":                     8,
	"gpt-5-nano":                                8,
	"gpt-5-nano-2025-08-07":                     8,
	"gpt-5-pro":                                 8,
	"gpt-5-pro-2025-10-06":                      8,
	"gpt-5.1":                                   8,
	"gpt-5.1-2025-11-13":                        8,
	"gpt-5.1-chat":                              8,
	"gpt-5.1-codex":                             8,
	"gpt-5.1-codex-max":                         8,
	"gpt-5.1-codex-mini":                        8,
	"gpt-5.2":                                   8,
	"gpt-5.2-2025-12-11":                        8,
	"gpt-5.2-chat":                              8,
	"gpt-5.2-codex":                             8,
	"gpt-5.2-pro":                               8,
	"gpt-5.2-pro-2025-12-11":                    8,
	"gpt-5.3-chat":                              8,
	"gpt-5.3-codex":                             8,
	"gpt-5.4":                                   6,
	"gpt-5.4-2026-03-05":                        6,
	"gpt-5.4-image-2":                           1.875,
	"gpt-5.4-mini":                              6,
	"gpt-5.4-nano":                              6.25,
	"gpt-5.4-pro":                               6,
	"gpt-5.4-pro-2026-03-05":                    6,
	"gpt-5.5":                                   6,
	"gpt-5.5-pro":                               6,
	"gpt-5.6-luna":                              6,
	"gpt-5.6-luna-pro":                          6,
	"gpt-5.6-sol":                               6,
	"gpt-5.6-sol-pro":                           6,
	"gpt-5.6-terra":                             6,
	"gpt-5.6-terra-pro":                         6,
	"gpt-audio":                                 4,
	"gpt-audio-2025-08-28":                      4,
	"gpt-audio-mini":                            4,
	"gpt-audio-mini-2025-10-06":                 4,
	"gpt-audio-mini-2025-12-15":                 4,
	"gpt-chat-latest":                           6,
	"gpt-oss-120b":                              4.594595,
	"gpt-oss-20b":                               4.333333,
	"gpt-oss-safeguard-20b":                     4,
	"hunyuan-a13b-instruct":                     4.071429,
	"hy3-preview":                               3.939394,
	"kimi-k2":                                   4.035088,
	"kimi-k2-0905":                              4.166667,
	"kimi-k2-thinking":                          4.166667,
	"kimi-k2.5":                                 5,
	"kimi-k2.6":                                 4.210526,
	"kimi-k2.7-code":                            4.666667,
	"kimi-k3":                                   5,
	"meta-llama/llama-3.1-70b-instruct":         1,
	"llama-3-1-70b-instruct":                    1,
	"meta-llama/llama-3.1-8b-instruct":          1.6,
	"meta-llama/llama-3.2-1b-instruct":          7.444444,
	"meta-llama/llama-3.2-3b-instruct":          6.581532,
	"meta-llama/llama-3.3-70b-instruct":         3.076923,
	"meta-llama/llama-4-maverick":               4,
	"meta-llama/llama-4-scout":                  3,
	"meta-llama/llama-guard-4-12b":              1,
	"meta/muse-spark-1.1":                       3.4,
	"mimo-v2.5":                                 2,
	"mimo-v2.5-pro":                             2,
	"minimax-01":                                5.5,
	"minimax-m1":                                5.5,
	"minimax-m2":                                4,
	"minimax-m2-her":                            4,
	"minimax-m2.1":                              4,
	"minimax-m2.5":                              4.0,
	"minimax-m2.7":                              4,
	"minimax-m3":                                4,
	"mistralai/codestral-2508":                  3,
	"mistralai/devstral-2512":                   5,
	"mistralai/ministral-14b-2512":              1,
	"mistralai/ministral-3b-2512":               1,
	"mistralai/ministral-8b-2512":               1,
	"mistralai/mistral-large":                   3,
	"mistralai/mistral-large-2407":              3,
	"mistralai/mistral-large-2512":              3,
	"mistralai/mistral-medium-3":                5,
	"mistralai/mistral-medium-3-5":              5,
	"mistralai/mistral-medium-3.1":              5,
	"mistralai/mistral-nemo":                    1.578947,
	"mistralai/mistral-saba":                    3,
	"mistralai/mistral-small-24b-instruct-2501": 1.6,
	"mistralai/mistral-small-2603":              4,
	"mistralai/mistral-small-3.1-24b-instruct":  1.581197,
	"mistralai/mistral-small-3.2-24b-instruct":  3,
	"mistralai/mixtral-8x22b-instruct":          3,
	"mistralai/voxtral-small-24b-2507":          3,
	"nex-n2-pro":                                5.0,
	"nova-2-lite-v1:0":                          8.333333,
	"nova-lite-v1:0":                            4.0,
	"nova-micro-v1:0":                           4.0,
	"nova-premier-v1:0":                         5.0,
	"nova-pro-v1:0":                             4.0,
	"nousresearch/hermes-3-llama-3.1-405b":      1,
	"nousresearch/hermes-3-llama-3.1-70b":       1,
	"nousresearch/hermes-4-405b":                3,
	"nousresearch/hermes-4-70b":                 3.076923,
	"o1":                                        4,
	"o1-pro":                                    4,
	"o3":                                        4,
	"o3-deep-research":                          4,
	"o3-mini":                                   4,
	"o3-mini-high":                              4,
	"o3-pro":                                    4,
	"o4-mini":                                   4,
	"o4-mini-deep-research":                     4,
	"o4-mini-high":                              4,
	"qwen/qwen-2.5-72b-instruct":                1.111111,
	"qwen/qwen-2.5-7b-instruct":                 2.5,
	"qwen/qwen-2.5-coder-32b-instruct":          1.515152,
	"qwen/qwen-plus":                            3,
	"qwen/qwen-plus-2025-07-28":                 3,
	"qwen/qwen2.5-vl-72b-instruct":              1.25,
	"qwen/qwen3-14b":                            2.4,
	"qwen/qwen3-235b-a22b":                      4,
	"qwen/qwen3-235b-a22b-2507":                 6.111111,
	"qwen/qwen3-235b-a22b-thinking-2507":        10,
	"qwen/qwen3-30b-a3b":                        4.166667,
	"qwen/qwen3-30b-a3b-instruct-2507":          3,
	"qwen/qwen3-30b-a3b-thinking-2507":          12,
	"qwen/qwen3-32b":                            3.5,
	"qwen/qwen3-8b":                             3.888889,
	"qwen/qwen3-coder":                          3.333333,
	"qwen/qwen3-coder-30b-a3b-instruct":         3.857143,
	"qwen3-coder-30b-a3b-instruct":              0.3,
	"qwen/qwen3-coder-flash":                    5,
	"qwen/qwen3-coder-next":                     7.272727,
	"qwen/qwen3-coder-plus":                     5,
	"qwen/qwen3-max":                            5,
	"qwen/qwen3-max-thinking":                   5,
	"qwen/qwen3-next-80b-a3b-instruct":          11,
	"qwen/qwen3-next-80b-a3b-thinking":          8,
	"qwen/qwen3-vl-235b-a22b-instruct":          9.047619,
	"qwen/qwen3-vl-235b-a22b-thinking":          10,
	"qwen/qwen3-vl-30b-a3b-instruct":            4,
	"qwen/qwen3-vl-30b-a3b-thinking":            12,
	"qwen/qwen3-vl-32b-instruct":                4,
	"qwen/qwen3-vl-8b-instruct":                 3.888889,
	"qwen/qwen3-vl-8b-thinking":                 11.666667,
	"qwen/qwen3.5-122b-a10b":                    8,
	"qwen/qwen3.5-27b":                          8,
	"qwen/qwen3.5-35b-a3b":                      7.142857,
	"qwen/qwen3.5-397b-a17b":                    6,
	"qwen/qwen3.5-9b":                           1.5,
	"qwen/qwen3.5-flash-02-23":                  4,
	"qwen/qwen3.5-plus-02-15":                   6,
	"qwen/qwen3.5-plus-20260420":                6,
	"qwen/qwen3.6-27b":                          6,
	"qwen/qwen3.6-35b-a3b":                      7.142857,
	"qwen/qwen3.6-flash":                        6,
	"qwen/qwen3.6-max-preview":                  6,
	"qwen/qwen3.6-plus":                         6,
	"qwen/qwen3.7-max":                          3,
	"qwen/qwen3.7-plus":                         4,
	"palmyra-x5":                                3,
	"seed-1.6":                                  8.0,
	"seed-1.6-flash":                            4.0,
	"seed-2.0-lite":                             8.0,
	"seed-2.0-mini":                             4.0,
	"step-3.5-flash":                            3.0,
}

// InitRatioSettings initializes all model related settings maps
func InitRatioSettings() {
	modelPriceMap.AddAll(defaultModelPrice)
	modelRatioMap.AddAll(defaultModelRatio)
	completionRatioMap.AddAll(defaultCompletionRatio)
	cacheRatioMap.AddAll(defaultCacheRatio)
	createCacheRatioMap.AddAll(defaultCreateCacheRatio)
	audioRatioMap.AddAll(defaultAudioRatio)
	audioCompletionRatioMap.AddAll(defaultAudioCompletionRatio)
	contextPricingMap.AddAll(defaultContextPricing)
}

func GetModelPriceMap() map[string]float64 {
	return modelPriceMap.ReadAll()
}

func ModelPrice2JSONString() string {
	return modelPriceMap.MarshalJSONString()
}

func UpdateModelPriceByJSONString(jsonStr string) error {
	return shared.LoadFromJsonString(modelPriceMap, jsonStr)
}

// GetModelPrice 返回模型的价格，如果模型不存在则返回-1，false
func GetModelPrice(name string, printErr bool) (float64, bool) {
	name = FormatMatchingModelName(name)

	if strings.HasSuffix(name, CompactModelSuffix) {
		price, ok := modelPriceMap.Get(CompactWildcardModelKey)
		if !ok {
			if printErr {
				common.SysError("model price not found: " + name)
			}
			return -1, false
		}
		return price, true
	}

	price, ok := modelPriceMap.Get(name)
	if !ok {
		if printErr {
			common.SysError("model price not found: " + name)
		}
		return -1, false
	}
	return price, true
}

func UpdateModelRatioByJSONString(jsonStr string) error {
	return shared.LoadFromJsonString(modelRatioMap, jsonStr)
}

// 处理带有思考预算的模型名称，方便统一定价
func handleThinkingBudgetModel(name, prefix, wildcard string) string {
	if strings.HasPrefix(name, prefix) && strings.Contains(name, "-thinking-") {
		return wildcard
	}
	return name
}

func GetModelRatio(name string) (float64, bool, string) {
	name = FormatMatchingModelName(name)

	ratio, ok := modelRatioMap.Get(name)
	if !ok {
		if strings.HasSuffix(name, CompactModelSuffix) {
			if wildcardRatio, ok := modelRatioMap.Get(CompactWildcardModelKey); ok {
				return wildcardRatio, true, name
			}
			//return 0, true, name
		}
		return 37.5, false, name
	}
	return ratio, true, name
}

func DefaultModelRatio2JSONString() string {
	jsonBytes, err := jsonx.Marshal(defaultModelRatio)
	if err != nil {
		common.SysError("error marshalling model ratio: " + err.Error())
	}
	return string(jsonBytes)
}

func GetDefaultModelRatioMap() map[string]float64 {
	return defaultModelRatio
}

func GetDefaultModelPriceMap() map[string]float64 {
	return defaultModelPrice
}

func CompletionRatio2JSONString() string {
	return completionRatioMap.MarshalJSONString()
}

func UpdateCompletionRatioByJSONString(jsonStr string) error {
	return shared.LoadFromJsonString(completionRatioMap, jsonStr)
}

func GetCompletionRatio(name string) float64 {
	name = FormatMatchingModelName(name)

	if strings.Contains(name, "/") {
		if ratio, ok := completionRatioMap.Get(name); ok {
			return ratio
		}
	}
	hardCodedRatio, contain := getHardcodedCompletionModelRatio(name)
	if contain {
		return hardCodedRatio
	}
	if ratio, ok := completionRatioMap.Get(name); ok {
		return ratio
	}
	return hardCodedRatio
}

func getHardcodedCompletionModelRatio(name string) (float64, bool) {

	isReservedModel := strings.HasSuffix(name, "-all") || strings.HasSuffix(name, "-gizmo-*")
	if isReservedModel {
		return 2, false
	}

	if strings.HasPrefix(name, "gpt-") {
		if strings.HasPrefix(name, "gpt-4o") {
			if name == "gpt-4o-2024-05-13" {
				return 3, true
			}
			if strings.HasPrefix(name, "gpt-4o-mini-tts") {
				return 20, false
			}
			return 4, false
		}
		// gpt-5 匹配
		if strings.HasPrefix(name, "gpt-5") {
			return 8, true
		}
		// gpt-4.5-preview匹配
		if strings.HasPrefix(name, "gpt-4.5-preview") {
			return 2, true
		}
		if strings.HasPrefix(name, "gpt-4-turbo") || strings.HasSuffix(name, "gpt-4-1106") || strings.HasSuffix(name, "gpt-4-1105") {
			return 3, true
		}
		// 没有特殊标记的 gpt-4 模型默认倍率为 2
		return 2, false
	}
	if strings.HasPrefix(name, "o1") || strings.HasPrefix(name, "o3") {
		return 4, true
	}
	if name == "chatgpt-4o-latest" {
		return 3, true
	}

	if strings.Contains(name, "claude-3") {
		return 5, true
	} else if strings.Contains(name, "claude-sonnet-4") || strings.Contains(name, "claude-opus-4") || strings.Contains(name, "claude-haiku-4") {
		return 5, true
	}

	if strings.HasPrefix(name, "gpt-3.5") {
		if name == "gpt-3.5-turbo" || strings.HasSuffix(name, "0125") {
			// https://openai.com/blog/new-embedding-models-and-api-updates
			// Updated GPT-3.5 Turbo model and lower pricing
			return 3, true
		}
		if strings.HasSuffix(name, "1106") {
			return 2, true
		}
		return 4.0 / 3.0, true
	}
	if strings.HasPrefix(name, "gemini-") {
		if strings.HasPrefix(name, "gemini-1.5") {
			return 4, true
		} else if strings.HasPrefix(name, "gemini-2.0") {
			return 4, true
		} else if strings.HasPrefix(name, "gemini-2.5-pro") { // 移除preview来增加兼容性，这里假设正式版的倍率和preview一致
			return 8, false
		} else if strings.HasPrefix(name, "gemini-2.5-flash") { // 处理不同的flash模型倍率
			if strings.HasPrefix(name, "gemini-2.5-flash-preview") {
				if strings.HasSuffix(name, "-nothinking") {
					return 4, false
				}
				return 3.5 / 0.15, false
			}
			if strings.HasPrefix(name, "gemini-2.5-flash-lite") {
				return 4, false
			}
			return 2.5 / 0.3, false
		} else if strings.HasPrefix(name, "gemini-robotics-er-1.5") {
			return 2.5 / 0.3, false
		} else if strings.HasPrefix(name, "gemini-3.5-flash-lite") {
			return 25.0 / 3.0, false
		} else if strings.HasPrefix(name, "gemini-3.5-flash") {
			return 6, false
		} else if strings.HasPrefix(name, "gemini-3.6-flash") || strings.HasPrefix(name, "gemini-3.8-flash") {
			return 5, false
		} else if strings.HasPrefix(name, "gemini-4-argon") {
			return 5, false
		} else if strings.HasPrefix(name, "gemini-nano-banana") {
			return 5, false
		} else if strings.HasPrefix(name, "gemini-3-pro") {
			if strings.HasPrefix(name, "gemini-3-pro-image") {
				return 60, false
			}
			return 6, false
		}
		return 4, false
	}
	// hint only applies official 4x ratio, since open-source model providers set their own prices
	if strings.HasPrefix(name, "ERNIE-Speed-") {
		return 2, true
	} else if strings.HasPrefix(name, "ERNIE-Lite-") {
		return 2, true
	} else if strings.HasPrefix(name, "ERNIE-Character") {
		return 2, true
	} else if strings.HasPrefix(name, "ERNIE-Functions") {
		return 2, true
	}
	switch name {
	case "llama2-70b-4096":
		return 0.8 / 0.64, true
	case "llama3-8b-8192":
		return 2, true
	case "llama3-70b-8192":
		return 0.79 / 0.59, true
	}
	return 1, false
}

func GetAudioRatio(name string) float64 {
	name = FormatMatchingModelName(name)
	if ratio, ok := audioRatioMap.Get(name); ok {
		return ratio
	}
	return 1
}

func GetAudioCompletionRatio(name string) float64 {
	name = FormatMatchingModelName(name)
	if ratio, ok := audioCompletionRatioMap.Get(name); ok {
		return ratio
	}
	return 1
}

func ContainsAudioRatio(name string) bool {
	name = FormatMatchingModelName(name)
	_, ok := audioRatioMap.Get(name)
	return ok
}

func ContainsAudioCompletionRatio(name string) bool {
	name = FormatMatchingModelName(name)
	_, ok := audioCompletionRatioMap.Get(name)
	return ok
}

func ModelRatio2JSONString() string {
	return modelRatioMap.MarshalJSONString()
}

var audioRatioMap = shared.NewRWMap[string, float64]()
var audioCompletionRatioMap = shared.NewRWMap[string, float64]()

func AudioRatio2JSONString() string {
	return audioRatioMap.MarshalJSONString()
}

func UpdateAudioRatioByJSONString(jsonStr string) error {
	return shared.LoadFromJsonString(audioRatioMap, jsonStr)
}

func AudioCompletionRatio2JSONString() string {
	return audioCompletionRatioMap.MarshalJSONString()
}

func UpdateAudioCompletionRatioByJSONString(jsonStr string) error {
	return shared.LoadFromJsonString(audioCompletionRatioMap, jsonStr)
}

func GetModelRatioCopy() map[string]float64 {
	return modelRatioMap.ReadAll()
}

func GetModelPriceCopy() map[string]float64 {
	return modelPriceMap.ReadAll()
}

func GetCompletionRatioCopy() map[string]float64 {
	return completionRatioMap.ReadAll()
}

// 转换模型名，减少渠道必须配置各种带参数模型
func FormatMatchingModelName(name string) string {

	if strings.HasPrefix(name, "gemini-2.5-flash-lite") {
		name = handleThinkingBudgetModel(name, "gemini-2.5-flash-lite", "gemini-2.5-flash-lite-thinking-*")
	} else if strings.HasPrefix(name, "gemini-2.5-flash") {
		name = handleThinkingBudgetModel(name, "gemini-2.5-flash", "gemini-2.5-flash-thinking-*")
	} else if strings.HasPrefix(name, "gemini-2.5-pro") {
		name = handleThinkingBudgetModel(name, "gemini-2.5-pro", "gemini-2.5-pro-thinking-*")
	}

	if strings.HasPrefix(name, "gpt-4-gizmo") {
		name = "gpt-4-gizmo-*"
	}
	if strings.HasPrefix(name, "gpt-4o-gizmo") {
		name = "gpt-4o-gizmo-*"
	}
	return name
}

// result: 倍率or价格， usePrice， exist
func GetModelRatioOrPrice(model string) (float64, bool, bool) { // price or ratio
	price, usePrice := GetModelPrice(model, false)
	if usePrice {
		return price, true, true
	}
	modelRatio, success, _ := GetModelRatio(model)
	if success {
		return modelRatio, false, true
	}
	if contextPricing, ok := GetContextPricingConfig(model); ok && contextPricing.Enabled && len(contextPricing.Tiers) > 0 {
		return contextPricing.Tiers[0].ModelRatio, false, true
	}
	return 37.5, false, false
}
