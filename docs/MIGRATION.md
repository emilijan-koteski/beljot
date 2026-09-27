# Migration runbook: Contabo (compose) → Winterfell (Dokploy)

Written for Emilijan, executed together with the Claude.ai session that owns the server side. Every command says where it runs: **old host** (`deploy@<old-host>`, stack in `/opt/beljot`), **Winterfell** (`root@178.18.243.34`), **laptop**, or a **UI**. Steps marked **irreversible** cannot be undone by the rollback in section 4.

Status as of 2026-09-27: ☑ done · ☐ to do.

## 0. Inputs still needed

| Input | Used in |
|---|---|
| Old host IP and SSH user (`scripts/deploy.sh` assumed user `deploy`) | 2.1–2.3, 4 |
| The old `/opt/beljot/.env` values (identical to the `PROD_ENV_FILE` GitHub secret): `BELJOT_JWT_SECRET`, the `BELJOT_SMTP_*` settings, `BELJOT_GOOGLE_CLIENT_ID` | 1.3 |
| The new `beljot_user` password | 1.3 |
| A maintenance window (any time; about 15 minutes of downtime) | 2 |
| The current Cloudflare records for `beljot.online` and `www` (A/AAAA, proxied) | 2.6, 4 |

## 1. Pre-cutover (no downtime)

### 1.1 DNS (UI: Cloudflare) ☐
The Beljot records are proxied (orange cloud): the public answer is Cloudflare's anycast address and an origin change takes effect within seconds, so no TTL change is needed. If any Beljot record turns out to be DNS-only (grey cloud), set its TTL to 300 now, a day ahead.

Add `next.beljot.online` as an **A** record → `178.18.243.34`, **DNS only** (grey cloud). It is the temporary verification host and is removed in 3.4.

### 1.2 Database ☑
`beljot_db` owned by `beljot_user` exists on `infrastructure-postgres-qopj51` with a fresh restore of the current dump. Nothing more is needed before the trial deploy; the final data comes over in section 2.

### 1.3 Dokploy project and applications (UI: https://dokploy.monexa.world) ☐
1. Project **Beljot**.
2. Application **beljot-backend**: Provider *Docker*, image `ghcr.io/emilijan-koteski/beljot-backend:latest` (the workflow replaces the tag with a pinned `sha-…` on every deploy), registry GHCR (already configured for the panel).
   - Environment tab: every variable in the backend table of [DEPLOYMENT.md](DEPLOYMENT.md), values copied from the old `.env`. Two differ from the old host:
     `BELJOT_DB_URL=postgres://beljot_user:<new password>@infrastructure-postgres-qopj51:5432/beljot_db?sslmode=disable` (percent-encode the password if it contains `@ / ? # % :`), and
     `BELJOT_CORS_ORIGINS=https://beljot.online,https://next.beljot.online` (trimmed back in 3.1).
     `POSTGRES_*`, `IMAGE_TAG`, `ACME_EMAIL` and `CLOUDFLARE_API_TOKEN` from the old `.env` are not used anymore.
   - Advanced → Cluster Settings → Swarm Settings: the Health Check, Update Config and Resources values from DEPLOYMENT.md.
   - Domains: three entries, all host `next.beljot.online`, container port `8080`, HTTPS with Let's Encrypt, Strip Path **off**: paths `/api`, `/ws`, `/health`.
3. Application **beljot-frontend**: same provider, image `ghcr.io/emilijan-koteski/beljot-frontend:latest`; no environment variables; Swarm settings from DEPLOYMENT.md. Domain: host `next.beljot.online`, path `/`, port `8080`, HTTPS.
4. Note each application's `applicationId` (last segment of its URL in the panel).

### 1.4 GitHub (repo → Settings → Secrets and variables → Actions) ☐
Secrets: `DOKPLOY_URL` = `https://dokploy.monexa.world`, `DOKPLOY_API_KEY` (Dokploy → profile → API tokens), `DOKPLOY_BELJOT_BACKEND_APP_ID`, `DOKPLOY_BELJOT_FRONTEND_APP_ID`. The variable `VITE_GOOGLE_CLIENT_ID` already exists. Leave the old `VPS_*`, `PROD_ENV_FILE` and `GHCR_PULL_TOKEN` alone until 3.5.

