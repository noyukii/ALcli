package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	AnilistAuthURL     = "https://anilist.co/api/v2/oauth/authorize"
	AnilistTokenURL    = "https://anilist.co/api/v2/oauth/token"
	AnilistPinRedirect = "https://anilist.co/api/v2/oauth/pin"
)

func BuildAuthURL(clientID string) string {
	return fmt.Sprintf("%s?client_id=%s&redirect_uri=%s&response_type=code",
		AnilistAuthURL, clientID, AnilistPinRedirect)
}

// BuildImplicitAuthURL returns the implicit-grant URL. The PIN page displays the
// access token directly, so no client secret is needed (public CLI).
func BuildImplicitAuthURL(clientID string) string {
	return fmt.Sprintf("%s?client_id=%s&response_type=token",
		AnilistAuthURL, clientID)
}

func ExchangeCodeForToken(code, clientID, clientSecret string) (string, error) {
	payload := map[string]string{
		"grant_type":    "authorization_code",
		"client_id":     clientID,
		"client_secret": clientSecret,
		"redirect_uri":  AnilistPinRedirect,
		"code":          code,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	req, err := http.NewRequest(http.MethodPost, AnilistTokenURL, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("token exchange request failed: %w", err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		detail := string(raw)
		var parsed map[string]any
		if err := json.Unmarshal(raw, &parsed); err == nil {
			if msg, ok := parsed["message"].(string); ok && msg != "" {
				detail = msg
			}
		}
		return "", fmt.Errorf("token exchange failed (%d): %s", resp.StatusCode, detail)
	}
	var data map[string]any
	if err := json.Unmarshal(raw, &data); err != nil {
		return "", fmt.Errorf("invalid token response: %w", err)
	}
	token, _ := data["access_token"].(string)
	if token == "" {
		return "", fmt.Errorf("no access_token in response")
	}
	return token, nil
}
