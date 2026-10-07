package chatclient

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type ChallengeStart struct {
	ChallengeID      string `json:"challengeId"`
	VerificationURL  string `json:"verificationUrl"`
	Secret           string `json:"secret"`
	ExpiresInSeconds int    `json:"expiresInSeconds"`
	PollURL          string `json:"pollUrl"`
}

type ChallengeStatus struct {
	Status           string `json:"status"`
	Token            string `json:"token"`
	ExpiresInSeconds int    `json:"expiresInSeconds"`
	Profile          struct {
		Code        string `json:"code"`
		DisplayName string `json:"displayName"`
		Role        string `json:"role"`
	} `json:"profile"`
}

// StartEnrollmentChallenge begins mode B enrollment.
func (c Client) StartEnrollmentChallenge(ctx context.Context, clientLabel string) (ChallengeStart, error) {
	body := map[string]string{}
	if strings.TrimSpace(clientLabel) != "" {
		body["clientLabel"] = clientLabel
	}
	var out ChallengeStart
	err := c.json(ctx, http.MethodPost, strings.TrimRight(c.BaseURL, "/")+"/chat/v1/enrollments/challenges", body, &out)
	return out, err
}

// WaitEnrollmentChallenge polls until completed or ctx ends.
func (c Client) WaitEnrollmentChallenge(ctx context.Context, challengeID string, waitSeconds int) (ChallengeStatus, error) {
	target := strings.TrimRight(c.BaseURL, "/") + "/chat/v1/enrollments/challenges/" + challengeID
	if waitSeconds > 0 {
		target += "?wait=" + strconv.Itoa(waitSeconds)
	}
	var out ChallengeStatus
	err := c.json(ctx, http.MethodGet, target, nil, &out)
	return out, err
}

// RefreshToken exchanges the current bearer for a new one (standalone).
func (c Client) RefreshToken(ctx context.Context) (string, time.Duration, error) {
	var out struct {
		Token            string `json:"token"`
		ExpiresInSeconds int    `json:"expiresInSeconds"`
	}
	err := c.json(ctx, http.MethodPost, strings.TrimRight(c.BaseURL, "/")+"/chat/v1/tokens/refresh", nil, &out)
	return out.Token, time.Duration(out.ExpiresInSeconds) * time.Second, err
}

// ApplyTokenRotated updates Client.Token from a token_rotated inbox event text JSON.
func (c *Client) ApplyTokenRotated(event InboxEvent) (bool, error) {
	if event.Kind != "token_rotated" {
		return false, nil
	}
	var payload struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal([]byte(event.Text), &payload); err != nil {
		return false, err
	}
	if payload.Token == "" {
		return false, nil
	}
	c.Token = payload.Token
	return true, nil
}
