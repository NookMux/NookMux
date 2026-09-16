package httpapi

import (
	"bytes"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/NookMux/NookMux/pkg/jsonx"
	"github.com/pkg/errors"

	"github.com/NookMux/NookMux/internal/common"
	"github.com/NookMux/NookMux/internal/domain/shared"
	"github.com/NookMux/NookMux/internal/infra/cache"
	"github.com/gin-gonic/gin"
)

const KeyRequestBody = "key_request_body"
const KeyBodyStorage = "key_body_storage"

func GetRequestBody(c *gin.Context) ([]byte, error) {
	// 首先检查是否有 cache.BodyStorage 缓存
	if storage, exists := c.Get(KeyBodyStorage); exists && storage != nil {
		if bs, ok := storage.(cache.BodyStorage); ok {
			if _, err := bs.Seek(0, io.SeekStart); err != nil {
				return nil, fmt.Errorf("failed to seek body storage: %w", err)
			}
			return bs.Bytes()
		}
	}

	// 检查旧的缓存方式
	cached, exists := c.Get(KeyRequestBody)
	if exists && cached != nil {
		if b, ok := cached.([]byte); ok {
			return b, nil
		}
	}

	maxMB := shared.MaxRequestBodyMB
	if maxMB <= 0 {
		maxMB = 128 // 默认 128MB
	}
	maxBytes := int64(maxMB) << 20

	contentLength := c.Request.ContentLength

	// 使用新的存储系统
	storage, err := cache.CreateBodyStorageFromReader(c.Request.Body, contentLength, maxBytes)
	_ = c.Request.Body.Close()

	if err != nil {
		if cache.IsRequestBodyTooLargeError(err) {
			return nil, errors.Wrap(cache.ErrRequestBodyTooLarge, fmt.Sprintf("request body exceeds %d MB", maxMB))
		}
		return nil, err
	}

	// 缓存存储对象
	c.Set(KeyBodyStorage, storage)

	// 获取字节数据
	body, err := storage.Bytes()
	if err != nil {
		return nil, err
	}

	// Keep the legacy byte cache only for in-memory storage. Disk-backed bodies
	// should stay on disk between reads instead of pinning another large []byte.
	if !storage.IsDisk() {
		c.Set(KeyRequestBody, body)
	}

	return body, nil
}

// SetRequestBody 用新的字节数据替换请求体缓存，并重置 c.Request.Body 与
// Content-Length，使后续 UnmarshalBodyReusable / GetRequestBody 读取到新内容。
// 用于入站请求体改写（如令牌级模型重定向）。旧的存储会被关闭以释放内存/磁盘记账。
func SetRequestBody(c *gin.Context, body []byte) error {
	storage, err := cache.CreateBodyStorage(body)
	if err != nil {
		return err
	}
	if old, exists := c.Get(KeyBodyStorage); exists && old != nil {
		if bs, ok := old.(cache.BodyStorage); ok {
			bs.Close()
		}
	}
	c.Set(KeyBodyStorage, storage)
	// 与 GetRequestBody 保持一致：仅内存存储保留 legacy 字节缓存，磁盘存储不驻留全量字节。
	if !storage.IsDisk() {
		c.Set(KeyRequestBody, body)
	} else {
		c.Set(KeyRequestBody, nil)
	}
	if storage.IsDisk() {
		if _, err := storage.Seek(0, io.SeekStart); err != nil {
			return err
		}
		c.Request.Body = io.NopCloser(storage)
	} else {
		c.Request.Body = io.NopCloser(bytes.NewReader(body))
	}
	c.Request.ContentLength = int64(len(body))
	return nil
}

