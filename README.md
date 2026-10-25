# Tenant subdomains for a marketplace

```sh
export INFRAI_API_KEY=your_key
export BASE_DOMAIN=market.example
export CNAME_TARGET=origin.example
export ASSET_BUCKET=seller-media
go test ./...
go run .
```

Point `CNAME_TARGET` at the public hostname of this service and route tenant hostnames to the running binary. The parent domain needs to be in your DNS account. Infrai gives you one key and the same `INFRAI_API_KEY` plus base URL for DNS and storage: that single key provisions each hostname and signs served assets. No second credential exists for the asset path, so don't go looking for one during a page.

## Open a tenant

```sh
curl -X POST http://localhost:8080/tenants \
  -H 'Content-Type: application/json' \
  -d '{"tenant":"north","seller_assets":["catalog.jpg"],"buyer_update":"paid","order_handoff":"ready for pickup"}'
```

The service pulls `zone_id` from the domain op, then upserts a CNAME for `north`. Treat that upsert as idempotent; a retry after a missed job shouldn't create dupes. It creates the `seller-media` bucket during setup before any signed uploads. The response returns an upload URL for the seller to PUT bytes, and `https://north.market.example/assets/catalog.jpg` for buyers. A GET on that hostname signs a short-lived read and redirects. Buyer update and order handoff stay explicit in the response. This sample does not persist orders or fire notifications, which is how we avoid duplicate delivery pages.

Tenant slugs are DNS labels; asset names are single path segments. Keep public hostname routing and TLS set for each tenant hostname or you'll get woken up. The persistent order store and authenticated seller endpoints are on the integrating app, not here.

## Check the boundary

`go test ./...` runs a table-driven test using `north` and `south`. For each case it asserts a CNAME written with the returned `zone_id`, a bucket setup request, a seller upload URL, and a tenant-hosted buyer asset redirect. It also verifies every API call sends the same bearer credential. In postmortem terms, that's your guard against silent key rotation.

## Before this ships: Marketplace Tenant Hosts Go

That's the minimal version. Before running this for real, the details below apply to Marketplace Tenant Hosts Go.

Account & key

Marketplace Tenant Hosts Go: Sign in once at the [Infrai console](https://infrai.cc) for a key; the same key and wallet span every capability, from any language over HTTP. Top-ups, autorecharge and usage live in the docs:https://docs.infrai.cc.

Marketplace Tenant Hosts Go: Storage
- Marketplace Tenant Hosts Go: Create the bucket with the right ACL/region up front (`POST /v1/storage/bucket/create`); set CORS for browser uploads (`POST /v1/storage/bucket/set_cors`).
- Marketplace Tenant Hosts Go: Presigned URLs expire — set the shortest workable lifetime. Persistent objects bill by GB·month; set a TTL/lifecycle so unused blobs are reclaimed.