### 1.5 First deploy, the trial ☐
Merge the PR. The Deploy workflow builds both images, pushes them to GHCR and points both applications at `sha-<short>`. If the secrets were not in place yet, the deploy job fails with `DOKPLOY_URL: parameter null or not set`; add them and run Actions → Deploy → *Run workflow* on `master`.

Watch the backend's first lines in Dozzle (or Dokploy → Logs):
```
database schema up to date version=29
starting server port=8080
```
`version=29` means the restored schema was recognised and nothing was applied. If the process exits instead, Swarm restarts it every few seconds; read the `database migration failed` line before touching anything.

Then, on Winterfell:
```bash
PG=$(docker ps -q --filter name=infrastructure-postgres-qopj51)
docker exec "$PG" psql -U beljot_user -d beljot_db -tAc 'select version, dirty from schema_migrations'   # 29|f
curl -fsS https://next.beljot.online/health; echo          # {"status":"ok"}  (backend, through Traefik)
curl -fsS https://next.beljot.online/version.json; echo    # {"version":"<full commit sha>"}
```

### 1.6 Smoke test on https://next.beljot.online ☐
- Log in with an existing account (the restored data), then log out.
- Google sign-in: add `https://next.beljot.online` to the OAuth client's *Authorized JavaScript origins* for the test, or skip it here and test it on the real hostname right after cutover.
- Forgot password: the email arrives through Gmail. Its link points at `https://beljot.online` (from `BELJOT_APP_BASE_URL`), so it opens the old app until cutover; that is expected.
- Create a room, add bots, play a hand: the WebSocket on `/ws` must connect (lobby stats and the rank banner update live).
- Profile, leaderboard and seasons pages load.
- If the log shows `season recalculation: done`, that is the one-off 2026 Q3 re-score the code queues; it runs once, marks itself done in the database, and is harmless to see again after the final restore.

Beljot keeps no files on disk (no uploads, no generated files), so there is nothing else to verify.

## 2. Cutover (maintenance window)

### 2.1 Freeze the old app (old host) ☐
```bash
ssh deploy@<old-host>
cd /opt/beljot
docker compose -f docker-compose.prod.yml stop web api
```
There is no read-only mode; stopping `api` is the freeze. Visitors get a Cloudflare 52x page from here until 2.6. `postgres` keeps running for the dump.

### 2.2 Final dump (old host) ☐
```bash
set -a; . ./.env; set +a
docker compose -f docker-compose.prod.yml exec -T postgres \
  pg_dump -U "$POSTGRES_USER" -d "$POSTGRES_DB" --clean --if-exists --no-owner --no-privileges \
  | gzip -9 > /var/backups/beljot/beljot-final.sql.gz
ls -la /var/backups/beljot/beljot-final.sql.gz
```
Anything played on the old host after this dump is not carried over; that is why `api` is stopped first.

### 2.3 Copy it ☐
From the laptop (or straight from Winterfell if it has SSH access to the old host):
```bash
scp deploy@<old-host>:/var/backups/beljot/beljot-final.sql.gz .
scp beljot-final.sql.gz root@178.18.243.34:/root/
```

### 2.4 Restore into beljot_db (Winterfell) ☐
Stop the backend first (Dokploy → beljot-backend → Stop) so no connection is open during the load; the frontend can stay up.
```bash
PG=$(docker ps -q --filter name=infrastructure-postgres-qopj51)
zcat /root/beljot-final.sql.gz | docker exec -i "$PG" psql -U beljot_user -d beljot_db -v ON_ERROR_STOP=1 --single-transaction -q
```
The dump was taken with `--clean --if-exists`, so every table is dropped and recreated: the trial data is replaced, not merged. With `--no-owner --no-privileges` there are no references to the old `beljot` role, so `beljot_user` owns everything it creates. The whole load is one transaction: on any error nothing changes and the command exits non-zero.