// GetBodyStorage 获取请求体存储对象（用于需要多次读取的场景）
func GetBodyStorage(c *gin.Context) (cache.BodyStorage, error) {
	// 检查是否已有存储
	if storage, exists := c.Get(KeyBodyStorage); exists && storage != nil {
		if bs, ok := storage.(cache.BodyStorage); ok {
			if _, err := bs.Seek(0, io.SeekStart); err != nil {
				return nil, fmt.Errorf("failed to seek body storage: %w", err)
			}
			return bs, nil
		}
	}

	// 如果没有，调用 GetRequestBody 创建存储
	_, err := GetRequestBody(c)
	if err != nil {
		return nil, err
	}

	// 再次获取存储
	if storage, exists := c.Get(KeyBodyStorage); exists && storage != nil {
		if bs, ok := storage.(cache.BodyStorage); ok {
			if _, err := bs.Seek(0, io.SeekStart); err != nil {
				return nil, fmt.Errorf("failed to seek body storage: %w", err)
			}
			return bs, nil
		}
	}

	return nil, errors.New("failed to get body storage")
}

// CleanupBodyStorage 清理请求体存储（应在请求结束时调用）
func CleanupBodyStorage(c *gin.Context) {
	if storage, exists := c.Get(KeyBodyStorage); exists && storage != nil {
		if bs, ok := storage.(cache.BodyStorage); ok {
			bs.Close()
		}
		c.Set(KeyBodyStorage, nil)
	}
}

func UnmarshalBodyReusable(c *gin.Context, v any) error {
	storage, err := GetBodyStorage(c)
	if err != nil {
		return err
	}
	contentType := c.Request.Header.Get("Content-Type")

	if storage.IsDisk() && strings.HasPrefix(contentType, "application/json") {
		if _, err := storage.Seek(0, io.SeekStart); err != nil {
			return err
		}
		if err := jsonx.DecodeJson(storage, v); err != nil {
			return err
		}
		if _, err := storage.Seek(0, io.SeekStart); err != nil {
			return err
		}
		c.Request.Body = io.NopCloser(storage)
		return nil
	}

	requestBody, err := storage.Bytes()
	if err != nil {
		return err
	}
	//if DebugEnabled {
	//	println("UnmarshalBodyReusable request body:", string(requestBody))
	//}
	if strings.HasPrefix(contentType, "application/json") {
		err = jsonx.Unmarshal(requestBody, v)
	} else if strings.Contains(contentType, gin.MIMEPOSTForm) {
		err = parseFormData(requestBody, v)
	} else if strings.Contains(contentType, gin.MIMEMultipartPOSTForm) {
		err = parseMultipartFormData(c, requestBody, v)
	}
	// 其余 content-type 暂不解析；err 保持为 nil。
	// TODO: someday non json request have variant model, we will need to implementation this
	if err != nil {
		return err
	}
	// Reset request body
	c.Request.Body = io.NopCloser(bytes.NewBuffer(requestBody))
	return nil
}

func SetContextKey(c *gin.Context, key common.ContextKey, value any) {
	c.Set(string(key), value)
}

func GetContextKey(c *gin.Context, key common.ContextKey) (any, bool) {
	return c.Get(string(key))
}

func GetContextKeyString(c *gin.Context, key common.ContextKey) string {
	return c.GetString(string(key))
}

func GetContextKeyInt(c *gin.Context, key common.ContextKey) int {
	return c.GetInt(string(key))
}

func GetContextKeyBool(c *gin.Context, key common.ContextKey) bool {
	return c.GetBool(string(key))
}

func GetContextKeyStringSlice(c *gin.Context, key common.ContextKey) []string {
	return c.GetStringSlice(string(key))
}

func GetContextKeyStringMap(c *gin.Context, key common.ContextKey) map[string]any {
	return c.GetStringMap(string(key))
}

func GetContextKeyTime(c *gin.Context, key common.ContextKey) time.Time {
	return c.GetTime(string(key))
}

func GetContextKeyType[T any](c *gin.Context, key common.ContextKey) (T, bool) {
	if value, ok := c.Get(string(key)); ok {
		if v, ok := value.(T); ok {
			return v, true
		}
	}
	var t T
	return t, false
}

func ApiError(c *gin.Context, err error) {
	c.JSON(http.StatusOK, gin.H{
		"success": false,
		"message": err.Error(),
	})
}

func ApiErrorMsg(c *gin.Context, msg string) {
	c.JSON(http.StatusOK, gin.H{
		"success": false,
		"message": msg,
	})
}

func ApiSuccess(c *gin.Context, data any) {
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    data,
	})
}

