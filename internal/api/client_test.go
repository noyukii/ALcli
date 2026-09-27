package api

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

func TestGetViewerSendsBearerTokenAndParsesResponse(t *testing.T) {
	client := NewClient("token-value")
	client.httpClient.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.String() != GraphQLURL {
			t.Errorf("request URL = %q", req.URL)
		}
		if got := req.Header.Get("Authorization"); got != "Bearer token-value" {
			t.Errorf("Authorization = %q", got)
		}
		body, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(body), "GetViewer") && !strings.Contains(string(body), "Viewer") {
			t.Errorf("request body = %s", body)
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"data":{"Viewer":{"id":42,"name":"test-user","avatar":{}}}}`)), Header: make(http.Header)}, nil
	})

	user, err := client.GetViewer()
	if err != nil {
		t.Fatalf("GetViewer(): %v", err)
	}
	if user.ID != 42 || user.Name != "test-user" {
		t.Fatalf("user = %#v", user)
	}
}

func TestSaveListEntrySendsMutationAndParsesSavedEntry(t *testing.T) {
	client := NewClient("token-value")
	client.httpClient.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		body, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(body), "SaveMediaListEntry") || !strings.Contains(string(body), `"mediaId":42`) || !strings.Contains(string(body), `"status":"CURRENT"`) {
			t.Errorf("mutation request body = %s", body)
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"data":{"SaveMediaListEntry":{"id":7,"status":"CURRENT","progress":3,"score":8,"media":{"id":42,"title":{"romaji":"Demo","english":"Demo EN"},"type":"ANIME"}}}}`)), Header: make(http.Header)}, nil
	})

	entry, err := client.SaveListEntry(42, "CURRENT", 3, 8, "", nil, 0, false)
	if err != nil {
		t.Fatalf("SaveListEntry(): %v", err)
	}
	if entry.ID != 7 || entry.Media.ID != 42 || entry.Progress != 3 || entry.Status != "CURRENT" {
		t.Fatalf("entry = %#v", entry)
	}
}

func TestUnauthorizedResponseIsAnError(t *testing.T) {
	client := NewClient("expired")
	client.httpClient.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusUnauthorized, Body: io.NopCloser(strings.NewReader(`{"message":"expired"}`)), Header: make(http.Header)}, nil
	})
	if _, err := client.GetViewer(); err == nil || !strings.Contains(err.Error(), "Unauthorized") {
		t.Fatalf("GetViewer() error = %v", err)
	}
}

func TestRateLimitExposesRetryAfter(t *testing.T) {
	client := NewClient("")
	client.httpClient.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 429, Body: io.NopCloser(strings.NewReader(`{"errors":[{"message":"Too Many Requests"}]}`)), Header: http.Header{"Retry-After": []string{"30"}}}, nil
	})
	_, err := client.Execute(context.Background(), "query { Viewer { id } }", nil)
	if err == nil || !strings.Contains(err.Error(), "30 seconds") {
		t.Fatalf("rate limit error=%v", err)
	}
}
