package middleware

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/NookMux/NookMux/internal/common"
	"github.com/NookMux/NookMux/internal/httpapi"
	"github.com/NookMux/NookMux/internal/infra/log"
	"github.com/NookMux/NookMux/pkg/jsonx"
	"github.com/gin-gonic/gin"
)

// ErrTokenModelMappingCycle 令牌级模型重定向规则中检测到环形引用。
var ErrTokenModelMappingCycle = errors.New("令牌模型重定向规则存在环形引用")

// resolveTokenModelMapping 在令牌级映射表中查找 originModel 的最终目标模型，
// 支持链式映射（A -> B -> C）。返回值为最终模型与是否命中映射。
//
// 环与自引用语义与渠道级 ModelMappedHelper 保持一致：
//   - 起点自引用（A -> A 且 A 为请求模型）：视为未配置映射，原样透传；
//   - 链中自引用（A -> B -> B）：停在 B，视为映射完成；
//   - 环形引用（A -> B -> A）：返回 ErrTokenModelMappingCycle，由调用方拦截请求。
func resolveTokenModelMapping(modelMap map[string]string, originModel string) (string, error) {
	currentModel := originModel
	visited := map[string]bool{currentModel: true}
	mapped := false
	for {
		next, exists := modelMap[currentModel]
		if !exists || next == "" {
			break
		}
		if visited[next] {
			if next == currentModel {
				if currentModel == originModel {
					// 起点自引用：视为未映射
					return originModel, nil
				}
				// 链中自引用：停在当前模型
				return currentModel, nil
			}
			return "", fmt.Errorf("%w: %s -> %s", ErrTokenModelMappingCycle, currentModel, next)
		}
		visited[next] = true
		currentModel = next
		mapped = true
	}
	if !mapped {
		return originModel, nil
	}
	return currentModel, nil
}

// applyTokenModelMapping 在分发前执行令牌级入站模型重定向：
// 若当前令牌配置了 model_mapping 且请求模型命中映射，则改写请求中的模型名
// （JSON body 的 model 字段 / 表单字段 / Gemini 路径段 / realtime 查询参数），
// 并同步更新 modelRequest.Model，使后续的白名单校验、查价预扣费、渠道调度、
// 日志统计全部按重定向后的模型执行。
//
// 对于无法安全改写来源的请求（multipart 表单、路径参数兜底等），跳过映射并
// 保持原始模型透传，保证计费与上游请求始终一致。返回 error 时调用方应拦截请求。
func applyTokenModelMapping(c *gin.Context, modelRequest *ModelRequest) error {
	modelMap, ok := httpapi.GetContextKeyType[map[string]string](c, common.ContextKeyTokenModelMapping)
	if !ok || len(modelMap) == 0 || modelRequest == nil || modelRequest.Model == "" {
		return nil
	}

	targetModel, err := resolveTokenModelMapping(modelMap, modelRequest.Model)
	if err != nil {
		return err
	}
	if targetModel == modelRequest.Model {
		return nil
	}

	rewritten, err := rewriteRequestModel(c, targetModel)
	if err != nil {
		return err
	}
	if !rewritten {
		// 请求模型来源无法安全改写（multipart 表单/路径参数兜底等），
		// 保持原始模型透传，避免计费模型与上游请求模型不一致。
		log.LogDebug(c, fmt.Sprintf("token model mapping skipped: model source of %s cannot be rewritten", c.Request.URL.Path))
		return nil
	}

	modelRequest.Model = targetModel
	return nil
}

// rewriteRequestModel 把请求中的模型名替换为 targetModel。
// 依据请求形态分别处理 JSON body、urlencoded 表单、Gemini 路径段与
// realtime 查询参数；返回是否完成了改写。
func rewriteRequestModel(c *gin.Context, targetModel string) (bool, error) {
	path := c.Request.URL.Path
	contentType := c.Request.Header.Get("Content-Type")

	// Gemini 原生路径：/v1beta/models/{model}:{action} 或 /v1/models/{model}:{action}
	if strings.HasPrefix(path, "/v1beta/models/") || strings.HasPrefix(path, "/v1/models/") {
		return rewriteGeminiPathModel(c, targetModel)
	}
	// realtime：模型来自 query 参数
	if strings.HasPrefix(path, "/v1/realtime") {
		return rewriteQueryModel(c, targetModel)
	}
	if strings.HasPrefix(contentType, "application/json") {
		return rewriteJsonBodyModel(c, targetModel)
	}
	if strings.Contains(contentType, gin.MIMEPOSTForm) {
		return rewriteFormBodyModel(c, targetModel)
	}
	// multipart（文件上传类接口）与路径参数兜底（/v1/engines/:model/embeddings）
	// 不做安全改写，由调用方保持透传。
	return false, nil
}

