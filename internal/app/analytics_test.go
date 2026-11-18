package app

import (
	"strings"
	"testing"
)

func TestAnalyticsInjectionReplacesPlaceholders(t *testing.T) {
	t.Setenv("UMAMI_WEBSITE_ID", "umami-site")
	t.Setenv("GOOGLE_ANALYTICS_ID", "ga-site")

	indexPage := []byte("<html><!--umami-->\n<!--Google Analytics-->\n</html>")
	indexPage = InjectUmamiAnalytics(indexPage)
	indexPage = InjectGoogleAnalytics(indexPage)

	page := string(indexPage)
	if strings.Contains(page, "<!--umami-->\n") || strings.Contains(page, "<!--Google Analytics-->\n") {
		t.Fatalf("analytics placeholders were not replaced: %s", page)
	}
	for _, want := range []string{
		`data-website-id="umami-site"`,
		"gtag/js?id=ga-site",
		"<!--Umami NookMux-->\n",
		"<!--Google Analytics NookMux-->\n",
	} {
		if !strings.Contains(page, want) {
			t.Fatalf("analytics injection missing %q: %s", want, page)
		}
	}
}
