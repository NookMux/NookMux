package common

import (
	"encoding/base64"
	"fmt"
	"net"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/NookMux/NookMux/pkg/jsonx"
	"github.com/samber/lo"
)

var (
	maskURLPattern    = regexp.MustCompile(`(http|https)://[^\s/$.?#].[^\s]*`)
	maskDomainPattern = regexp.MustCompile(`\b(?:[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?\.)+[a-zA-Z]{2,}\b`)
	maskIPPattern     = regexp.MustCompile(`\b(?:\d{1,3}\.){3}\d{1,3}\b`)
	// maskBracketedIPv6Pattern 匹配 Go 网络错误中 IPv6 的方括号形式（如
	// "dial tcp [2001:db8::1]:443"，可带 %zone），内容需再经 net.ParseIP
	// 确认后才遮蔽，避免误伤 items[0] 一类下标写法。
	maskBracketedIPv6Pattern = regexp.MustCompile(`\[([0-9A-Fa-f:.]*:[0-9A-Fa-f:.]*)(%[^\]\s]*)?\]`)
	// maskApiKeyPattern matches patterns like 'api_key:xxx' or "api_key:xxx" to mask the API key value
	maskApiKeyPattern = regexp.MustCompile(`(['"]?)api_key:([^\s'"]+)(['"]?)`)
	// maskMinimaxBrandPattern masks upstream provider brand names (case-insensitive) leaked in error messages
	maskMinimaxBrandPattern = regexp.MustCompile(`(?i)minimax`)
	// maskSpeechPrefixPattern masks upstream internal model prefixes (e.g. speech-02-hd) leaked in error messages
	maskSpeechPrefixPattern = regexp.MustCompile(`speech-`)
)

const LocalLogContentLimit = 2048

func LocalLogPreview(content string) string {
	if DebugEnabled || len(content) <= LocalLogContentLimit {
		return content
	}
	return fmt.Sprintf("%s... [truncated, original_length=%d, limit=%d]", content[:LocalLogContentLimit], len(content), LocalLogContentLimit)
}

func GetStringIfEmpty(str string, defaultValue string) string {
	if str == "" {
		return defaultValue
	}
	return str
}

func GetRandomString(length int) string {
	if length <= 0 {
		return ""
	}
	return lo.RandomString(length, lo.AlphanumericCharset)
}

func MapToJsonStr(m map[string]interface{}) string {
	bytes, err := jsonx.Marshal(m)
	if err != nil {
		return ""
	}
	return string(bytes)
}

func StrToMap(str string) (map[string]interface{}, error) {
	m := make(map[string]interface{})
	err := jsonx.Unmarshal([]byte(str), &m)
	if err != nil {
		return nil, err
	}
	return m, nil
}

func StrToJsonArray(str string) ([]interface{}, error) {
	var js []interface{}
	err := jsonx.Unmarshal([]byte(str), &js)
	if err != nil {
		return nil, err
	}
	return js, nil
}

func IsJsonArray(str string) bool {
	var js []interface{}
	return jsonx.Unmarshal([]byte(str), &js) == nil
}

func IsJsonObject(str string) bool {
	var js map[string]interface{}
	return jsonx.Unmarshal([]byte(str), &js) == nil
}

func String2Int(str string) int {
	num, err := strconv.Atoi(str)
	if err != nil {
		return 0
	}
	return num
}

func StringsContains(strs []string, str string) bool {
	for _, s := range strs {
		if s == str {
			return true
		}
	}
	return false
}

func EncodeBase64(str string) string {
	return base64.StdEncoding.EncodeToString([]byte(str))
}

func GetJsonString(data any) string {
	if data == nil {
		return ""
	}
	b, _ := jsonx.Marshal(data)
	return string(b)
}

// MaskEmail masks a user email to prevent PII leakage in logs
// Returns "***masked***" if email is empty, otherwise shows only the domain part
func MaskEmail(email string) string {
	if email == "" {
		return "***masked***"
	}

	// Find the @ symbol
	atIndex := strings.Index(email, "@")
	if atIndex == -1 {
		// No @ symbol found, return masked
		return "***masked***"
	}

	// Return only the domain part with @ symbol
	return "***@" + email[atIndex+1:]
}

// maskHostTail returns the tail parts of a domain/host that should be preserved.
// It keeps 2 parts for likely country-code TLDs (e.g., co.uk, com.cn), otherwise keeps only the TLD.
func maskHostTail(parts []string) []string {
	if len(parts) < 2 {
		return parts
	}
	lastPart := parts[len(parts)-1]
	secondLastPart := parts[len(parts)-2]
	if len(lastPart) == 2 && len(secondLastPart) <= 3 {
		// Likely country code TLD like co.uk, com.cn
		return []string{secondLastPart, lastPart}
	}
	return []string{lastPart}
}

// maskHostForURL collapses subdomains and keeps only masked prefix + preserved tail.
// Example: api.openai.com -> ***.com, sub.domain.co.uk -> ***.co.uk
func maskHostForURL(host string) string {
	parts := strings.Split(host, ".")
	if len(parts) < 2 {
		return "***"
	}
	tail := maskHostTail(parts)
	return "***." + strings.Join(tail, ".")
}

