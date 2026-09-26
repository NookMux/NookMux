package oauth

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func withGitHubTestServer(t *testing.T, handler http.HandlerFunc) {
	t.Helper()

	server := httptest.NewServer(handler)
	oldBase := githubAPIBaseURL
	githubAPIBaseURL = server.URL
	t.Cleanup(func() {
		githubAPIBaseURL = oldBase
		server.Close()
	})
}

func TestLookupGitHubLoginSuccess(t *testing.T) {
	withGitHubTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/users/octocat" {
			t.Errorf("path = %q, want /users/octocat", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"id":583231,"login":"octocat"}`)
	})

	id, err := LookupGitHubLogin(context.Background(), "octocat")
	if err != nil {
		t.Fatalf("lookup: %v", err)
	}
	if id != 583231 {
		t.Fatalf("id = %d, want 583231", id)
	}
}

func TestLookupGitHubLoginNotFound(t *testing.T) {
	withGitHubTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})

	_, err := LookupGitHubLogin(context.Background(), "no-such-user")
	if !errors.Is(err, ErrGitHubLookupNotFound) {
		t.Fatalf("err = %v, want ErrGitHubLookupNotFound", err)
	}
}

func TestLookupGitHubLoginRateLimited(t *testing.T) {
	withGitHubTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-RateLimit-Remaining", "0")
		w.WriteHeader(http.StatusForbidden)
	})

	_, err := LookupGitHubLogin(context.Background(), "octocat")
	if !errors.Is(err, ErrGitHubLookupRateLimited) {
		t.Fatalf("err = %v, want ErrGitHubLookupRateLimited", err)
	}
}

func TestLookupGitHubLoginServerError(t *testing.T) {
	withGitHubTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})

	_, err := LookupGitHubLogin(context.Background(), "octocat")
	if !errors.Is(err, ErrGitHubLookupFailed) {
		t.Fatalf("err = %v, want ErrGitHubLookupFailed", err)
	}
}

func TestLookupGitHubLoginEmptyID(t *testing.T) {
	withGitHubTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"id":0,"login":"octocat"}`)
	})

	_, err := LookupGitHubLogin(context.Background(), "octocat")
	if !errors.Is(err, ErrGitHubLookupFailed) {
		t.Fatalf("err = %v, want ErrGitHubLookupFailed for empty id", err)
	}
}
