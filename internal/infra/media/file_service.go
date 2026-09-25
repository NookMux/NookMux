package media

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"strings"

	"github.com/NookMux/NookMux/internal/common"
	"github.com/NookMux/NookMux/internal/domain/shared"
	"github.com/NookMux/NookMux/internal/infra/cache"
	"github.com/NookMux/NookMux/internal/infra/log"
	"github.com/NookMux/NookMux/internal/infra/security"
	"github.com/gin-gonic/gin"
	"golang.org/x/image/webp"
)

// FileService 统一的文件处理服务
// 提供文件下载、解码、缓存等功能的统一入口

// getContextCacheKey 生成 context 缓存的 key
func getContextCacheKey(url string) string {
	return fmt.Sprintf("file_cache_%s", security.GenerateHMAC(url))
}

// LoadFileSource 加载文件源数据
// 这是统一的入口，会自动处理缓存和不同的来源类型
func LoadFileSource(c *gin.Context, source *shared.FileSource, reason ...string) (*shared.CachedFileData, error) {
	if source == nil {
		return nil, fmt.Errorf("file source is nil")
	}

	if common.DebugEnabled {
		log.LogDebug(c, fmt.Sprintf("LoadFileSource starting for: %s", source.GetIdentifier()))
	}

	// 1. 快速检查内部缓存
	if source.HasCache() {
		// 即使命中内部缓存，也要确保注册到清理列表（如果尚未注册）
		if c != nil {
			registerSourceForCleanup(c, source)
		}
		return source.GetCache(), nil
	}

	// 2. 加锁保护加载过程
	source.Mu().Lock()
	defer source.Mu().Unlock()

	// 3. 双重检查
	if source.HasCache() {
		if c != nil {
			registerSourceForCleanup(c, source)
		}
		return source.GetCache(), nil
	}

	// 4. 如果是 URL，检查 Context 缓存
	var contextKey string
	if source.IsURL() && c != nil {
		contextKey = getContextCacheKey(source.URL)
		if cachedData, exists := c.Get(contextKey); exists {
			data := cachedData.(*shared.CachedFileData)
			source.SetCache(data)
			registerSourceForCleanup(c, source)
			return data, nil
		}
	}

	// 5. 执行加载逻辑
	var cachedData *shared.CachedFileData
	var err error

	if source.IsURL() {
		cachedData, err = loadFromURL(c, source.URL, reason...)
	} else {
		cachedData, err = loadFromBase64(source.Base64Data, source.MimeType)
	}

	if err != nil {
		return nil, err
	}

	// 6. 设置缓存
	source.SetCache(cachedData)
	if contextKey != "" && c != nil {
		c.Set(contextKey, cachedData)
	}

	// 7. 注册到 context 以便请求结束时自动清理
	if c != nil {
		registerSourceForCleanup(c, source)
	}

	return cachedData, nil
}

// registerSourceForCleanup 注册 FileSource 到 context 以便请求结束时清理
func registerSourceForCleanup(c *gin.Context, source *shared.FileSource) {
	if source.IsRegistered() {
		return
	}

	key := string(common.ContextKeyFileSourcesToCleanup)
	var sources []*shared.FileSource
	if existing, exists := c.Get(key); exists {
		sources = existing.([]*shared.FileSource)
	}
	sources = append(sources, source)
	c.Set(key, sources)
	source.SetRegistered(true)
}

// CleanupFileSources 清理请求中所有注册的 FileSource
// 应在请求结束时调用（通常由中间件自动调用）
func CleanupFileSources(c *gin.Context) {
	key := string(common.ContextKeyFileSourcesToCleanup)
	if sources, exists := c.Get(key); exists {
		for _, source := range sources.([]*shared.FileSource) {
			if cache := source.GetCache(); cache != nil {
				cache.Close()
			}
		}
		c.Set(key, nil) // 清除引用
	}
}