// maskHostForPlainDomain masks a plain domain and reflects subdomain depth with multiple ***.
// Example: openai.com -> ***.com, api.openai.com -> ***.***.com, sub.domain.co.uk -> ***.***.co.uk
func maskHostForPlainDomain(domain string) string {
	parts := strings.Split(domain, ".")
	if len(parts) < 2 {
		return domain
	}
	tail := maskHostTail(parts)
	numStars := len(parts) - len(tail)
	if numStars < 1 {
		numStars = 1
	}
	stars := strings.TrimSuffix(strings.Repeat("***.", numStars), ".")
	return stars + "." + strings.Join(tail, ".")
}

// MaskSensitiveInfo masks sensitive information like URLs, IPs, and domain names in a string
// Example:
// http://example.com -> http://***.com
// https://api.test.org/v1/users/123?key=secret -> https://***.org/***/***/?key=***
// https://sub.domain.co.uk/path/to/resource -> https://***.co.uk/***/***
// 192.168.1.1 -> ***.***.***.***
// openai.com -> ***.com
// www.openai.com -> ***.***.com
// api.openai.com -> ***.***.com
func MaskSensitiveInfo(str string) string {
	// Mask URLs
	str = maskURLPattern.ReplaceAllStringFunc(str, func(urlStr string) string {
		u, err := url.Parse(urlStr)
		if err != nil {
			return urlStr
		}

		host := u.Host
		if host == "" {
			return urlStr
		}

		// Mask host with unified logic
		maskedHost := maskHostForURL(host)
		// IP 字面量主机整体遮蔽并保留端口：网络错误中的 IP 是解析出的
		// 上游或网关自身地址，与主机是否自定义无关，必须无条件遮蔽。
		if hostname := u.Hostname(); hostname != "" {
			if ip := net.ParseIP(hostname); ip != nil {
				if ip.To4() != nil {
					maskedHost = "***.***.***.***"
				} else {
					maskedHost = "***"
				}
				if port := u.Port(); port != "" {
					maskedHost += ":" + port
				}
			}
		}

		result := u.Scheme + "://" + maskedHost

		// Mask path
		if u.Path != "" && u.Path != "/" {
			pathParts := strings.Split(strings.Trim(u.Path, "/"), "/")
			maskedPathParts := make([]string, len(pathParts))
			for i := range pathParts {
				if pathParts[i] != "" {
					maskedPathParts[i] = "***"
				}
			}
			if len(maskedPathParts) > 0 {
				result += "/" + strings.Join(maskedPathParts, "/")
			}
		} else if u.Path == "/" {
			result += "/"
		}

		// Mask query parameters
		if u.RawQuery != "" {
			values, err := url.ParseQuery(u.RawQuery)
			if err != nil {
				// If can't parse query, just mask the whole query string
				result += "?***"
			} else {
				maskedParams := make([]string, 0, len(values))
				for key := range values {
					maskedParams = append(maskedParams, key+"=***")
				}
				if len(maskedParams) > 0 {
					result += "?" + strings.Join(maskedParams, "&")
				}
			}
		}

		return result
	})

	// Mask bracketed IPv6 addresses (Go network errors write them as "[addr]:port")
	str = maskBracketedIPv6Pattern.ReplaceAllStringFunc(str, func(match string) string {
		if net.ParseIP(maskBracketedIPv6Pattern.FindStringSubmatch(match)[1]) == nil {
			return match
		}
		return "[***]"
	})

	// Mask domain names without protocol (like openai.com, www.openai.com)
	str = maskDomainPattern.ReplaceAllStringFunc(str, func(domain string) string {
		return maskHostForPlainDomain(domain)
	})

	// Mask IP addresses
	str = maskIPPattern.ReplaceAllString(str, "***.***.***.***")

	// Mask API keys (e.g., "api_key:EXAMPLE-KEY-REPLACE-ME" -> "api_key:***")
	str = maskApiKeyPattern.ReplaceAllString(str, "${1}api_key:***${3}")

	// Mask upstream provider/brand identifiers leaked in error messages
	str = maskMinimaxBrandPattern.ReplaceAllString(str, "*")
	str = maskSpeechPrefixPattern.ReplaceAllString(str, "*")

	return str
}

// MaskSensitiveInfoWithExemptions works like MaskSensitiveInfo but preserves the
// given exemption strings (e.g. the current model name and group name) so that
// legitimate values are not masked away. Exemptions are temporarily replaced
// with null-byte-delimited placeholders that cannot appear in normal text and
// are not touched by any mask rule.
func MaskSensitiveInfoWithExemptions(str string, exemptions []string) string {
	seen := make(map[string]bool)
	valid := make([]string, 0, len(exemptions))
	for _, s := range exemptions {
		if s == "" || seen[s] {
			continue
		}
		seen[s] = true
		valid = append(valid, s)
	}
	if len(valid) == 0 {
		return MaskSensitiveInfo(str)
	}
	// Sort by length descending so longer exemptions are protected first.
	sort.Slice(valid, func(i, j int) bool {
		return len(valid[i]) > len(valid[j])
	})
	masked := str
	placeholders := make([]string, len(valid))
	for i, s := range valid {
		placeholders[i] = fmt.Sprintf("\x00E%d\x00", i)
		masked = strings.ReplaceAll(masked, s, placeholders[i])
	}
	masked = MaskSensitiveInfo(masked)
	for i := len(valid) - 1; i >= 0; i-- {
		masked = strings.ReplaceAll(masked, placeholders[i], valid[i])
	}
	return masked
}
