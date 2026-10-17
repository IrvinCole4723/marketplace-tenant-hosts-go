package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestProvisionAndTenantAsset(t *testing.T) {
	for _, tenant := range []string{"north", "south"} {
		t.Run(tenant, func(t *testing.T) {
			var calls []string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("Authorization") != "Bearer test-key" {
					t.Errorf("missing shared credential")
				}
				calls = append(calls, r.Method+" "+r.URL.Path)
				var body map[string]any
				_ = json.NewDecoder(r.Body).Decode(&body)
				data := any(map[string]string{})
				switch r.URL.Path {
				case "/v1/dns/domain/add":
					if body["domain"] != "market.example" {
						t.Errorf("domain: %v", body)
					}
					data = map[string]string{"zone_id": "zone-42"}
				case "/v1/dns/record/upsert":
					if body["zone_id"] != "zone-42" || body["name"] != tenant || body["record_type"] != "CNAME" {
						t.Errorf("record: %v", body)
					}
				case "/v1/storage/bucket/create":
					if body["name"] != "seller-media" {
						t.Errorf("bucket: %v", body)
					}
				default:
					if body["expires_seconds"] == nil {
						t.Errorf("expiry: %v", body)
					}
					data = map[string]string{"url": "https://uploads.example/signed"}
				}
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "data": data})
			}))
			defer server.Close()
			m := marketplace{api: client{base: server.URL, key: "test-key", http: server.Client()}, domain: "market.example", target: "origin.example", bucket: "seller-media"}
			got, err := m.provision(tenantRequest{Tenant: tenant, SellerAssets: []string{"catalog.jpg"}, BuyerUpdate: "paid", OrderHandoff: "ready for pickup"})
			if err != nil {
				t.Fatal(err)
			}
			if got.Hostname != tenant+".market.example" || got.Assets[0].TenantURL != "https://"+tenant+".market.example/assets/catalog.jpg" || got.OrderHandoff != "ready for pickup" {
				t.Fatalf("result: %+v", got)
			}
			req := httptest.NewRequest("GET", "http://"+got.Hostname+"/assets/catalog.jpg", nil)
			response := httptest.NewRecorder()
			m.serveAsset(response, req)
			if response.Code != http.StatusTemporaryRedirect || response.Header().Get("Location") != "https://uploads.example/signed" {
				t.Fatalf("asset: %d %s", response.Code, response.Body.String())
			}
			if len(calls) != 5 {
				t.Fatal(fmt.Sprint(calls))
			}
		})
	}
}
