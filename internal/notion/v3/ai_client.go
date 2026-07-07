// The three v3 AI endpoints, as methods on Client. Kept separate from
// client.go (Phase 4) since AI landed later; they share the same post/timeout
// plumbing.

package v3

import "context"

// GetAvailableModels lists the AI models available in a space.
func (c *Client) GetAvailableModels(ctx context.Context, spaceID string) (GetAvailableModelsResponse, error) {
	var out GetAvailableModelsResponse
	err := c.post(ctx, "getAvailableModels", map[string]any{"spaceId": spaceID}, &out, defaultTimeout, nil)
	return out, err
}

// GetInferenceTranscriptsForUser lists the user's AI chat threads in a space.
func (c *Client) GetInferenceTranscriptsForUser(ctx context.Context, spaceID string, limit int) (GetInferenceTranscriptsResponse, error) {
	body := map[string]any{
		"threadParentPointer": map[string]any{
			"table":   "space",
			"id":      spaceID,
			"spaceId": spaceID,
		},
		"limit": orInt(limit, 50),
	}
	var out GetInferenceTranscriptsResponse
	err := c.post(ctx, "getInferenceTranscriptsForUser", body, &out, defaultTimeout, nil)
	return out, err
}

// MarkInferenceTranscriptSeen marks a chat thread as read.
func (c *Client) MarkInferenceTranscriptSeen(ctx context.Context, spaceID, threadID string) (MarkTranscriptSeenResponse, error) {
	body := map[string]any{"spaceId": spaceID, "threadId": threadID}
	var out MarkTranscriptSeenResponse
	err := c.post(ctx, "markInferenceTranscriptSeen", body, &out, defaultTimeout, nil)
	return out, err
}
