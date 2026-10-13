package chatclient

import (
	"context"
	"net/http"
	"strings"
)

type Profile struct {
	Code        string `json:"code"`
	DisplayName string `json:"displayName"`
	Role        string `json:"role"`
}

// CurrentProfile returns the authenticated chat profile (id and display name).
func (c Client) CurrentProfile(ctx context.Context) (Profile, error) {
	var out struct {
		Profile Profile `json:"profile"`
	}
	err := c.json(ctx, http.MethodGet, strings.TrimRight(c.BaseURL, "/")+"/chat/v1/profile", nil, &out)
	return out.Profile, err
}
