package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type apiError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (e apiError) Error() string { return e.Code + ": " + e.Message }

type client struct {
	base string
	key  string
	http *http.Client
}

func (c client) call(method, path string, body any, out any) error {
	var payload []byte
	if body != nil {
		var err error
		payload, err = json.Marshal(body)
		if err != nil {
			return err
		}
	}
	for attempt := 0; attempt < 4; attempt++ {
		req, err := http.NewRequest(method, c.base+path, bytes.NewReader(payload))
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Bearer "+c.key)
		if body != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		res, err := c.http.Do(req)
		if err != nil {
			return err
		}
		var env struct {
			OK    bool            `json:"ok"`
			Data  json.RawMessage `json:"data"`
			Error apiError        `json:"error"`
		}
		decodeErr := json.NewDecoder(io.LimitReader(res.Body, 1<<20)).Decode(&env)
		res.Body.Close()
		if res.StatusCode == http.StatusTooManyRequests && attempt < 3 {
			wait := time.Duration(1<<attempt) * time.Second
			if seconds, err := strconv.Atoi(res.Header.Get("Retry-After")); err == nil && seconds >= 0 {
				wait = time.Duration(seconds) * time.Second
			}
			time.Sleep(wait)
			continue
		}
		if decodeErr != nil {
			return fmt.Errorf("decode Infrai response (HTTP %d): %w", res.StatusCode, decodeErr)
		}
		if !env.OK {
			return env.Error
		}
		if res.StatusCode >= 500 {
			return fmt.Errorf("Infrai HTTP %d", res.StatusCode)
		}
		if out != nil {
			return json.Unmarshal(env.Data, out)
		}
		return nil
	}
	return errors.New("retry limit reached")
}

type tenantRequest struct {
	Tenant       string   `json:"tenant"`
	SellerAssets []string `json:"seller_assets"`
	BuyerUpdate  string   `json:"buyer_update"`
	OrderHandoff string   `json:"order_handoff"`
}

type assetLink struct {
	Key       string `json:"key"`
	UploadURL string `json:"upload_url"`
	TenantURL string `json:"tenant_url"`
}

type tenantResult struct {
	Hostname     string      `json:"hostname"`
	Assets       []assetLink `json:"assets"`
	BuyerUpdate  string      `json:"buyer_update"`
	OrderHandoff string      `json:"order_handoff"`
}

type marketplace struct {
	api    client
	domain string
	target string
	bucket string
}

func validLabel(s string) bool {
	if len(s) == 0 || len(s) > 63 || s[0] == '-' || s[len(s)-1] == '-' {
		return false
	}
	for _, r := range s {
		if !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-') {
			return false
		}
	}
	return true
}

func (m marketplace) provision(input tenantRequest) (tenantResult, error) {
	if !validLabel(input.Tenant) || len(input.SellerAssets) == 0 || input.BuyerUpdate == "" || input.OrderHandoff == "" {
		return tenantResult{}, errors.New("tenant, seller_assets, buyer_update and order_handoff are required")
	}
	for _, key := range input.SellerAssets {
		if key == "" || strings.Contains(key, "/") || strings.Contains(key, "..") {
			return tenantResult{}, errors.New("asset keys must be nonempty single path segments")
		}
	}
	var zone struct {
		ZoneID string `json:"zone_id"`
	}
	if err := m.api.call("POST", "/v1/dns/domain/add", map[string]any{"domain": m.domain}, &zone); err != nil {
		return tenantResult{}, err
	}
	if zone.ZoneID == "" {
		return tenantResult{}, errors.New("missing zone_id")
	}
	if err := m.api.call("PUT", "/v1/dns/record/upsert", map[string]any{"zone_id": zone.ZoneID, "record_type": "CNAME", "name": input.Tenant, "content": m.target}, nil); err != nil {
		return tenantResult{}, err
	}
	if err := m.api.call("POST", "/v1/storage/bucket/create", map[string]any{"name": m.bucket}, nil); err != nil {
		return tenantResult{}, err
	}
	host := input.Tenant + "." + m.domain
	result := tenantResult{Hostname: host, BuyerUpdate: input.BuyerUpdate, OrderHandoff: input.OrderHandoff}
	for _, key := range input.SellerAssets {
		var signed struct {
			URL string `json:"url"`
		}
		path := "/v1/storage/object/presign/" + url.PathEscape(m.bucket) + "/" + url.PathEscape(input.Tenant+"-"+key)
		if err := m.api.call("POST", path, map[string]any{"op": "put", "expires_seconds": 900, "idempotency_key": input.Tenant + "-" + key}, &signed); err != nil {
			return tenantResult{}, err
		}
		result.Assets = append(result.Assets, assetLink{Key: key, UploadURL: signed.URL, TenantURL: "https://" + host + "/assets/" + url.PathEscape(key)})
	}
	return result, nil
}

func (m marketplace) serveAsset(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	host := strings.Split(r.Host, ":")[0]
	tenant, ok := strings.CutSuffix(host, "."+m.domain)
	key := strings.TrimPrefix(r.URL.Path, "/assets/")
	if !ok || !validLabel(tenant) || key == "" || strings.Contains(key, "/") || strings.Contains(key, "..") {
		http.Error(w, "invalid asset address", http.StatusBadRequest)
		return
	}
	var signed struct {
		URL string `json:"url"`
	}
	path := "/v1/storage/object/presign/" + url.PathEscape(m.bucket) + "/" + url.PathEscape(tenant+"-"+key)
	if err := m.api.call("POST", path, map[string]any{"op": "get", "expires_seconds": 300}, &signed); err != nil {
		writeError(w, err)
		return
	}
	http.Redirect(w, r, signed.URL, http.StatusTemporaryRedirect)
}

func writeError(w http.ResponseWriter, err error) {
	var remote apiError
	if errors.As(err, &remote) {
		http.Error(w, remote.Error(), http.StatusBadRequest)
		return
	}
	http.Error(w, err.Error(), http.StatusBadGateway)
}
