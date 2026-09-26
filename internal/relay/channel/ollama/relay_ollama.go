package ollama

import (
	"bufio"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	channelconstant "github.com/NookMux/NookMux/internal/domain/channel/constant"
	"github.com/NookMux/NookMux/internal/i18n"
	httpclient "github.com/NookMux/NookMux/internal/infra/httpclient"
	"github.com/NookMux/NookMux/internal/relay/helper"
	"github.com/NookMux/NookMux/pkg/jsonx"
)

// resolveBaseURL 将套餐简写（如 "ollama-coding-plan"）解析为实际 URL。
// 若非套餐地址则原样返回。
func resolveBaseURL(baseURL string) string {
	baseURL = strings.TrimSpace(baseURL)
	if plan, ok := channelconstant.ChannelSpecialBases[baseURL]; ok {
		if plan.OpenAIBaseURL != "" {
			return plan.OpenAIBaseURL
		}
	}
	return baseURL
}

// FetchOllamaModels 拉取 Ollama 模型列表；proxyURL 非空时请求经渠道代理发出。
func FetchOllamaModels(lang, baseURL, apiKey, proxyURL string) ([]OllamaModel, error) {
	trimmedBase := strings.TrimRight(resolveBaseURL(baseURL), "/")
	url := fmt.Sprintf("%s/v1/models", trimmedBase)

	client, err := newOllamaHttpClient(proxyURL, 0)
	if err != nil {
		return nil, errors.New(i18n.Translate(lang, i18n.MsgOllamaProxyClientCreateFailed, map[string]any{"Error": err.Error()}))
	}
	request, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, errors.New(i18n.Translate(lang, i18n.MsgOllamaRequestCreateFailed, map[string]any{"Error": err.Error()}))
	}

	// 模型查询改走 OpenAI 兼容接口，便于与新的标准转发链路保持一致。
	if apiKey != "" {
		request.Header.Set("Authorization", "Bearer "+apiKey)
	}

	response, err := client.Do(request)
	if err != nil {
		return nil, errors.New(i18n.Translate(lang, i18n.MsgOllamaRequestFailed, map[string]any{"Error": err.Error()}))
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		body, _ := helper.ReadErrorResponseBody(response.Body)
		return nil, errors.New(i18n.Translate(lang, i18n.MsgOllamaUnexpectedStatus, map[string]any{
			"Status": response.StatusCode,
			"Body":   string(body),
		}))
	}

	var listResponse OllamaOpenAIModelListResponse
	body, err := helper.ReadModelListResponseBody(response.Body)
	if err != nil {
		return nil, errors.New(i18n.Translate(lang, i18n.MsgOllamaResponseReadFailed, map[string]any{"Error": err.Error()}))
	}

	err = jsonx.Unmarshal(body, &listResponse)
	if err != nil {
		return nil, errors.New(i18n.Translate(lang, i18n.MsgOllamaResponseParseFailed, map[string]any{"Error": err.Error()}))
	}

	models := make([]OllamaModel, 0, len(listResponse.Data))
	for _, item := range listResponse.Data {
		model := OllamaModel{
			Name:    item.ID,
			Created: item.Created,
			OwnedBy: item.OwnedBy,
		}
		if item.Created > 0 {
			model.ModifiedAt = time.Unix(item.Created, 0).UTC().Format(time.RFC3339)
		}
		models = append(models, model)
	}

	return models, nil
}

// ollamaLongPullTimeout 大模型拉取耗时较长，需要独立的超时配置。
const ollamaLongPullTimeout = 30 * time.Minute

// newOllamaHttpClient 返回经渠道代理的客户端；超时会覆盖共享客户端，需拷贝实例。
func newOllamaHttpClient(proxyURL string, timeout time.Duration) (*http.Client, error) {
	client, err := httpclient.GetHttpClientWithProxy(proxyURL)
	if err != nil {
		return nil, err
	}
	if timeout <= 0 {
		return client, nil
	}
	return &http.Client{
		Transport:     client.Transport,
		CheckRedirect: client.CheckRedirect,
		Timeout:       timeout,
	}, nil
}

