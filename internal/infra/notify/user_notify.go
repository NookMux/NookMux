package notify

import (
	"bytes"
	"fmt"
	"github.com/NookMux/NookMux/internal/common"
	"github.com/NookMux/NookMux/internal/config/system"
	"github.com/NookMux/NookMux/internal/domain/shared"
	"github.com/NookMux/NookMux/internal/infra/email"
	httpclient "github.com/NookMux/NookMux/internal/infra/httpclient"
	media "github.com/NookMux/NookMux/internal/infra/media"
	"github.com/NookMux/NookMux/internal/store/user"
	"github.com/NookMux/NookMux/pkg/jsonx"
	"net/http"
	"net/url"
	"strings"
)

func NotifyRootUser(t string, subject string, content string) {
	user := userstore.GetRootUser().ToBaseUser()
	err := NotifyUser(user.Id, user.Email, user.GetSetting(), shared.NewNotify(t, subject, content, nil))
	if err != nil {
		common.SysLog(fmt.Sprintf("failed to notify root user: %s", err.Error()))
	}
}

func NotifyUser(userId int, userEmail string, userSetting shared.UserSetting, data shared.Notify) error {
	notifyType := userSetting.NotifyType
	if notifyType == "" {
		notifyType = shared.NotifyTypeEmail
	}

	// Check notification limit
	canSend, err := CheckNotificationLimit(userId, data.Type)
	if err != nil {
		common.SysLog(fmt.Sprintf("failed to check notification limit: %s", err.Error()))
		return err
	}
	if !canSend {
		return fmt.Errorf("notification limit exceeded for user %d with type %s", userId, notifyType)
	}

	switch notifyType {
	case shared.NotifyTypeEmail:
		// 优先使用设置中的通知邮箱，如果为空则使用用户的默认邮箱
		emailToUse := userSetting.NotificationEmail
		if emailToUse == "" {
			emailToUse = userEmail
		}
		if emailToUse == "" {
			common.SysLog(fmt.Sprintf("user %d has no email, skip sending email", userId))
			return nil
		}
		return sendEmailNotify(emailToUse, data)
	case shared.NotifyTypeWebhook:
		webhookURLStr := userSetting.WebhookUrl
		if webhookURLStr == "" {
			common.SysLog(fmt.Sprintf("user %d has no webhook url, skip sending webhook", userId))
			return nil
		}

		// 获取 webhook secret
		webhookSecret := userSetting.WebhookSecret
		return SendWebhookNotify(webhookURLStr, webhookSecret, data)
	case shared.NotifyTypeBark:
		barkURL := userSetting.BarkUrl
		if barkURL == "" {
			common.SysLog(fmt.Sprintf("user %d has no bark url, skip sending bark", userId))
			return nil
		}
		return sendBarkNotify(barkURL, data)
	case shared.NotifyTypeGotify:
		gotifyUrl := userSetting.GotifyUrl
		gotifyToken := userSetting.GotifyToken
		if gotifyUrl == "" || gotifyToken == "" {
			common.SysLog(fmt.Sprintf("user %d has no gotify url or token, skip sending gotify", userId))
			return nil
		}
		return sendGotifyNotify(gotifyUrl, gotifyToken, userSetting.GotifyPriority, data)
	}
	return nil
}

func sendEmailNotify(userEmail string, data shared.Notify) error {
	// make email content
	content := data.Content
	// 处理占位符
	for _, value := range data.Values {
		content = strings.Replace(content, shared.ContentValueParam, fmt.Sprintf("%v", value), 1)
	}
	return email.SendEmail(data.Title, userEmail, content)
}

const (
	barkNotifyUserAgent   = "NookMux-Bark-Notify/1.0"
	gotifyNotifyUserAgent = "NookMux-Gotify-Notify/1.0"
)

// notifyHeaders builds HTTP headers for outbound notification requests.
func notifyHeaders(userAgent string, contentType string) map[string]string {
	headers := map[string]string{
		"User-Agent": userAgent,
	}
	if contentType != "" {
		headers["Content-Type"] = contentType
	}
	return headers
}

// sendNotifyHTTPRequest sends a notification request via the configured worker or direct SSRF-validated HTTP client.
func sendNotifyHTTPRequest(method, targetURL, channelLabel string, headers map[string]string, body []byte) error {
	if system.EnableWorker() {
		workerReq := &media.WorkerRequest{
			URL:     targetURL,
			Key:     system.WorkerValidKey,
			Method:  method,
			Headers: headers,
			Body:    body,
		}
		resp, err := media.DoWorkerRequest(workerReq)
		if err != nil {
			return fmt.Errorf("failed to send %s request through worker: %v", channelLabel, err)
		}
		defer resp.Body.Close()

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return fmt.Errorf("%s request failed with status code: %d", channelLabel, resp.StatusCode)
		}
		return nil
	}

	var req *http.Request
	var err error
	if body != nil {
		req, err = httpclient.NewSSRFValidatedRequest(method, targetURL, bytes.NewBuffer(body))
	} else {
		req, err = httpclient.NewSSRFValidatedRequest(method, targetURL, nil)
	}
	if err != nil {
		return fmt.Errorf("request reject: %v", err)
	}

	for key, value := range headers {
		req.Header.Set(key, value)
	}

	resp, err := httpclient.GetHttpClient().Do(req)
	if err != nil {
		return fmt.Errorf("failed to send %s request: %v", channelLabel, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("%s request failed with status code: %d", channelLabel, resp.StatusCode)
	}
	return nil
}

func sendBarkNotify(barkURL string, data shared.Notify) error {
	// 处理占位符
	content := data.Content
	for _, value := range data.Values {
		content = strings.Replace(content, shared.ContentValueParam, fmt.Sprintf("%v", value), 1)
	}

	// 替换模板变量
	finalURL := strings.ReplaceAll(barkURL, "{{title}}", url.QueryEscape(data.Title))
	finalURL = strings.ReplaceAll(finalURL, "{{content}}", url.QueryEscape(content))

	return sendNotifyHTTPRequest(
		http.MethodGet,
		finalURL,
		"bark",
		notifyHeaders(barkNotifyUserAgent, ""),
		nil,
	)
}

func sendGotifyNotify(gotifyUrl string, gotifyToken string, priority int, data shared.Notify) error {
	// 处理占位符
	content := data.Content
	for _, value := range data.Values {
		content = strings.Replace(content, shared.ContentValueParam, fmt.Sprintf("%v", value), 1)
	}

	// 构建完整的 Gotify API URL
	// 确保 URL 以 /message 结尾
	finalURL := strings.TrimSuffix(gotifyUrl, "/") + "/message?token=" + url.QueryEscape(gotifyToken)

	// Gotify优先级范围0-10，如果超出范围则使用默认值5
	if priority < 0 || priority > 10 {
		priority = 5
	}

	// 构建 JSON payload
	type GotifyMessage struct {
		Title    string `json:"title"`
		Message  string `json:"message"`
		Priority int    `json:"priority"`
	}

	payload := GotifyMessage{
		Title:    data.Title,
		Message:  content,
		Priority: priority,
	}

	// 序列化为 JSON
	payloadBytes, err := jsonx.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal gotify payload: %v", err)
	}

	return sendNotifyHTTPRequest(
		http.MethodPost,
		finalURL,
		"gotify",
		notifyHeaders(gotifyNotifyUserAgent, "application/json; charset=utf-8"),
		payloadBytes,
	)
}
