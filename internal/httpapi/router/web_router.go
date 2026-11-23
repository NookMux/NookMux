package router

import (
	"errors"
	"io"
	"io/fs"
	"mime"
	"net/http"
	"path"
	"strconv"
	"strings"

	"github.com/NookMux/NookMux/internal/app/webdist"
	"github.com/NookMux/NookMux/internal/httpapi/controller/relay"
	"github.com/NookMux/NookMux/internal/httpapi/middleware"
	"github.com/gin-contrib/gzip"
	"github.com/gin-contrib/static"
	"github.com/gin-gonic/gin"
)

type WebAssets struct {
	BuildFS   fs.FS
	IndexPage []byte
}

func SetWebRouter(router *gin.Engine, assets WebAssets) {
	webFS := webdist.EmbedFolder(assets.BuildFS, "dist")

	// Web 限流、缓存头与静态资源服务只挂在 SPA 回退链上，作用于页面、构建产物与
	// 未匹配路径；先于本函数注册的 API 与 Relay 路由不经过这条链。
	router.NoRoute(
		middleware.GlobalWebRateLimit(webFS),
		middleware.Cache(),
		staticAssetCache(),
		precompressedStaticAssets(assets.BuildFS),
		gzip.Gzip(gzip.DefaultCompression),
		static.Serve("/", webFS),
		func(c *gin.Context) {
			if strings.HasPrefix(c.Request.RequestURI, "/v1") || strings.HasPrefix(c.Request.RequestURI, "/api") || strings.HasPrefix(c.Request.RequestURI, "/assets") {
				relaycontroller.RelayNotFound(c)
				return
			}
			c.Header("Cache-Control", "no-cache")
			c.Data(http.StatusOK, "text/html; charset=utf-8", assets.IndexPage)
		},
	)
}

func staticAssetCache() gin.HandlerFunc {
	return func(c *gin.Context) {
		if strings.HasPrefix(c.Request.URL.Path, "/static/") || c.Request.URL.Path == "/favicon.ico" {
			c.Header("Cache-Control", "public, max-age=31536000, immutable")
		}
		c.Next()
	}
}

func precompressedStaticAssets(buildFS fs.FS) gin.HandlerFunc {
	return precompressedStaticAssetsFS(buildFS, "dist")
}

func precompressedStaticAssetsFS(buildFS fs.FS, root string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Method != http.MethodGet && c.Request.Method != http.MethodHead {
			return
		}
		if !strings.HasPrefix(c.Request.URL.Path, "/static/") {
			return
		}

		if !acceptsBrotli(c.GetHeader("Accept-Encoding")) {
			return
		}

		assetPath := path.Clean(strings.TrimPrefix(c.Request.URL.Path, "/"))
		if assetPath == "." || strings.HasPrefix(assetPath, "../") || strings.Contains(assetPath, "/../") || !strings.HasPrefix(assetPath, "static/") {
			return
		}

		compressedPath := path.Join(root, assetPath+".br")
		file, err := buildFS.Open(compressedPath)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				return
			}
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}
		defer file.Close()

		stat, err := file.Stat()
		if err != nil {
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}
		seeker, ok := file.(io.ReadSeeker)
		if !ok {
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}

		contentType := mime.TypeByExtension(path.Ext(assetPath))
		if contentType != "" {
			c.Header("Content-Type", contentType)
		}
		c.Header("Content-Encoding", "br")
		c.Header("Vary", "Accept-Encoding")
		http.ServeContent(c.Writer, c.Request, path.Base(assetPath), stat.ModTime(), seeker)
		c.Abort()
	}
}

func acceptsBrotli(acceptEncoding string) bool {
	for _, value := range strings.Split(acceptEncoding, ",") {
		token, allowed := parseAcceptEncodingToken(value)
		if !allowed {
			continue
		}
		if token == "br" {
			return true
		}
	}
	return false
}

func parseAcceptEncodingToken(value string) (string, bool) {
	parts := strings.Split(value, ";")
	token := strings.ToLower(strings.TrimSpace(parts[0]))
	for _, part := range parts[1:] {
		param := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(part), " ", ""))
		if strings.HasPrefix(param, "q=") {
			q, err := strconv.ParseFloat(strings.TrimPrefix(param, "q="), 64)
			if err == nil && q <= 0 {
				return token, false
			}
		}
	}
	return token, token != ""
}