// loadFromURL 从 URL 加载文件
func loadFromURL(c *gin.Context, url string, reason ...string) (*shared.CachedFileData, error) {
	// 下载文件
	var maxFileSize = shared.MaxFileDownloadMB * 1024 * 1024

	if common.DebugEnabled {
		log.LogDebug(c, "loadFromURL: initiating download")
	}
	resp, err := DoDownloadRequest(url, reason...)
	if err != nil {
		return nil, fmt.Errorf("failed to download file from %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("failed to download file, status code: %d", resp.StatusCode)
	}

	// 读取文件内容（限制大小）
	if common.DebugEnabled {
		log.LogDebug(c, "loadFromURL: reading response body")
	}
	fileBytes, err := io.ReadAll(io.LimitReader(resp.Body, int64(maxFileSize+1)))
	if err != nil {
		return nil, fmt.Errorf("failed to read file content: %w", err)
	}
	if len(fileBytes) > maxFileSize {
		return nil, fmt.Errorf("file size exceeds maximum allowed size: %dMB", shared.MaxFileDownloadMB)
	}

	// 转换为 base64
	base64Data := base64.StdEncoding.EncodeToString(fileBytes)

	// 智能获取 MIME 类型
	mimeType := smartDetectMimeType(resp, url, fileBytes)

	// 判断是否使用磁盘缓存
	base64Size := int64(len(base64Data))
	var cachedData *shared.CachedFileData

	if shouldUseDiskCache(base64Size) {
		// 使用磁盘缓存
		diskPath, err := writeToDiskCache(base64Data)
		if err != nil {
			// 磁盘缓存失败，回退到内存
			log.LogWarn(c, fmt.Sprintf("Failed to write to disk cache, falling back to memory: %v", err))
			cachedData = shared.NewMemoryCachedData(base64Data, mimeType, int64(len(fileBytes)))
		} else {
			cachedData = shared.NewDiskCachedData(diskPath, mimeType, int64(len(fileBytes)))
			cachedData.DiskSize = base64Size
			cachedData.OnClose = func(size int64) {
				cache.DecrementDiskFiles(size)
			}
			cache.IncrementDiskFiles(base64Size)
			if common.DebugEnabled {
				log.LogDebug(c, fmt.Sprintf("File cached to disk: %s, size: %d bytes", diskPath, base64Size))
			}
		}
	} else {
		// 使用内存缓存
		cachedData = shared.NewMemoryCachedData(base64Data, mimeType, int64(len(fileBytes)))
	}

	// 如果是图片，尝试获取图片配置
	if strings.HasPrefix(mimeType, "image/") {
		if common.DebugEnabled {
			log.LogDebug(c, "loadFromURL: decoding image config")
		}
		config, format, err := decodeImageConfig(fileBytes)
		if err == nil {
			cachedData.ImageConfig = &config
			cachedData.ImageFormat = format
			// 如果通过图片解码获取了更准确的格式，更新 MIME 类型
			if mimeType == "application/octet-stream" || mimeType == "" {
				cachedData.MimeType = "image/" + format
			}
		}
	}

	return cachedData, nil
}

// shouldUseDiskCache 判断是否应该使用磁盘缓存
func shouldUseDiskCache(dataSize int64) bool {
	return cache.ShouldUseDiskCache(dataSize)
}

// writeToDiskCache 将数据写入磁盘缓存
func writeToDiskCache(base64Data string) (string, error) {
	return cache.WriteDiskCacheFileString(cache.DiskCacheTypeFile, base64Data)
}

// smartDetectMimeType 智能检测 MIME 类型
func smartDetectMimeType(resp *http.Response, url string, fileBytes []byte) string {
	// 1. 尝试从 Content-Type header 获取
	mimeType := resp.Header.Get("Content-Type")
	if idx := strings.Index(mimeType, ";"); idx != -1 {
		mimeType = strings.TrimSpace(mimeType[:idx])
	}
	if mimeType != "" && mimeType != "application/octet-stream" {
		return mimeType
	}

	// 2. 尝试从 Content-Disposition header 的 filename 获取
	if cd := resp.Header.Get("Content-Disposition"); cd != "" {
		parts := strings.Split(cd, ";")
		for _, part := range parts {
			part = strings.TrimSpace(part)
			if strings.HasPrefix(strings.ToLower(part), "filename=") {
				name := strings.TrimSpace(strings.TrimPrefix(part, "filename="))
				// 移除引号
				if len(name) > 2 && name[0] == '"' && name[len(name)-1] == '"' {
					name = name[1 : len(name)-1]
				}
				if dot := strings.LastIndex(name, "."); dot != -1 && dot+1 < len(name) {
					ext := strings.ToLower(name[dot+1:])
					if ext != "" {
						mt := GetMimeTypeByExtension(ext)
						if mt != "application/octet-stream" {
							return mt
						}
					}
				}
				break
			}
		}
	}

	// 3. 尝试从 URL 路径获取扩展名
	mt := guessMimeTypeFromURL(url)
	if mt != "application/octet-stream" {
		return mt
	}

	// 4. 使用 http.DetectContentType 内容嗅探
	if len(fileBytes) > 0 {
		sniffed := http.DetectContentType(fileBytes)
		if sniffed != "" && sniffed != "application/octet-stream" {
			// 去除可能的 charset 参数
			if idx := strings.Index(sniffed, ";"); idx != -1 {
				sniffed = strings.TrimSpace(sniffed[:idx])
			}
			return sniffed
		}

		if heifMime := detectHEIF(fileBytes); heifMime != "" {
			return heifMime
		}
	}

	// 5. 尝试作为图片解码获取格式
	if len(fileBytes) > 0 {
		if _, format, err := decodeImageConfig(fileBytes); err == nil && format != "" {
			return "image/" + strings.ToLower(format)
		}
	}

	// 最终回退
	return "application/octet-stream"
}

// loadFromBase64 从 base64 字符串加载文件
func loadFromBase64(base64String string, providedMimeType string) (*shared.CachedFileData, error) {
	var mimeType string
	var cleanBase64 string

	// 处理 data: 前缀
	if strings.HasPrefix(base64String, "data:") {
		idx := strings.Index(base64String, ",")
		if idx != -1 {
			header := base64String[:idx]
			cleanBase64 = base64String[idx+1:]

			if strings.Contains(header, ":") && strings.Contains(header, ";") {
				mimeStart := strings.Index(header, ":") + 1
				mimeEnd := strings.Index(header, ";")
				if mimeStart < mimeEnd {
					mimeType = header[mimeStart:mimeEnd]
				}
			}
		} else {
			cleanBase64 = base64String
		}
	} else {
		cleanBase64 = base64String
	}

	if providedMimeType != "" {
		mimeType = providedMimeType
	}

	decodedData, err := base64.StdEncoding.DecodeString(cleanBase64)
	if err != nil {
		return nil, fmt.Errorf("failed to decode base64 data: %w", err)
	}

	base64Size := int64(len(cleanBase64))
	var cachedData *shared.CachedFileData

	if shouldUseDiskCache(base64Size) {
		diskPath, err := writeToDiskCache(cleanBase64)
		if err != nil {
			cachedData = shared.NewMemoryCachedData(cleanBase64, mimeType, int64(len(decodedData)))
		} else {
			cachedData = shared.NewDiskCachedData(diskPath, mimeType, int64(len(decodedData)))
			cachedData.DiskSize = base64Size
			cachedData.OnClose = func(size int64) {
				cache.DecrementDiskFiles(size)
			}
			cache.IncrementDiskFiles(base64Size)
		}
	} else {
		cachedData = shared.NewMemoryCachedData(cleanBase64, mimeType, int64(len(decodedData)))
	}

	if mimeType == "" || strings.HasPrefix(mimeType, "image/") {
		config, format, err := decodeImageConfig(decodedData)
		if err == nil {
			cachedData.ImageConfig = &config
			cachedData.ImageFormat = format
			if mimeType == "" {
				cachedData.MimeType = "image/" + format
			}
		}
	}

	return cachedData, nil
}

// GetImageConfig 获取图片配置
func GetImageConfig(c *gin.Context, source *shared.FileSource) (image.Config, string, error) {
	cachedData, err := LoadFileSource(c, source, "get_image_config")
	if err != nil {
		return image.Config{}, "", err
	}

	if cachedData.ImageConfig != nil {
		return *cachedData.ImageConfig, cachedData.ImageFormat, nil
	}

	base64Str, err := cachedData.GetBase64Data()
	if err != nil {
		return image.Config{}, "", fmt.Errorf("failed to get base64 data: %w", err)
	}
	decodedData, err := base64.StdEncoding.DecodeString(base64Str)
	if err != nil {
		return image.Config{}, "", fmt.Errorf("failed to decode base64 for image config: %w", err)
	}

	config, format, err := decodeImageConfig(decodedData)
	if err != nil {
		return image.Config{}, "", err
	}

	cachedData.ImageConfig = &config
	cachedData.ImageFormat = format

	return config, format, nil
}

// GetBase64Data 获取 base64 编码的数据
func GetBase64Data(c *gin.Context, source *shared.FileSource, reason ...string) (string, string, error) {
	cachedData, err := LoadFileSource(c, source, reason...)
	if err != nil {
		return "", "", err
	}
	base64Str, err := cachedData.GetBase64Data()
	if err != nil {
		return "", "", fmt.Errorf("failed to get base64 data: %w", err)
	}
	return base64Str, cachedData.MimeType, nil
}

// GetMimeType 获取文件的 MIME 类型
func GetMimeType(c *gin.Context, source *shared.FileSource) (string, error) {
	if source.HasCache() {
		return source.GetCache().MimeType, nil
	}

	if source.IsURL() {
		mimeType, err := GetFileTypeFromUrl(c, source.URL, "get_mime_type")
		if err == nil && mimeType != "" && mimeType != "application/octet-stream" {
			return mimeType, nil
		}
	}

	cachedData, err := LoadFileSource(c, source, "get_mime_type")
	if err != nil {
		return "", err
	}
	return cachedData.MimeType, nil
}

// DetectFileType 检测文件类型
func DetectFileType(mimeType string) shared.FileType {
	if strings.HasPrefix(mimeType, "image/") {
		return shared.FileTypeImage
	}
	if strings.HasPrefix(mimeType, "audio/") {
		return shared.FileTypeAudio
	}
	if strings.HasPrefix(mimeType, "video/") {
		return shared.FileTypeVideo
	}
	return shared.FileTypeFile
}

// decodeImageConfig 从字节数据解码图片配置
func decodeImageConfig(data []byte) (image.Config, string, error) {
	reader := bytes.NewReader(data)

	config, format, err := image.DecodeConfig(reader)
	if err == nil {
		return config, format, nil
	}

	if _, err := reader.Seek(0, io.SeekStart); err != nil {
		return image.Config{}, "", fmt.Errorf("failed to seek image reader: %w", err)
	}
	config, err = webp.DecodeConfig(reader)
	if err == nil {
		return config, "webp", nil
	}

	if heifMime := detectHEIF(data); heifMime != "" {
		formatName := "heif"
		if heifMime == "image/heic" {
			formatName = "heic"
		}
		if w, h, ok := parseHEIFDimensions(data); ok {
			return image.Config{Width: w, Height: h}, formatName, nil
		}
		return image.Config{}, "", fmt.Errorf("failed to decode HEIF/HEIC image dimensions")
	}

	return image.Config{}, "", fmt.Errorf("failed to decode image config: unsupported format")
}

// detectHEIF checks ISOBMFF magic bytes to detect HEIC/HEIF files.
func detectHEIF(data []byte) string {
	if len(data) < 12 {
		return ""
	}
	if string(data[4:8]) != "ftyp" {
		return ""
	}
	switch string(data[8:12]) {
	case "heic", "heix", "hevc", "hevx", "heim", "heis":
		return "image/heic"
	case "mif1", "msf1":
		return "image/heif"
	default:
		return ""
	}
}

// parseHEIFDimensions walks the ISOBMFF box tree and reads width/height from ispe.
func parseHEIFDimensions(data []byte) (int, int, bool) {
	size := len(data)
	if size < 12 {
		return 0, 0, false
	}

	offset := 0
	for offset+8 <= size {
		boxSize := int(binary.BigEndian.Uint32(data[offset : offset+4]))
		boxType := string(data[offset+4 : offset+8])
		headerLen := 8

		if boxSize == 1 {
			if offset+16 > size {
				break
			}
			boxSize = int(binary.BigEndian.Uint64(data[offset+8 : offset+16]))
			headerLen = 16
		} else if boxSize == 0 {
			boxSize = size - offset
		}

		if boxSize < headerLen || offset+boxSize > size {
			break
		}
		if boxType == "meta" {
			metaData := data[offset+headerLen : offset+boxSize]
			if len(metaData) < 4 {
				return 0, 0, false
			}
			return findISPE(metaData[4:])
		}
		offset += boxSize
	}
	return 0, 0, false
}

// maxImageDimension 采信的图片宽高上限。图片头部声明的尺寸来自不受信来源，
// 超大声明值会使下游基于面积的计费计算溢出，超限即视为非法尺寸、解析未命中。
const maxImageDimension = 30000

func findISPE(data []byte) (int, int, bool) {
	offset := 0
	size := len(data)
	for offset+8 <= size {
		boxSize := int(binary.BigEndian.Uint32(data[offset : offset+4]))
		boxType := string(data[offset+4 : offset+8])
		if boxSize < 8 || offset+boxSize > size {
			break
		}
		content := data[offset+8 : offset+boxSize]
		switch boxType {
		case "iprp", "ipco":
			if w, h, ok := findISPE(content); ok {
				return w, h, true
			}
		case "ispe":
			if len(content) >= 12 {
				w := int(binary.BigEndian.Uint32(content[4:8]))
				h := int(binary.BigEndian.Uint32(content[8:12]))
				if w > maxImageDimension || h > maxImageDimension {
					return 0, 0, false
				}
				if w > 0 && h > 0 {
					return w, h, true
				}
			}
		}
		offset += boxSize
	}
	return 0, 0, false
}

// guessMimeTypeFromURL 从 URL 猜测 MIME 类型
func guessMimeTypeFromURL(url string) string {
	cleanedURL := url
	if q := strings.Index(cleanedURL, "?"); q != -1 {
		cleanedURL = cleanedURL[:q]
	}

	if slash := strings.LastIndex(cleanedURL, "/"); slash != -1 && slash+1 < len(cleanedURL) {
		last := cleanedURL[slash+1:]
		if dot := strings.LastIndex(last, "."); dot != -1 && dot+1 < len(last) {
			ext := strings.ToLower(last[dot+1:])
			return GetMimeTypeByExtension(ext)
		}
	}

	return "application/octet-stream"
}
