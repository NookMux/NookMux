package vertex

import (
	"fmt"
	"strings"
)

const (
	DefaultAPIVersion    = "v1"
	OpenSourceAPIVersion = "v1beta1"
	PublisherGoogle      = "google"
	PublisherAnthropic   = "anthropic"
)

// normalizeVertexBaseURL trims surrounding whitespace and trailing slashes so a
// configured gateway prefix like "https://gateway.example/vertex/" does not end
// up with a doubled slash when path pieces are appended.
func normalizeVertexBaseURL(baseURL string) string {
	return strings.TrimRight(strings.TrimSpace(baseURL), "/")
}

func normalizeVertexRegion(region string) string {
	region = strings.TrimSpace(region)
	if region == "" {
		return "global"
	}
	return region
}

// appendVertexAPIVersion appends the Vertex API version segment unless the base
// URL already ends with it, so a base_url that already carries the version is
// not duplicated.
func appendVertexAPIVersion(baseURL, version string) string {
	version = strings.Trim(strings.TrimSpace(version), "/")
	if version == "" {
		return baseURL
	}
	if strings.HasSuffix(baseURL, "/"+version) {
		return baseURL
	}
	return baseURL + "/" + version
}

// BuildAPIBaseURL builds the projects/locations prefix for a Vertex request.
// When baseURL is provided it is treated as a gateway prefix (the entire host
// portion is replaced by it); otherwise the default Google aiplatform hosts are
// used so existing channels keep working unchanged.
func BuildAPIBaseURL(baseURL, version, projectID, region string) string {
	if normalized := normalizeVertexBaseURL(baseURL); normalized != "" {
		normalized = appendVertexAPIVersion(normalized, version)

		region = normalizeVertexRegion(region)
		if strings.TrimSpace(projectID) != "" {
			normalized = fmt.Sprintf("%s/projects/%s/locations/%s", normalized, projectID, region)
		}
		return normalized
	}

	region = normalizeVertexRegion(region)
	host := defaultVertexAPIHost(region, version)
	if strings.TrimSpace(projectID) == "" {
		return host
	}
	return fmt.Sprintf("%s/projects/%s/locations/%s", host, projectID, region)
}

// defaultVertexAPIHost 返回官方默认主机：global 与 us/eu 多区域端点使用各自
// 不带区域前缀的专属域名，其余区域使用 {region}-aiplatform.googleapis.com。
func defaultVertexAPIHost(region, version string) string {
	if region == "global" {
		return fmt.Sprintf("https://aiplatform.googleapis.com/%s", version)
	}
	if region == "us" || region == "eu" {
		return fmt.Sprintf("https://aiplatform.%s.rep.googleapis.com/%s", region, version)
	}
	return fmt.Sprintf("https://%s-aiplatform.googleapis.com/%s", region, version)
}

func BuildPublisherModelURL(baseURL, version, projectID, region, publisher, modelName, action string) string {
	return fmt.Sprintf(
		"%s/publishers/%s/models/%s:%s",
		BuildAPIBaseURL(baseURL, version, projectID, region),
		publisher,
		modelName,
		action,
	)
}

func BuildGoogleModelURL(baseURL, version, projectID, region, modelName, action string) string {
	return BuildPublisherModelURL(baseURL, version, projectID, region, PublisherGoogle, modelName, action)
}

func BuildAnthropicModelURL(baseURL, version, projectID, region, modelName, action string) string {
	return BuildPublisherModelURL(baseURL, version, projectID, region, PublisherAnthropic, modelName, action)
}

func BuildOpenSourceChatCompletionsURL(baseURL, projectID, region string) string {
	return fmt.Sprintf(
		"%s/endpoints/openapi/chat/completions",
		BuildAPIBaseURL(baseURL, OpenSourceAPIVersion, projectID, region),
	)
}
