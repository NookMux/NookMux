package notify

import "testing"

func TestNotifyHeaders_UseNookMuxBrandUserAgent(t *testing.T) {
	barkHeaders := notifyHeaders(barkNotifyUserAgent, "")
	if got := barkHeaders["User-Agent"]; got != "NookMux-Bark-Notify/1.0" {
		t.Fatalf("bark User-Agent = %q, want %q", got, "NookMux-Bark-Notify/1.0")
	}
	if _, exists := barkHeaders["Content-Type"]; exists {
		t.Fatalf("unexpected Content-Type header for bark: %q", barkHeaders["Content-Type"])
	}

	gotifyHeaders := notifyHeaders(gotifyNotifyUserAgent, "application/json; charset=utf-8")
	if got := gotifyHeaders["User-Agent"]; got != "NookMux-Gotify-Notify/1.0" {
		t.Fatalf("gotify User-Agent = %q, want %q", got, "NookMux-Gotify-Notify/1.0")
	}
	if got := gotifyHeaders["Content-Type"]; got != "application/json; charset=utf-8" {
		t.Fatalf("gotify Content-Type = %q, want %q", got, "application/json; charset=utf-8")
	}
}