### 2.5 Verify row counts on both sides ☐
Run the query below on the old host
```bash
docker compose -f docker-compose.prod.yml exec -T postgres psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -f - > /tmp/counts-old.txt
```
and on Winterfell
```bash
docker exec -i "$PG" psql -U beljot_user -d beljot_db -f - > /tmp/counts-new.txt
```
feeding it this on stdin (exact counts for every table):
```sql
SELECT table_name,
       (xpath('/row/c/text()', query_to_xml(format('select count(*) as c from %I', table_name), false, true, '')))[1]::text::int AS rows
FROM information_schema.tables
WHERE table_schema = 'public' AND table_type = 'BASE TABLE'
ORDER BY table_name;
```
`diff` the two files: they must be identical. Then Dokploy → beljot-backend → Start and confirm the log shows `version=29` again.

### 2.6 Switch the domains ☐
1. Dokploy → beljot-frontend → Domains: add `beljot.online` (path `/`) and `www.beljot.online` (path `/`), port `8080`, HTTPS with Let's Encrypt. Advanced → Redirects: preset **www to non-www**.
2. Dokploy → beljot-backend → Domains: add host `beljot.online` three times with paths `/api`, `/ws`, `/health`, port `8080`, HTTPS.
3. Cloudflare → DNS: point every A/AAAA record for `beljot.online` and `www` at `178.18.243.34`, still proxied. SSL/TLS → *Full (strict)*; Edge Certificates → *Always Use HTTPS* off (Traefik does the redirect; the HTTP path must stay open for the Let's Encrypt challenge).
4. Wait for the certificates. Until Traefik has them, Cloudflare answers **526** on the new hostnames; on Winterfell `docker service logs dokploy-traefik --since 5m 2>&1 | grep -i acme` shows progress. Usually under a minute:
   ```bash
   curl -fsS https://beljot.online/health; echo
   curl -fsS https://beljot.online/version.json; echo
   curl -sSI https://www.beljot.online/ | grep -i '^location'      # https://beljot.online/
   ```
5. A real login and a bot match on https://beljot.online.

Cutover is done; players can play.

## 3. Post-cutover

1. ☐ Dokploy → beljot-backend → Environment: `BELJOT_CORS_ORIGINS=https://beljot.online`, then Redeploy.
2. ☐ Google Cloud console → the OAuth client: remove `https://next.beljot.online` if it was added in 1.6. Nothing else references a hostname: email is Gmail SMTP, there are no webhooks and no object storage.
3. ☐ UptimeRobot: keep or add monitors for `https://beljot.online/health` (backend, through Traefik) and `https://beljot.online/` (frontend).
4. ☐ Remove the `next.beljot.online` domains from both applications and delete its DNS record.
5. ☐ GitHub: delete the secrets `VPS_HOST`, `VPS_PORT`, `VPS_USER`, `VPS_SSH_KEY`, `PROD_ENV_FILE`, `GHCR_PULL_TOKEN` and the `production` environment; the new workflow uses none of them.
6. ☐ Old host: copy `/var/backups/beljot/*.sql.gz` (the last seven nightly dumps plus `beljot-final.sql.gz`) off the machine, for example `scp 'deploy@<old-host>:/var/backups/beljot/*.sql.gz' ~/beljot-old-backups/`. Winterfell's nightly job has covered `beljot_db` since it was created.
7. ☐ **Irreversible.** Cloudflare → API Tokens: revoke the token Caddy used for DNS challenges (it carries `Zone:DNS:Edit` on the zone). Do this only once the rollback window (section 4) is over.
8. ☐ **Irreversible.** Decommission the Contabo VPS after a quiet week. The snapshot taken before this migration is the only way back afterwards.

## 4. Rollback

Valid until 3.8. Matches played on Winterfell after 2.6 are lost by rolling back, so decide within the first hour of the cutover.

1. Cloudflare → DNS: A/AAAA records for `beljot.online` and `www` back to the old host's IP (still proxied).
2. Old host: `cd /opt/beljot && docker compose -f docker-compose.prod.yml start api web`.
3. `curl -fsS https://beljot.online/version.json` shows the old build's SHA. The Dokploy applications can stay running; nothing routes to them.
