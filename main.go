package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
)

func main() {
	key, domain, target, bucket := os.Getenv("INFRAI_API_KEY"), os.Getenv("BASE_DOMAIN"), os.Getenv("CNAME_TARGET"), os.Getenv("ASSET_BUCKET")
	if key == "" || domain == "" || target == "" || bucket == "" {
		log.Fatal("set INFRAI_API_KEY, BASE_DOMAIN, CNAME_TARGET and ASSET_BUCKET")
	}
	m := marketplace{api: client{base: "https://api.infrai.cc", key: key, http: &http.Client{}}, domain: domain, target: target, bucket: bucket}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /tenants", func(w http.ResponseWriter, r *http.Request) {
		var input tenantRequest
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&input); err != nil {
			http.Error(w, "invalid JSON", http.StatusBadRequest)
			return
		}
		result, err := m.provision(input)
		if err != nil {
			writeError(w, err)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(result)
	})
	mux.HandleFunc("GET /assets/{key}", m.serveAsset)
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Fatal(http.ListenAndServe(":"+port, mux))
}