// ApiErrorI18n returns a translated error message based on the user's language preference
// key is the i18n message key, args is optional template data
func ApiErrorI18n(c *gin.Context, key string, args ...map[string]any) {
	msg := TranslateMessage(c, key, args...)
	c.JSON(http.StatusOK, gin.H{
		"success": false,
		"message": msg,
	})
}

// ApiSuccessI18n returns a translated success message based on the user's language preference
func ApiSuccessI18n(c *gin.Context, key string, data any, args ...map[string]any) {
	msg := TranslateMessage(c, key, args...)
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": msg,
		"data":    data,
	})
}

// TranslateMessage is a helper function that calls i18n.T
// This function is defined here to avoid circular imports
// The actual implementation will be set during init
var TranslateMessage func(c *gin.Context, key string, args ...map[string]any) string

func init() {
	// Default implementation that returns the key as-is
	// This will be replaced by i18n.T during i18n initialization
	TranslateMessage = func(c *gin.Context, key string, args ...map[string]any) string {
		return key
	}
}

func ParseMultipartFormReusable(c *gin.Context) (*multipart.Form, error) {
	requestBody, err := GetRequestBody(c)
	if err != nil {
		return nil, err
	}

	contentType := c.Request.Header.Get("Content-Type")
	boundary, err := parseBoundary(contentType)
	if err != nil {
		return nil, err
	}

	reader := multipart.NewReader(bytes.NewReader(requestBody), boundary)
	form, err := reader.ReadForm(multipartMemoryLimit())
	if err != nil {
		return nil, err
	}

	// Reset request body
	c.Request.Body = io.NopCloser(bytes.NewBuffer(requestBody))
	return form, nil
}

func processFormMap(formMap map[string]any, v any) error {
	jsonData, err := jsonx.Marshal(formMap)
	if err != nil {
		return err
	}

	err = jsonx.Unmarshal(jsonData, v)
	if err != nil {
		return err
	}

	return nil
}

func parseFormData(data []byte, v any) error {
	values, err := url.ParseQuery(string(data))
	if err != nil {
		return err
	}
	formMap := make(map[string]any)
	for key, vals := range values {
		if len(vals) == 1 {
			formMap[key] = vals[0]
		} else {
			formMap[key] = vals
		}
	}

	return processFormMap(formMap, v)
}

func parseMultipartFormData(c *gin.Context, data []byte, v any) error {
	contentType := c.Request.Header.Get("Content-Type")
	boundary, err := parseBoundary(contentType)
	if err != nil {
		if errors.Is(err, errBoundaryNotFound) {
			return jsonx.Unmarshal(data, v) // Fallback to JSON
		}
		return err
	}

	reader := multipart.NewReader(bytes.NewReader(data), boundary)
	form, err := reader.ReadForm(multipartMemoryLimit())
	if err != nil {
		return err
	}
	defer func() {
		if err := form.RemoveAll(); err != nil {
			// 清理 multipart 临时文件失败仅记录：表单解析已完成，不影响主流程。
			common.SysError("failed to remove multipart form temp files: " + err.Error())
		}
	}()
	formMap := make(map[string]any)
	for key, vals := range form.Value {
		if len(vals) == 1 {
			formMap[key] = vals[0]
		} else {
			formMap[key] = vals
		}
	}

	return processFormMap(formMap, v)
}

var errBoundaryNotFound = errors.New("multipart boundary not found")

// parseBoundary extracts the multipart boundary from the Content-Type header using mime.ParseMediaType
func parseBoundary(contentType string) (string, error) {
	if contentType == "" {
		return "", errBoundaryNotFound
	}
	// Boundary-UUID / boundary-------xxxxxx
	_, params, err := mime.ParseMediaType(contentType)
	if err != nil {
		return "", err
	}
	boundary, ok := params["boundary"]
	if !ok || boundary == "" {
		return "", errBoundaryNotFound
	}
	return boundary, nil
}

// multipartMemoryLimit returns the configured multipart memory limit in bytes
func multipartMemoryLimit() int64 {
	limitMB := shared.MaxFileDownloadMB
	if limitMB <= 0 {
		limitMB = 32
	}
	return int64(limitMB) << 20
}
