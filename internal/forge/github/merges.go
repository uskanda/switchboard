package github

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/uskanda/switchboard/internal/forge"
)

var _ forge.Merges = (*Client)(nil)

const readyMutation = `mutation($id: ID!) {
  markPullRequestReadyForReview(input: {pullRequestId: $id}) { pullRequest { isDraft } }
}`

// MergeChangeRequest implements forge.Merges. The head is checked before the draft is marked ready, so a pull request
// that moved is left as it was; the merge itself carries sha too, which GitHub refuses with 409 when the head moved
// in between (V38).
func (c *Client) MergeChangeRequest(ctx context.Context, owner, repo string, number int, sha string) error {
	var pr struct {
		State  string `json:"state"`
		Merged bool   `json:"merged"`
		Draft  bool   `json:"draft"`
		NodeID string `json:"node_id"`
		Head   struct {
			SHA string `json:"sha"`
		} `json:"head"`
	}
	path := fmt.Sprintf("/repos/%s/%s/pulls/%d", owner, repo, number)
	if _, err := c.do(ctx, http.MethodGet, c.url(path, nil), nil, &pr, nil); err != nil {
		return err
	}
	switch {
	case pr.Merged:
		return &forge.NotMergeableError{Reason: fmt.Sprintf("pull request #%d is already merged", number)}
	case pr.State != "open":
		return &forge.NotMergeableError{Reason: fmt.Sprintf("pull request #%d is closed", number)}
	case pr.Head.SHA != sha:
		return fmt.Errorf("github: pull request #%d: %w", number, forge.ErrHeadMoved)
	}
	if pr.Draft {
		if err := c.markReady(ctx, pr.NodeID); err != nil {
			return err
		}
	}
	_, err := c.do(ctx, http.MethodPut, c.url(path+"/merge", nil), map[string]any{"sha": sha}, nil, nil)
	var ae *apiError
	if errors.As(err, &ae) {
		switch ae.status {
		case http.StatusConflict:
			return fmt.Errorf("%w: %w", forge.ErrHeadMoved, err)
		case http.StatusMethodNotAllowed, http.StatusUnprocessableEntity:
			reason := ae.message
			if ae.details != "" {
				reason += ": " + ae.details
			}
			return &forge.NotMergeableError{Reason: reason}
		}
	}
	return err
}

// markReady takes a pull request out of draft. The REST API cannot; only this GraphQL mutation can.
func (c *Client) markReady(ctx context.Context, nodeID string) error {
	in := map[string]any{"query": readyMutation, "variables": map[string]any{"id": nodeID}}
	var out struct {
		Errors []struct {
			Type    string `json:"type"`
			Message string `json:"message"`
		} `json:"errors"`
	}
	if _, err := c.do(ctx, http.MethodPost, c.graphqlURL, in, &out, nil); err != nil {
		return err
	}
	if len(out.Errors) == 0 {
		return nil
	}
	e := out.Errors[0]
	err := fmt.Errorf("github: graphql: markPullRequestReadyForReview: %s", e.Message)
	// GraphQL answers 200 with an error of type FORBIDDEN when the token lacks the permission.
	if e.Type == "FORBIDDEN" || strings.Contains(e.Message, "not accessible") {
		return fmt.Errorf("%w: %w", forge.ErrForbidden, err)
	}
	return err
}
