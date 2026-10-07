package chathttp

import (
	"errors"
	"net/http"
	"strings"
)

func (h *Handler) putDevice(w http.ResponseWriter, r *http.Request) {
	participant, ok := h.authenticate(w, r)
	if !ok {
		return
	}
	deviceID := strings.TrimSpace(r.PathValue("deviceId"))
	if deviceID == "" {
		writeError(w, http.StatusUnprocessableEntity, "device_id_required", "deviceId is required")
		return
	}
	var reg DeviceRegistration
	if !decode(w, r, &reg) {
		return
	}
	d, err := h.devices.Upsert(participant, deviceID, reg)
	if h.deviceError(w, err) {
		return
	}
	writeJSON(w, http.StatusOK, d)
}

func (h *Handler) deleteDevice(w http.ResponseWriter, r *http.Request) {
	participant, ok := h.authenticate(w, r)
	if !ok {
		return
	}
	deviceID := strings.TrimSpace(r.PathValue("deviceId"))
	if deviceID == "" {
		writeError(w, http.StatusUnprocessableEntity, "device_id_required", "deviceId is required")
		return
	}
	if h.deviceError(w, h.devices.Delete(participant, deviceID)) {
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) heartbeatDevice(w http.ResponseWriter, r *http.Request) {
	participant, ok := h.authenticate(w, r)
	if !ok {
		return
	}
	deviceID := strings.TrimSpace(r.PathValue("deviceId"))
	if deviceID == "" {
		writeError(w, http.StatusUnprocessableEntity, "device_id_required", "deviceId is required")
		return
	}
	var body struct {
		Foreground bool `json:"foreground"`
	}
	if r.ContentLength > 0 {
		if !decode(w, r, &body) {
			return
		}
	}
	if h.deviceError(w, h.devices.Touch(participant, deviceID, body.Foreground)) {
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) deviceError(w http.ResponseWriter, err error) bool {
	if err == nil {
		return false
	}
	switch {
	case errors.Is(err, ErrDeviceIDRequired):
		writeError(w, http.StatusUnprocessableEntity, "device_id_required", "deviceId is required")
	case errors.Is(err, ErrPushTokenRequired):
		writeError(w, http.StatusUnprocessableEntity, "push_token_required", "pushToken is required")
	case errors.Is(err, ErrInvalidPlatform):
		writeError(w, http.StatusUnprocessableEntity, "invalid_platform", "invalid platform")
	case errors.Is(err, ErrInvalidPushChannel):
		writeError(w, http.StatusUnprocessableEntity, "invalid_push_channel", "invalid pushChannel")
	case errors.Is(err, ErrDeviceNotFound):
		writeError(w, http.StatusNotFound, "device_not_found", "device not found")
	default:
		writeError(w, http.StatusInternalServerError, "device", err.Error())
	}
	return true
}
