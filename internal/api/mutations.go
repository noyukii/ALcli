package api

import (
	"context"
	"fmt"
	"strings"
)

type mutationSpec struct {
	document string
	required []string
}

var namedMutations = map[string]mutationSpec{
	"follows_toggle":       {"mutation($userId: Int) { ToggleFollow(userId: $userId) { id name isFollowing } }", []string{"userId"}},
	"activities_post":      {"mutation($text: String) { SaveTextActivity(text: $text) { id text } }", []string{"text"}},
	"activities_delete":    {"mutation($id: Int) { DeleteActivity(id: $id) { deleted } }", []string{"id"}},
	"messages_send":        {"mutation($recipientId: Int, $message: String) { SaveMessageActivity(recipientId: $recipientId, message: $message) { id message } }", []string{"recipientId", "message"}},
	"forums_post":          {"mutation($title: String, $body: String) { SaveThread(title: $title, body: $body) { id title } }", []string{"title", "body"}},
	"forums_delete":        {"mutation($id: Int) { DeleteThread(id: $id) { deleted } }", []string{"id"}},
	"reviews_save":         {"mutation($mediaId: Int, $summary: String, $body: String, $score: Int) { SaveReview(mediaId: $mediaId, summary: $summary, body: $body, score: $score) { id summary score } }", []string{"mediaId", "summary", "body", "score"}},
	"reviews_delete":       {"mutation($id: Int) { DeleteReview(id: $id) { deleted } }", []string{"id"}},
	"recommendations_save": {"mutation($mediaId: Int, $mediaRecommendationId: Int, $rating: RecommendationRating) { SaveRecommendation(mediaId: $mediaId, mediaRecommendationId: $mediaRecommendationId, rating: $rating) { id rating } }", []string{"mediaId", "mediaRecommendationId", "rating"}},
}

var namedMutationHelp = map[string]string{
	"follows_toggle":       "Toggle following a user; variables: userId",
	"favorites_toggle":     "Toggle a favorite; variables: kind (anime, manga, character, staff, studio), id",
	"activities_post":      "Post a text activity; variables: text",
	"activities_delete":    "Delete own activity; variables: id",
	"messages_send":        "Send a message; variables: recipientId, message",
	"forums_post":          "Create a forum thread; variables: title, body",
	"forums_delete":        "Delete own forum thread; variables: id",
	"reviews_save":         "Create a review; variables: mediaId, summary (20-120 chars), body (2600+ chars), score (0-100)",
	"reviews_delete":       "Delete own review; variables: id",
	"recommendations_save": "Recommend media; variables: mediaId, mediaRecommendationId, rating (RATE_UP, RATE_DOWN, NO_RATING)",
}

func NamedMutationHelp(name string) string { return namedMutationHelp[name] }

func NamedMutationNames() []string {
	return []string{"follows_toggle", "favorites_toggle", "activities_post", "activities_delete", "messages_send", "forums_post", "forums_delete", "reviews_save", "reviews_delete", "recommendations_save"}
}

// NamedMutation executes an allowlisted AniList account change. Variables are
// passed as GraphQL variables, never interpolated into a document.
func (c *Client) NamedMutation(ctx context.Context, name string, variables map[string]any) (map[string]any, error) {
	if c.token == "" {
		return nil, fmt.Errorf("%s requires AniList login", name)
	}
	if name == "favorites_toggle" {
		kind, ok := variables["kind"].(string)
		if !ok {
			return nil, fmt.Errorf("favorites_toggle requires kind")
		}
		field := map[string]string{"anime": "animeId", "manga": "mangaId", "character": "characterId", "staff": "staffId", "studio": "studioId"}[strings.ToLower(kind)]
		if field == "" {
			return nil, fmt.Errorf("favorite kind must be anime, manga, character, staff, or studio")
		}
		if variables["id"] == nil {
			return nil, fmt.Errorf("favorites_toggle requires id")
		}
		doc := fmt.Sprintf("mutation($id: Int) { ToggleFavourite(%s: $id) { %s { nodes { id } } } }", field, map[string]string{"anime": "anime", "manga": "manga", "character": "characters", "staff": "staff", "studio": "studios"}[strings.ToLower(kind)])
		return c.Execute(ctx, doc, map[string]any{"id": variables["id"]})
	}
	spec, ok := namedMutations[name]
	if !ok {
		return nil, fmt.Errorf("unknown named mutation %q", name)
	}
	for _, key := range spec.required {
		if variables[key] == nil {
			return nil, fmt.Errorf("%s requires %s", name, key)
		}
	}
	return c.Execute(ctx, spec.document, variables)
}
