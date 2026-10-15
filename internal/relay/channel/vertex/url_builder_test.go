package vertex

import (
	"strings"
	"testing"
)

// TestBuildAPIBaseURL_DefaultGlobalProject mirrors the pre-existing default host
// so existing Vertex channels keep the same upstream URL when no base_url is set.
func TestBuildAPIBaseURL_DefaultGlobalProject(t *testing.T) {
	got := BuildAPIBaseURL("", DefaultAPIVersion, "my-project", "global")
	want := "https://aiplatform.googleapis.com/v1/projects/my-project/locations/global"
	if got != want {
		t.Fatalf("BuildAPIBaseURL global = %q, want %q", got, want)
	}
}

func TestBuildAPIBaseURL_DefaultRegionalProject(t *testing.T) {
	got := BuildAPIBaseURL("", DefaultAPIVersion, "my-project", "us-central1")
	want := "https://us-central1-aiplatform.googleapis.com/v1/projects/my-project/locations/us-central1"
	if got != want {
		t.Fatalf("BuildAPIBaseURL regional = %q, want %q", got, want)
	}
}

// TestBuildAPIBaseURL_MultiRegionEndpoints guards the us/eu multi-region hosts
// (aiplatform.{us,eu}.rep.googleapis.com) used by newer Claude / Gemini models.
func TestBuildAPIBaseURL_MultiRegionEndpoints(t *testing.T) {
	got := BuildAPIBaseURL("", DefaultAPIVersion, "p", "us")
	want := "https://aiplatform.us.rep.googleapis.com/v1/projects/p/locations/us"
	if got != want {
		t.Fatalf("BuildAPIBaseURL us multi-region = %q, want %q", got, want)
	}

	got = BuildAPIBaseURL("", DefaultAPIVersion, "p", "eu")
	want = "https://aiplatform.eu.rep.googleapis.com/v1/projects/p/locations/eu"
	if got != want {
		t.Fatalf("BuildAPIBaseURL eu multi-region = %q, want %q", got, want)
	}

	got = BuildGoogleModelURL("", DefaultAPIVersion, "p", "eu", "claude-sonnet-5-5", "rawPredict")
	want = "https://aiplatform.eu.rep.googleapis.com/v1/projects/p/locations/eu/publishers/google/models/claude-sonnet-5-5:rawPredict"
	if got != want {
		t.Fatalf("BuildGoogleModelURL eu multi-region = %q, want %q", got, want)
	}
}

// TestBuildAPIBaseURL_CustomBaseAsGatewayPrefix is the core regression for #148:
// a configured base_url must become the gateway prefix instead of being ignored.
func TestBuildAPIBaseURL_CustomBaseAsGatewayPrefix(t *testing.T) {
	got := BuildAPIBaseURL("https://gateway.example/vertex", DefaultAPIVersion, "my-project", "us-central1")
	want := "https://gateway.example/vertex/v1/projects/my-project/locations/us-central1"
	if got != want {
		t.Fatalf("BuildAPIBaseURL custom = %q, want %q", got, want)
	}
}

// TestBuildAPIBaseURL_CustomBaseTrailingSlashAndVersionDedup ensures a sloppy
// base_url (trailing slash or an already-included version) does not corrupt the URL.
func TestBuildAPIBaseURL_CustomBaseTrailingSlashAndVersionDedup(t *testing.T) {
	withSlash := BuildAPIBaseURL("https://gateway.example/vertex/", DefaultAPIVersion, "p", "global")
	if strings.Contains(withSlash, "vertex//v1") {
		t.Fatalf("trailing slash produced doubled path separators: %s", withSlash)
	}

	withVersion := BuildAPIBaseURL("https://gateway.example/vertex/v1", DefaultAPIVersion, "p", "global")
	if strings.Contains(withVersion, "v1/v1") {
		t.Fatalf("already-versioned base produced duplicated version: %s", withVersion)
	}
}

func TestBuildGoogleModelURL_CustomBase(t *testing.T) {
	got := BuildGoogleModelURL("https://gateway.example/vertex", DefaultAPIVersion, "p", "us-central1", "gemini-1.5-pro", "generateContent")
	want := "https://gateway.example/vertex/v1/projects/p/locations/us-central1/publishers/google/models/gemini-1.5-pro:generateContent"
	if got != want {
		t.Fatalf("BuildGoogleModelURL custom = %q, want %q", got, want)
	}
}

func TestBuildAnthropicModelURL_CustomBase(t *testing.T) {
	got := BuildAnthropicModelURL("https://gateway.example/vertex", DefaultAPIVersion, "p", "global", "claude-sonnet-4@20250514", "rawPredict")
	want := "https://gateway.example/vertex/v1/projects/p/locations/global/publishers/anthropic/models/claude-sonnet-4@20250514:rawPredict"
	if got != want {
		t.Fatalf("BuildAnthropicModelURL custom = %q, want %q", got, want)
	}
}

func TestBuildOpenSourceChatCompletionsURL_CustomBase(t *testing.T) {
	got := BuildOpenSourceChatCompletionsURL("https://gateway.example/vertex", "p", "global")
	want := "https://gateway.example/vertex/v1beta1/projects/p/locations/global/endpoints/openapi/chat/completions"
	if got != want {
		t.Fatalf("BuildOpenSourceChatCompletionsURL custom = %q, want %q", got, want)
	}
}
