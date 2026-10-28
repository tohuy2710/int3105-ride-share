package clients

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"int3105-ride-share/internal/contracts"
	"int3105-ride-share/internal/platform/fault"
	"io"
	"math"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type HTTP struct {
	base, token string
	client      *http.Client
}

func NewHTTP(base, token string) (*HTTP, error) {
	u, err := url.Parse(base)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return nil, fmt.Errorf("invalid service URL")
	}
	if len(token) < 16 {
		return nil, fmt.Errorf("INTERNAL_TOKEN must have at least 16 bytes")
	}
	return &HTTP{base: strings.TrimRight(base, "/"), token: token, client: &http.Client{Timeout: 8 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}, nil
}

func (h *HTTP) call(ctx context.Context, method, path string, in, out any, status int) error {
	var body io.Reader
	if in != nil {
		data, err := json.Marshal(in)
		if err != nil {
			return fault.Unavailable
		}
		body = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(ctx, method, h.base+path, body)
	if err != nil {
		return fault.Unavailable
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Internal-Token", h.token)
	resp, err := h.client.Do(req)
	if err != nil {
		return fault.Unavailable
	}
	defer resp.Body.Close()
	if resp.StatusCode != status {
		if resp.StatusCode == 404 && strings.HasPrefix(path, "/internal/rides/") {
			return fault.NotFound
		}
		return fault.Unavailable
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, (1<<20)+1))
	if err != nil || len(data) > 1<<20 || json.Unmarshal(data, out) != nil {
		return fault.Unavailable
	}
	return nil
}
func (h *HTTP) GetRide(ctx context.Context, id string) (contracts.Ride, error) {
	var out contracts.Ride
	err := h.call(ctx, "GET", "/internal/rides/"+url.PathEscape(id), nil, &out, 200)
	if err == nil && (out.RideID != id || strings.TrimSpace(out.FareID) == "" || strings.TrimSpace(out.RiderID) == "" || out.Status == "") {
		err = fault.Unavailable
	}
	return out, err
}
func (h *HTTP) Nearby(ctx context.Context, in contracts.NearbyInput) (contracts.NearbyResponse, error) {
	var out contracts.NearbyResponse
	query := url.Values{"lat": {fmt.Sprint(in.Lat)}, "long": {fmt.Sprint(in.Long)}, "radiusMeters": {fmt.Sprint(in.RadiusMeters)}}
	err := h.call(ctx, "GET", "/internal/drivers/nearby?"+query.Encode(), nil, &out, 200)
	if err == nil {
		if out.Drivers == nil {
			return out, fault.Unavailable
		}
		for _, driver := range out.Drivers {
			p := driver.Location
			if strings.TrimSpace(driver.DriverID) == "" || driver.DistanceMeters < 0 || math.IsNaN(p.Lat) || math.IsNaN(p.Long) || p.Lat < -90 || p.Lat > 90 || p.Long < -180 || p.Long > 180 {
				return out, fault.Unavailable
			}
		}
	}
	return out, err
}
func (h *HTTP) Request(ctx context.Context, in contracts.MatchingRequest) (contracts.MatchingResponse, error) {
	var out contracts.MatchingResponse
	err := h.call(ctx, "POST", "/internal/matching/requests", in, &out, 202)
	if err == nil && (out.RideID != in.RideID || (out.Status != "MATCHING" && out.Status != "NO_DRIVERS" && out.Status != "MATCHED" && out.Status != "CANCELLED")) {
		err = fault.Unavailable
	}
	return out, err
}
func (h *HTTP) Get(ctx context.Context, id string) (contracts.MatchingResponse, error) {
	var out contracts.MatchingResponse
	err := h.call(ctx, "GET", "/internal/matching/requests/"+url.PathEscape(id), nil, &out, 200)
	return out, err
}
func (h *HTTP) RequestRide(ctx context.Context, in contracts.NotificationRequest) (contracts.NotificationResponse, error) {
	var out contracts.NotificationResponse
	err := h.call(ctx, "POST", "/internal/notifications/ride-requests", in, &out, 202)
	if err == nil && (strings.TrimSpace(out.NotificationID) == "" || out.RideID != in.RideID || out.DriverID != in.DriverID || out.Type != in.Type || out.Status != "QUEUED") {
		err = fault.Unavailable
	}
	return out, err
}
func (h *HTTP) Send(ctx context.Context, in contracts.NotificationRequest) (contracts.NotificationResponse, error) {
	return h.RequestRide(ctx, in)
}
