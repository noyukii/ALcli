package api

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestNamedFollowsAndMessagesQueries(t *testing.T) {
	for _, tc := range []struct {
		kind string
		id   int
		want string
	}{
		{"follows", 12, "following(userId: $id)"},
		{"messages", 13, "activities(messengerId: $id, type: MESSAGE)"},
	} {
		client := NewClient("token")
		client.httpClient.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
			var payload struct {
				Query     string         `json:"query"`
				Variables map[string]any `json:"variables"`
			}
			if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(payload.Query, tc.want) || payload.Variables["id"] != float64(tc.id) {
				t.Errorf("%s query: %s %#v", tc.kind, payload.Query, payload.Variables)
			}
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"data":{"Page":{"pageInfo":{"hasNextPage":false}}}}`)), Header: make(http.Header)}, nil
		})
		if _, err := client.Named(context.Background(), tc.kind, "list", NamedInput{ID: tc.id}); err != nil {
			t.Fatal(err)
		}
	}
}

func TestDocumentKind(t *testing.T) {
	for input, want := range map[string]string{"{ Viewer { id } }": "query", "# comment\nmutation { DeleteActivity(id: 1) { deleted } }": "mutation", "subscription { thing }": ""} {
		if got := DocumentKind(input); got != want {
			t.Errorf("%q: %q, want %q", input, got, want)
		}
	}
}

func TestNamedMutationUsesFixedDocumentAndVariables(t *testing.T) {
	client := NewClient("saved-token")
	client.httpClient.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		var payload struct {
			Query     string         `json:"query"`
			Variables map[string]any `json:"variables"`
		}
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(payload.Query, "ToggleFavourite(animeId: $id)") || payload.Variables["id"] != float64(42) {
			t.Errorf("payload = %#v", payload)
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"data":{"ToggleFavourite":{}}}`)), Header: make(http.Header)}, nil
	})
	if _, err := client.NamedMutation(context.Background(), "favorites_toggle", map[string]any{"kind": "anime", "id": 42}); err != nil {
		t.Fatal(err)
	}
	if _, err := client.NamedMutation(context.Background(), "favorites_toggle", map[string]any{"kind": "invalid", "id": 42}); err == nil {
		t.Fatal("invalid favorite kind accepted")
	}
}

func TestFavoritesListValidatesKindAndPage(t *testing.T) {
	c := NewClient("saved-token")
	if _, err := c.FavoritesList(context.Background(), "anime", 1, 26); err == nil {
		t.Fatal("accepted >25 per page")
	}
	if _, err := c.FavoritesList(context.Background(), "invalid", 1, 20); err == nil {
		t.Fatal("accepted invalid kind")
	}
}