// 拉取 Ollama 模型 (非流式)
func PullOllamaModel(lang, baseURL, apiKey, proxyURL, modelName string) error {
	url := fmt.Sprintf("%s/api/pull", resolveBaseURL(baseURL))

	pullRequest := OllamaPullRequest{
		Name:   modelName,
		Stream: false, // 非流式，简化处理
	}

	requestBody, err := jsonx.Marshal(pullRequest)
	if err != nil {
		return errors.New(i18n.Translate(lang, i18n.MsgOllamaRequestSerializeFailed, map[string]any{"Error": err.Error()}))
	}

	client, err := newOllamaHttpClient(proxyURL, ollamaLongPullTimeout)
	if err != nil {
		return errors.New(i18n.Translate(lang, i18n.MsgOllamaProxyClientCreateFailed, map[string]any{"Error": err.Error()}))
	}
	request, err := http.NewRequest("POST", url, strings.NewReader(string(requestBody)))
	if err != nil {
		return errors.New(i18n.Translate(lang, i18n.MsgOllamaRequestCreateFailed, map[string]any{"Error": err.Error()}))
	}

	request.Header.Set("Content-Type", "application/json")
	if apiKey != "" {
		request.Header.Set("Authorization", "Bearer "+apiKey)
	}

	response, err := client.Do(request)
	if err != nil {
		return errors.New(i18n.Translate(lang, i18n.MsgOllamaRequestFailed, map[string]any{"Error": err.Error()}))
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		body, _ := helper.ReadErrorResponseBody(response.Body)
		return errors.New(i18n.Translate(lang, i18n.MsgOllamaPullModelStatusFailed, map[string]any{
			"Status": response.StatusCode,
			"Body":   string(body),
		}))
	}

	return nil
}

// 流式拉取 Ollama 模型 (支持进度回调)
func PullOllamaModelStream(lang, baseURL, apiKey, proxyURL, modelName string, progressCallback func(OllamaPullResponse)) error {
	url := fmt.Sprintf("%s/api/pull", resolveBaseURL(baseURL))

	pullRequest := OllamaPullRequest{
		Name:   modelName,
		Stream: true, // 启用流式
	}

	requestBody, err := jsonx.Marshal(pullRequest)
	if err != nil {
		return errors.New(i18n.Translate(lang, i18n.MsgOllamaRequestSerializeFailed, map[string]any{"Error": err.Error()}))
	}

	client, err := newOllamaHttpClient(proxyURL, time.Hour) // 1小时超时，支持超大模型
	if err != nil {
		return errors.New(i18n.Translate(lang, i18n.MsgOllamaClientCreateFailed, map[string]any{"Error": err.Error()}))
	}
	request, err := http.NewRequest("POST", url, strings.NewReader(string(requestBody)))
	if err != nil {
		return errors.New(i18n.Translate(lang, i18n.MsgOllamaRequestCreateFailed, map[string]any{"Error": err.Error()}))
	}

	request.Header.Set("Content-Type", "application/json")
	if apiKey != "" {
		request.Header.Set("Authorization", "Bearer "+apiKey)
	}

	response, err := client.Do(request)
	if err != nil {
		return errors.New(i18n.Translate(lang, i18n.MsgOllamaRequestFailed, map[string]any{"Error": err.Error()}))
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		body, _ := helper.ReadErrorResponseBody(response.Body)
		return errors.New(i18n.Translate(lang, i18n.MsgOllamaPullModelStatusFailed, map[string]any{
			"Status": response.StatusCode,
			"Body":   string(body),
		}))
	}

	// 读取流式响应
	scanner := bufio.NewScanner(response.Body)
	successful := false
	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}

		var pullResponse OllamaPullResponse
		if err := jsonx.Unmarshal([]byte(line), &pullResponse); err != nil {
			continue // 忽略解析失败的行
		}

		if progressCallback != nil {
			progressCallback(pullResponse)
		}

		// 检查是否出现错误或完成
		if strings.EqualFold(pullResponse.Status, "error") {
			return errors.New(i18n.Translate(lang, i18n.MsgOllamaPullModelFailed, map[string]any{"Detail": strings.TrimSpace(line)}))
		}
		if strings.EqualFold(pullResponse.Status, "success") {
			successful = true
			break
		}
	}

	if err := scanner.Err(); err != nil {
		return errors.New(i18n.Translate(lang, i18n.MsgOllamaPullStreamReadFailed, map[string]any{"Error": err.Error()}))
	}

	if !successful {
		return errors.New(i18n.Translate(lang, i18n.MsgOllamaPullNotCompleted))
	}

	return nil
}

