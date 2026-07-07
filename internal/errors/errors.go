// Package errors classifies Notion API failures into the family error
// contract ({error, fixable_by, hint}), porting the TS handleAction guidance
// messages. Commands funnel backend/client errors through Classify so every
// group shares one classification seam (the agent-mongo/agent-slack pattern).
package errors

import (
	"context"
	stderrors "errors"
	"fmt"
	"net"

	"github.com/shhac/agent-notion/internal/notion/official"
	v3 "github.com/shhac/agent-notion/internal/notion/v3"
	output "github.com/shhac/lib-agent-output"
)

// Classify wraps err with a fixable_by classification and an actionable hint.
// Already-classified errors pass through untouched; nil stays nil.
func Classify(err error) error {
	if err == nil {
		return nil
	}

	var already *output.Error
	if stderrors.As(err, &already) {
		return err
	}

	var v3Err *v3.HTTPError
	if stderrors.As(err, &v3Err) {
		return classifyV3(v3Err)
	}

	var apiErr *official.APIError
	if stderrors.As(err, &apiErr) {
		return classifyOfficial(apiErr)
	}

	if stderrors.Is(err, context.DeadlineExceeded) {
		return output.Wrap(err, output.FixableByRetry).
			WithHint("the request timed out; retry, or raise --timeout")
	}
	var netErr net.Error
	if stderrors.As(err, &netErr) && netErr.Timeout() {
		return output.Wrap(err, output.FixableByRetry).
			WithHint("the request timed out; retry, or raise --timeout")
	}

	return output.Wrap(err, output.FixableByAgent)
}

func classifyV3(err *v3.HTTPError) error {
	switch {
	case err.Status == 401:
		return output.New("desktop token expired", output.FixableByHuman).
			WithHint("run 'agent-notion auth import-desktop' (or import-browser) to re-import")
	case err.Status == 403:
		return output.New("access denied: the desktop token may not have access to this resource, or it may have expired",
			output.FixableByHuman).
			WithHint("open the page in Notion to confirm access, or re-import the token")
	case err.Status == 404:
		return output.New("not found", output.FixableByAgent).
			WithHint("check the ID, or ensure the page is accessible with your desktop token")
	case err.Status == 429:
		return output.New("rate limited by Notion", output.FixableByRetry).
			WithHint("wait a moment and retry")
	case err.Status >= 500:
		return output.Wrap(err, output.FixableByRetry).
			WithHint("Notion server error; retry shortly")
	default:
		return output.Wrap(err, output.FixableByAgent)
	}
}

func classifyOfficial(err *official.APIError) error {
	switch err.Code {
	case "unauthorized":
		return output.New("not authenticated with the Notion API", output.FixableByHuman).
			WithHint("set NOTION_API_KEY/NOTION_TOKEN, or run 'agent-notion auth login' or 'agent-notion auth import'")
	case "restricted_resource":
		return output.New("the integration is not allowed to access this resource", output.FixableByHuman).
			WithHint("adjust the integration's capabilities at https://www.notion.so/my-integrations")
	case "object_not_found":
		return output.New("not found: the integration may not have access", output.FixableByAgent).
			WithHint("share the resource with your integration in Notion, or check the ID")
	case "validation_error":
		return output.New(fmt.Sprintf("Notion API validation error: %s", err.Message), output.FixableByAgent)
	case "rate_limited":
		return output.New("rate limited by the Notion API", output.FixableByRetry).
			WithHint("wait a moment and retry")
	default:
		if err.Status >= 500 {
			return output.Wrap(err, output.FixableByRetry).
				WithHint("Notion server error; retry shortly")
		}
		return output.Wrap(err, output.FixableByAgent)
	}
}
