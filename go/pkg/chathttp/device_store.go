package chathttp

import (
	"errors"
	"sync"
	"time"
)

var (
	ErrDeviceNotFound      = errors.New("device_not_found")
	ErrDeviceIDRequired    = errors.New("device_id_required")
	ErrPushTokenRequired   = errors.New("push_token_required")
	ErrInvalidPlatform     = errors.New("invalid_platform")
	ErrInvalidPushChannel  = errors.New("invalid_push_channel")
)

const deviceActiveTTL = 30 * 24 * time.Hour

// DeviceRegistration is the PUT body for device upsert.
type DeviceRegistration struct {
	Platform     string `json:"platform"`
	PushChannel  string `json:"pushChannel"`
	PushToken    string `json:"pushToken"`
	AppBundleID  string `json:"appBundleId,omitempty"`
	Locale       string `json:"locale,omitempty"`
}

// Device is the stored push endpoint for one install.
type Device struct {
	DeviceID    string    `json:"deviceId"`
	ProfileID   string    `json:"profileId"`
	Platform    string    `json:"platform"`
	PushChannel string    `json:"pushChannel"`
	PushToken   string    `json:"pushToken,omitempty"`
	AppBundleID string    `json:"appBundleId,omitempty"`
	Locale      string    `json:"locale,omitempty"`
	LastSeenAt  time.Time `json:"lastSeenAt"`
	Foreground  bool      `json:"foreground,omitempty"`
}

// DeviceStore persists mobile push endpoints (product layer in production).
type DeviceStore interface {
	Upsert(profileID, deviceID string, reg DeviceRegistration) (Device, error)
	Delete(profileID, deviceID string) error
	Touch(profileID, deviceID string, foreground bool) error
	ListActiveByProfiles(profileIDs []string) ([]Device, error)
}

type memoryDeviceKey struct {
	profileID string
	deviceID  string
}

// MemoryDeviceStore is an in-process DeviceStore for reference servers and tests.
type MemoryDeviceStore struct {
	mu      sync.Mutex
	devices map[memoryDeviceKey]Device
}

func NewMemoryDeviceStore() *MemoryDeviceStore {
	return &MemoryDeviceStore{devices: make(map[memoryDeviceKey]Device)}
}

func validateDeviceRegistration(reg DeviceRegistration) error {
	switch reg.Platform {
	case "ios", "macos", "android":
	default:
		return ErrInvalidPlatform
	}
	switch reg.PushChannel {
	case "apns", "fcm":
	default:
		return ErrInvalidPushChannel
	}
	if (reg.Platform == "ios" || reg.Platform == "macos") && reg.PushChannel != "apns" {
		return ErrInvalidPushChannel
	}
	if reg.Platform == "android" && reg.PushChannel != "fcm" {
		return ErrInvalidPushChannel
	}
	if reg.PushToken == "" {
		return ErrPushTokenRequired
	}
	return nil
}

func (s *MemoryDeviceStore) Upsert(profileID, deviceID string, reg DeviceRegistration) (Device, error) {
	if deviceID == "" {
		return Device{}, ErrDeviceIDRequired
	}
	if err := validateDeviceRegistration(reg); err != nil {
		return Device{}, err
	}
	now := time.Now().UTC()
	d := Device{
		DeviceID:    deviceID,
		ProfileID:   profileID,
		Platform:    reg.Platform,
		PushChannel: reg.PushChannel,
		PushToken:   reg.PushToken,
		AppBundleID: reg.AppBundleID,
		Locale:      reg.Locale,
		LastSeenAt:  now,
	}
	s.mu.Lock()
	s.devices[memoryDeviceKey{profileID, deviceID}] = d
	s.mu.Unlock()
	return d, nil
}

func (s *MemoryDeviceStore) Delete(profileID, deviceID string) error {
	if deviceID == "" {
		return ErrDeviceIDRequired
	}
	s.mu.Lock()
	key := memoryDeviceKey{profileID, deviceID}
	if _, ok := s.devices[key]; !ok {
		s.mu.Unlock()
		return ErrDeviceNotFound
	}
	delete(s.devices, key)
	s.mu.Unlock()
	return nil
}

func (s *MemoryDeviceStore) Touch(profileID, deviceID string, foreground bool) error {
	if deviceID == "" {
		return ErrDeviceIDRequired
	}
	s.mu.Lock()
	key := memoryDeviceKey{profileID, deviceID}
	d, ok := s.devices[key]
	if !ok {
		s.mu.Unlock()
		return ErrDeviceNotFound
	}
	d.LastSeenAt = time.Now().UTC()
	d.Foreground = foreground
	s.devices[key] = d
	s.mu.Unlock()
	return nil
}

func (s *MemoryDeviceStore) ListActiveByProfiles(profileIDs []string) ([]Device, error) {
	cutoff := time.Now().UTC().Add(-deviceActiveTTL)
	s.mu.Lock()
	out := make([]Device, 0)
	seen := make(map[string]struct{}, len(profileIDs))
	for _, id := range profileIDs {
		seen[id] = struct{}{}
	}
	for key, d := range s.devices {
		if _, ok := seen[key.profileID]; !ok {
			continue
		}
		if d.PushToken == "" || d.LastSeenAt.Before(cutoff) {
			continue
		}
		out = append(out, d)
	}
	s.mu.Unlock()
	return out, nil
}
