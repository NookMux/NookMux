package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestMaskLogPath(t *testing.T) {
	cases := []struct {
		name string
		path string
		want string
	}{
		{
			name: "无查询串的路径原样输出",
			path: "/v1/chat/completions",
			want: "/v1/chat/completions",
		},
		{
			name: "API 密钥查询参数的值被遮蔽、键名保留",
			path: "/v1beta/models?key=sk-secret123&alt=sse",
			want: "/v1beta/models?alt=***&key=***",
		},
		{
			name: "OAuth 回调的查询串整段截断",
			path: "/api/oauth/github?code=o_auth_code&state=s_tate",
			want: "/api/oauth/github",
		},
		{
			name: "无 oauth 前缀的路径不走整段截断",
			path: "/api/oauthx/callback?code=leak",
			want: "/api/oauthx/callback?code=***",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, tc.path, nil)
			param := gin.LogFormatterParams{
				Path:    tc.path,
				Request: request,
			}
			got := maskLogPath(param)
			if got != tc.want {
				t.Fatalf("maskLogPath(%q) = %q, want %q", tc.path, got, tc.want)
			}
		})
	}
}

func TestMaskLogPathKeepsCredentialValuesOut(t *testing.T) {
	const path = "/v1beta/models?key=sk-super-secret"
	request := httptest.NewRequest(http.MethodGet, path, nil)
	got := maskLogPath(gin.LogFormatterParams{Path: path, Request: request})
	if strings.Contains(got, "sk-super-secret") {
		t.Fatalf("masked path %q still contains credential value", got)
	}
	if !strings.Contains(got, "key=***") {
		t.Fatalf("masked path %q lost the parameter key", got)
	}
}