// rewriteJsonBodyModel 改写 JSON body 顶层 model 字段。
// 使用 map[string]json.RawMessage 承载其余字段的原始字节，避免数字精度等
// 序列化差异；仅调用 jsonx 的包装函数完成序列化/反序列化。
func rewriteJsonBodyModel(c *gin.Context, targetModel string) (bool, error) {
	body, err := httpapi.GetRequestBody(c)
	if err != nil {
		return false, err
	}
	bodyMap := make(map[string]json.RawMessage)
	if err := jsonx.Unmarshal(body, &bodyMap); err != nil {
		// 非 JSON 对象（数组/畸形 body）：留给下游请求校验拦截，不改写。
		return false, nil
	}
	rawModel, ok := bodyMap["model"]
	if !ok {
		// body 未显式携带 model（由分发逻辑填充默认值），不改写。
		return false, nil
	}
	var modelStr string
	if err := jsonx.Unmarshal(rawModel, &modelStr); err != nil {
		// model 不是字符串：畸形请求，交给下游校验拦截。
		return false, nil
	}
	modelJson, err := jsonx.Marshal(targetModel)
	if err != nil {
		return false, err
	}
	bodyMap["model"] = modelJson
	newBody, err := jsonx.Marshal(bodyMap)
	if err != nil {
		return false, err
	}
	if err := httpapi.SetRequestBody(c, newBody); err != nil {
		return false, err
	}
	return true, nil
}

// rewriteFormBodyModel 改写 application/x-www-form-urlencoded body 的 model 字段。
func rewriteFormBodyModel(c *gin.Context, targetModel string) (bool, error) {
	body, err := httpapi.GetRequestBody(c)
	if err != nil {
		return false, err
	}
	values, err := url.ParseQuery(string(body))
	if err != nil {
		return false, nil
	}
	if _, ok := values["model"]; !ok {
		return false, nil
	}
	values.Set("model", targetModel)
	if err := httpapi.SetRequestBody(c, []byte(values.Encode())); err != nil {
		return false, err
	}
	return true, nil
}

// rewriteGeminiPathModel 改写 Gemini 路径中的模型段，保留 :action 后缀。
func rewriteGeminiPathModel(c *gin.Context, targetModel string) (bool, error) {
	path := c.Request.URL.Path
	modelsPrefix := "/models/"
	modelsIndex := strings.Index(path, modelsPrefix)
	if modelsIndex == -1 {
		return false, nil
	}
	start := modelsIndex + len(modelsPrefix)
	if start >= len(path) {
		return false, nil
	}
	rest := path[start:]
	colonIndex := strings.Index(rest, ":")
	actionPart := ""
	modelPart := rest
	if colonIndex != -1 {
		modelPart = rest[:colonIndex]
		actionPart = rest[colonIndex:]
	}
	if modelPart == "" {
		return false, nil
	}
	c.Request.URL.Path = path[:start] + targetModel + actionPart
	// gin 路由已匹配完成，但 RequestURI 可能被下游转发/日志使用，同步更新。
	c.Request.RequestURI = c.Request.URL.RequestURI()
	return true, nil
}

// rewriteQueryModel 改写 realtime 请求 query 中的 model 参数。
func rewriteQueryModel(c *gin.Context, targetModel string) (bool, error) {
	query := c.Request.URL.Query()
	if _, ok := query["model"]; !ok {
		return false, nil
	}
	query.Set("model", targetModel)
	c.Request.URL.RawQuery = query.Encode()
	c.Request.RequestURI = c.Request.URL.RequestURI()
	return true, nil
}
