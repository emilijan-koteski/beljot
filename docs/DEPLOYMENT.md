# Deployment

Beljot runs on the "Winterfell" VPS under [Dokploy](https://dokploy.monexa.world), next to other projects. Dokploy never clones or builds this repo: GitHub Actions builds the images, pushes them to GHCR and tells Dokploy which tag to run. The database is `beljot_db` on the shared PostgreSQL 16 service; the backend applies its own migrations at start. The data cutover from the old host is in [MIGRATION.md](MIGRATION.md).

## Flow

```
git push master
   └─► GitHub Actions (.github/workflows/deploy.yml)
          ├─ test: lint + vitest + go test                       (.github/workflows/ci.yml)
          ├─ build backend image  ──► ghcr.io/emilijan-koteski/beljot-backend:{sha-<short>,latest}   (only if server/** changed)
          ├─ build frontend image ──► ghcr.io/emilijan-koteski/beljot-frontend:{sha-<short>,latest}  (only if client/** changed)
          └─ deploy: Dokploy API  application.update {dockerImage: …:sha-<short>} → application.deploy
                 └─► Dokploy pulls the image and rolls the Swarm service
                        └─► Traefik: beljot.online/api, /ws, /health → backend:8080; everything else → frontend:8080
                               └─► backend → infrastructure-postgres-qopj51:5432/beljot_db
```

Images are `linux/amd64`. Both Dokploy services are *Application* services with provider **Docker** (registry `ghcr.io`).

## Images

| Service  | Image | Tags pushed per run | Container port | Runs as |
|---|---|---|---|---|
| backend  | `ghcr.io/emilijan-koteski/beljot-backend`  | `sha-<7-char sha>`, `latest` | `8080` (`BELJOT_PORT`) | `beljot`, uid 1000 |
| frontend | `ghcr.io/emilijan-koteski/beljot-frontend` | `sha-<7-char sha>`, `latest` | `8080` | `nginx`, uid 101 |

Dokploy is always pointed at the `sha-…` tag. Redeploying a floating tag such as `latest` re-pulls but does not roll the Swarm task on Dokploy 0.30.x ([Dokploy #5496](https://github.com/Dokploy/dokploy/issues/5496)); pinned tags also make rollbacks explicit.

## GitHub secrets

| Secret | Purpose |
|---|---|
| `DOKPLOY_URL` | `https://dokploy.monexa.world`. |
| `DOKPLOY_API_KEY` | API token (Dokploy → profile → API tokens). Sent as `x-api-key`. |
| `DOKPLOY_BELJOT_BACKEND_APP_ID` | `applicationId` of the backend application (last segment of its URL in Dokploy). |
| `DOKPLOY_BELJOT_FRONTEND_APP_ID` | `applicationId` of the frontend application. |

`GITHUB_TOKEN` (automatic, `packages: write`) pushes to GHCR.

## GitHub variables (not secret)

Settings → Secrets and variables → Actions → *Variables*:

| Variable | Value | Purpose |
|---|---|---|
| `VITE_GOOGLE_CLIENT_ID` | `<id>.apps.googleusercontent.com` | Baked into the frontend image for the Google sign-in button. Must equal the backend's `BELJOT_GOOGLE_CLIENT_ID`; empty hides the button. |

## Backend service — Dokploy Environment tab

| Variable | Required | Secret | Description |
|---|---|---|---|
| `BELJOT_DB_URL` | yes | yes | `postgres://beljot_user:<password>@infrastructure-postgres-qopj51:5432/beljot_db?sslmode=disable&connect_timeout=5`. `sslmode=disable` is intentional (same-host overlay); `connect_timeout=5` makes an unreachable database fail the start within seconds instead of hanging until the health check kills the task. Percent-encode the password if it contains `@ / ? # %` or `:`. |
| `BELJOT_JWT_SECRET` | yes | yes | HMAC key for access and refresh tokens. The process refuses to start with the default value outside development. Reuse the old server's value so nobody is logged out by the move. |
| `BELJOT_ENV` | yes | no | `production`. Any value but `development` makes a missing JWT secret fatal and turns on the production warnings. |
| `BELJOT_APP_BASE_URL` | yes | no | `https://beljot.online`. Origin used in password-reset links. |
| `BELJOT_CORS_ORIGINS` | yes | no | `https://beljot.online`. Comma-separated browser origins allowed by CORS. |
| `BELJOT_GOOGLE_CLIENT_ID` | no | no | Google OAuth client ID (public by design). Empty disables Google sign-in server-side. |
| `BELJOT_SMTP_HOST` | no | no | `smtp.gmail.com`. If host, username or password is empty, password-reset links are logged instead of emailed. |
| `BELJOT_SMTP_PORT` | no | no | Default `587` (STARTTLS). |
| `BELJOT_SMTP_USERNAME` | no | no | The Gmail address. |
| `BELJOT_SMTP_PASSWORD` | no | yes | Gmail App password; spaces are stripped. |
| `BELJOT_SMTP_FROM` | no | no | Sender address. |
| `BELJOT_SMTP_FROM_NAME` | no | no | Default `Beljot.online`. |
| `BELJOT_PORT` | no | no | Listen port; the image sets `8080`. Must match the container port in the Domains tab and the health-check URL. |
| `BELJOT_ACCESS_TOKEN_TTL` | no | no | Go duration, default `15m`. |
| `BELJOT_REFRESH_IDLE_TTL` | no | no | Go duration, default `720h`. |
| `BELJOT_REFRESH_ABSOLUTE_TTL` | no | no | Go duration, default `4320h`. |

The backend applies pending migrations at start (golang-migrate, `schema_migrations` table, files embedded from `server/migrations`). The restored production database is at version 29 (`000029_queue_2026_q3_recalculation`), so the first start logs `database schema up to date version=29` and changes nothing. A failed migration ends the process with `database migration failed: …`; with the Swarm settings below the previous task keeps serving.

The connection pool is capped at 10 open connections so Beljot takes a small share of the shared instance's connection budget (Postgres defaults `max_connections` to 100; the actual value is a server-side setting, not confirmed here).

## Frontend service

No runtime environment variables. Values are baked in at build time by `deploy.yml`:

| Build arg | Value |
|---|---|
| `VITE_GOOGLE_CLIENT_ID` | repository variable `VITE_GOOGLE_CLIENT_ID` |
| `APP_VERSION` | the commit SHA; served as `/version.json` and compiled into the bundle so open tabs notice a new deploy |

nginx (`nginxinc/nginx-unprivileged:1.28-alpine`, `client/nginx.conf`) listens on **8080** as uid 101 and serves the SPA with `index.html` fallback, gzip, `immutable` caching for `/assets/*` and `no-cache` for everything else, so `index.html` and `/version.json` are always revalidated. It sends HSTS, `nosniff`, `X-Frame-Options: SAMEORIGIN` and a referrer policy. TLS is terminated by Traefik.

## Ingress (Dokploy → Domains)

The API is path-routed behind the frontend on the same host, as it was with Caddy. Traefik ranks a `Host && PathPrefix` rule above the bare `Host` rule, so the order of the entries does not matter.

| Application | Host | Path | Container port |
|---|---|---|---|
| beljot-frontend | `beljot.online` | `/` | 8080 |
| beljot-frontend | `www.beljot.online` | `/` | 8080 |
| beljot-backend | `beljot.online` | `/api` | 8080 |
| beljot-backend | `beljot.online` | `/ws` | 8080 |
| beljot-backend | `beljot.online` | `/health` | 8080 |

Strip Path stays **off** everywhere (the backend serves `/api/v1/…` itself). The `www` → apex redirect is the preset *www to non-www* under beljot-frontend → Advanced → Redirects. HTTPS with Let's Encrypt on every entry; Cloudflare proxies the records with SSL mode *Full (strict)* and *Always Use HTTPS* off, so the HTTP-01 challenge reaches Traefik.

WebSockets need nothing extra: the browser opens `wss://beljot.online/ws` from the same origin, and the backend accepts the connection when the `Origin` host equals the request host.

## Health endpoints

| Service | Path | Auth | Behaviour |
|---|---|---|---|
| backend | `GET`/`HEAD` `/healthz` | none | Pings the database with a 2 s timeout: `200 {"status":"ok"}` or `503 {"status":"unavailable"}`. `/health` is the same handler and is the path Traefik exposes publicly. |
| frontend | `GET /health` | none | `200 OK` from nginx. |

Both images declare a Docker `HEALTHCHECK` with busybox `wget`; the same probe goes into Dokploy → Advanced → Cluster Settings → Swarm Settings → Health Check:

backend
```json
{"Test":["CMD","wget","-qO-","http://127.0.0.1:8080/healthz"],"Interval":30000000000,"Timeout":5000000000,"StartPeriod":15000000000,"Retries":3}
```
frontend
```json
{"Test":["CMD","wget","-qO-","http://127.0.0.1:8080/health"],"Interval":30000000000,"Timeout":5000000000,"StartPeriod":5000000000,"Retries":3}
```

The backend handles SIGTERM by closing the WebSocket hub, stopping the season rollover ticker and draining in-flight requests for up to 10 s, which is what Swarm sends on every redeploy.

Client IPs in the request log come from `X-Forwarded-For`, trusting private ranges (Traefik on the overlay) and Cloudflare's published edge ranges (`server/cmd/api/clientip.go`). This is only correct while Traefik's `forwardedHeaders.trustedIPs` is the Cloudflare list.

## Swarm settings and memory

Update Config for both applications (the new task must be healthy before the old one stops; a failing rollout rolls back):
```json
{"Parallelism":1,"Delay":10000000000,"FailureAction":"rollback","Order":"start-first"}
```

Measured idle right after boot against a local database. The limits leave room for a lobby full of matches plus the Go garbage collector's headroom, and are small enough not to matter next to the other projects on the box:

| Container | Idle RSS | Image size | Memory limit (bytes) | Memory reservation (bytes) |
|---|---|---|---|---|
| backend | ~10 MiB | 47 MB | `268435456` (256 MiB) | `67108864` (64 MiB) |
| frontend | ~15 MiB | 105 MB (card and audio assets) | `67108864` (64 MiB) | `16777216` (16 MiB) |

Dokploy takes the values in raw bytes (Advanced → Resources). With `start-first`, a redeploy briefly runs two copies of a service, so count twice the limit as the peak.

## Rolling back

1. Dokploy → application → Provider → Docker: set the image to an earlier `ghcr.io/emilijan-koteski/beljot-<service>:sha-<short>` and click Deploy. Every run's tag is in the Actions log ("Deploy requested for …") and in the GHCR package's tag list.
2. Or `git revert` the commit and push to `master`; the workflow builds and deploys the reverted state.

Rolling back an image does not undo a migration; an older binary against a newer schema only works when the migration was additive.

## Manual deploy

Actions → Deploy → *Run workflow* on `master` builds and deploys both services regardless of what changed; a manual run on any other branch stops after the tests. Merging a PR into `master` deploys the changed halves.

The deploy job returns as soon as Dokploy accepts the request ("Deploy requested for …"); the rollout itself is visible in the application's Deployments tab and in Dozzle.

## Local development

```bash
cp .env.example .env            # local values only
docker compose up -d            # PostgreSQL 16 on localhost:5433
make dev                        # Vite on :5173 proxying /api and /ws to the Go server on :8080
```

The server applies migrations at start, so `make migrate` (the golang-migrate CLI from `mise`) is only needed for `down` migrations or for a database the server does not point at.

The production images build locally with the same Dockerfiles:

```bash
docker build -t beljot-backend:local ./server
docker build -t beljot-frontend:local --build-arg APP_VERSION=local ./client
docker run --rm --network beljot_default -p 127.0.0.1:8082:8080 \
  -e BELJOT_DB_URL='postgres://beljot:beljot_dev_password@postgres:5432/beljot?sslmode=disable' \
  -e BELJOT_JWT_SECRET=local-only beljot-backend:local
docker run --rm -p 127.0.0.1:8081:8080 beljot-frontend:local
```

The frontend container on its own has nothing to send `/api` to; in production Traefik makes that split.