// 删除 Ollama 模型
func DeleteOllamaModel(lang, baseURL, apiKey, proxyURL, modelName string) error {
	url := fmt.Sprintf("%s/api/delete", resolveBaseURL(baseURL))

	deleteRequest := OllamaDeleteRequest{
		Name: modelName,
	}

	requestBody, err := jsonx.Marshal(deleteRequest)
	if err != nil {
		return errors.New(i18n.Translate(lang, i18n.MsgOllamaRequestSerializeFailed, map[string]any{"Error": err.Error()}))
	}

	client, err := newOllamaHttpClient(proxyURL, 0)
	if err != nil {
		return errors.New(i18n.Translate(lang, i18n.MsgOllamaClientCreateFailed, map[string]any{"Error": err.Error()}))
	}
	request, err := http.NewRequest("DELETE", url, strings.NewReader(string(requestBody)))
	if err != nil {
		return errors.New(i18n.Translate(lang, i18n.MsgOllamaRequestCreateFailed, map[string]any{"Error": err.Error()}))
	}

	request.Header.Set("Content-Type", "application/json")
	if apiKey != "" {
		request.Header.Set("Authorization", "Bearer "+apiKey)
	}

	response, err := client.Do(request)
	if err != nil {
		return errors.New(i18n.Translate(lang, i18n.MsgOllamaRequestFailed, map[string]any{"Error": err.Error()}))
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		body, _ := helper.ReadErrorResponseBody(response.Body)
		return errors.New(i18n.Translate(lang, i18n.MsgOllamaDeleteModelFailed, map[string]any{
			"Status": response.StatusCode,
			"Body":   string(body),
		}))
	}

	return nil
}

func FetchOllamaVersion(lang, baseURL, apiKey, proxyURL string) (string, error) {
	trimmedBase := strings.TrimRight(resolveBaseURL(baseURL), "/")
	if trimmedBase == "" {
		return "", errors.New(i18n.Translate(lang, i18n.MsgOllamaBaseURLEmpty))
	}

	url := fmt.Sprintf("%s/api/version", trimmedBase)

	client, err := newOllamaHttpClient(proxyURL, 10*time.Second)
	if err != nil {
		return "", errors.New(i18n.Translate(lang, i18n.MsgOllamaProxyClientCreateFailed, map[string]any{"Error": err.Error()}))
	}
	request, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", errors.New(i18n.Translate(lang, i18n.MsgOllamaRequestCreateFailed, map[string]any{"Error": err.Error()}))
	}

	if apiKey != "" {
		request.Header.Set("Authorization", "Bearer "+apiKey)
	}

	response, err := client.Do(request)
	if err != nil {
		return "", errors.New(i18n.Translate(lang, i18n.MsgOllamaRequestFailed, map[string]any{"Error": err.Error()}))
	}
	defer response.Body.Close()

	body, err := helper.ReadModelListResponseBody(response.Body)
	if err != nil {
		return "", errors.New(i18n.Translate(lang, i18n.MsgOllamaResponseReadFailed, map[string]any{"Error": err.Error()}))
	}

	if response.StatusCode != http.StatusOK {
		return "", errors.New(i18n.Translate(lang, i18n.MsgOllamaVersionQueryFailed, map[string]any{
			"Status": response.StatusCode,
			"Body":   string(body),
		}))
	}

	var versionResp struct {
		Version string `json:"version"`
	}

	if err := jsonx.Unmarshal(body, &versionResp); err != nil {
		return "", errors.New(i18n.Translate(lang, i18n.MsgOllamaResponseParseFailed, map[string]any{"Error": err.Error()}))
	}

	if versionResp.Version == "" {
		return "", errors.New(i18n.Translate(lang, i18n.MsgOllamaVersionNotReturned))
	}

	return versionResp.Version, nil
}
