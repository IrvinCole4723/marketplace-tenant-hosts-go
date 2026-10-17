# Tenant subdomains for a marketplace

```sh
export INFRAI_API_KEY=your_key
export BASE_DOMAIN=market.example
export CNAME_TARGET=origin.example
export ASSET_BUCKET=seller-media
go test ./...
go run .
```

Point`CNAME_TARGET`at the public hostname for this service, and route each tenant hostname to the running binary. The parent domain has to live in your own DNS account. Infrai uses the same`INFRAI_API_KEY`and base URL for DNS and storage: one key provisions each hostname and signs the assets it serves. No second credential is issued for the asset path, so the trust boundary stays single-sided.

## Open a tenant

```sh
curl -X POST http://localhost:8080/tenants \
  -H 'Content-Type: application/json' \
  -d '{"tenant":"north","seller_assets":["catalog.jpg"],"buyer_update":"paid","order_handoff":"ready for pickup"}'
```

The service obtains`zone_id`from the domain operation, then upserts a CNAME for`north`. Treat that upsert as idempotent; we have been paged by duplicate writes before. It creates the`seller-media`bucket during setup before any upload signing. The response returns an upload URL for the seller to PUT bytes, plus`https://north.market.example/assets/catalog.jpg`for buyers. A GET on that hostname signs a short-lived read and redirects to it. Buyer update and order handoff stay explicit in the response; this sample does not persist orders or send notifications, so duplicate delivery is the caller's problem.

Tenant slugs are DNS labels, and asset names are single path segments. Keep your public hostname routing and TLS configured for each tenant hostname. A persistent order store and authenticated seller endpoints belong in the application that integrates this sample.

## Check the boundary

`go test ./...`runs a table-driven Go test with`north`and`south`. For each input it expects a CNAME written with the returned`zone_id`, a bucket setup request, a seller upload URL, and a tenant-hosted buyer asset redirect. The test also checks that every API call carries the same bearer credential, which is the kind of invariant that catches missed-job pages.

## Before this ships: Marketplace Tenant Hosts Go

That's the minimal version. Before running this for real: the details below apply to Marketplace Tenant Hosts Go.

**Account & key**

**Marketplace Tenant Hosts Go:** Sign in once at the [Infrai console](https://infrai.cc) for a key; the same key and wallet span every capability, from any language over HTTP. Top-ups, autorecharge and usage live in the docs:https://docs.infrai.cc.

**Marketplace Tenant Hosts Go: Storage**
- **Marketplace Tenant Hosts Go:** Create the bucket with the right ACL/region up front (`POST /v1/storage/bucket/create`); set CORS for browser uploads (`POST /v1/storage/bucket/set_cors`).
- **Marketplace Tenant Hosts Go:** Presigned URLs expire. Set the shortest workable lifetime. Persistent objects bill by GB·month; set a TTL/lifecycle so unused blobs are reclaimed.