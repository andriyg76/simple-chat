package chatclient

import (
	"context"
	"net/http"
	"net/url"
)

func (c Client) UpdateTitle(ctx context.Context, code, title string) (Session, error) {
	var out Session
	err := c.json(ctx, http.MethodPatch, c.base(code), map[string]string{"title": title}, &out)
	return out, err
}

func (c Client) ListMembers(ctx context.Context, code string) ([]string, error) {
	var out struct {
		Members []string `json:"members"`
	}
	err := c.json(ctx, http.MethodGet, c.base(code)+"/members", nil, &out)
	return out.Members, err
}

func (c Client) AddMember(ctx context.Context, code, profileID string) error {
	resp, err := c.request(ctx, http.MethodPost, c.base(code)+"/members", map[string]string{"profileId": profileID})
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return responseError(resp)
	}
	return nil
}

func (c Client) RemoveMember(ctx context.Context, code, profileID string) error {
	resp, err := c.request(ctx, http.MethodDelete, c.base(code)+"/members/"+url.PathEscape(profileID), nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return responseError(resp)
	}
	return nil
}
