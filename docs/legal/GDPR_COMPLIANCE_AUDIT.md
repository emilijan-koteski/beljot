# Beljot GDPR Compliance Audit

**Audit date:** 2026-11-27 (run 1)
**Application:** Beljot, an online multiplayer Belot card game
**Domain:** beljot.online
**Codebase:** `master` @ `fdb87af1`
**Auditor:** `gdpr-legal-expert` agent (`.claude/agents/gdpr-legal-expert.md`), with four parallel scanners (server data, client, infrastructure and processors, rights and retention), an adversarial verifier and a gap hunter.
**Standard:** bare-minimum, pragmatic GDPR compliance. Users must be clearly informed, the owner must be able to erase a user completely on request, export can be minimal, and there is no consent theatre.

> **Disclaimer.** This audit is expert guidance, not legal advice. Have the final Privacy Policy and Terms reviewed by a lawyer familiar with the North Macedonian LPDP (Official Gazette 42/2020) and the GDPR before publication.

---

## 1. Executive summary

**Verdict: NOT READY for public launch. 3 CRITICAL and 14 WARNING findings are open.**

The core data handling is in good shape. Beljot collects little:

- no IP addresses, devices or locations in the database
- chat is never stored
- no analytics or tracking
- virtual coins only, with no payments
- bcrypt passwords and hashed tokens
- a token in memory and an httpOnly cookie, with only the user id in the JWT
- re-encoded avatars

All browser storage is strictly necessary, so **no cookie banner is needed** once Google Fonts are self-hosted.

The gaps are mostly about **transparency, erasure and housekeeping**, not about over-collection:

| # | Severity | Finding |
|---|---|---|
| G-01 | CRITICAL | Google Fonts load from Google on every page, which sends every visitor's IP to Google |
| G-02 | CRITICAL | Privacy Policy and Terms are "testing purposes only" placeholders, yet signup requires agreeing to them |
| G-03 | CRITICAL | No way to erase a user; a plain `DELETE` fails on 8 foreign keys, and the only delete is a soft delete that keeps the email and password hash |
| G-04 | ~~WARNING~~ closed | App-password-shaped test string; owner confirmed it is not real |
| G-05 | WARNING | Real player usernames and ranking history in committed planning docs, and a real username in a landing screenshot (public repo) |
| G-06 | WARNING → disclose | Backups: 14 days on the same VPS (DB and Garage), no off-site copy; old VPS gone; local dumps out of scope by owner decision |
| G-07 | WARNING | Every request is logged with client IP and user-id URIs, with no defined log retention |
| G-08 | ~~WARNING~~ closed | Email moved from consumer Gmail to Resend (EU region) on 2026-11-28 |
| G-09 | WARNING | Cloudflare proxies all traffic but is not documented as a processor |
| G-10 | WARNING | Google Identity Services loads on page view of /login, /register and /settings, not on click |
| G-11 | WARNING | Google sign-up turns the person's real Google name into their public username |
| G-12 | WARNING | No minimum-age statement |
| G-13 | WARNING | The honor score is profiling and is disclosed nowhere |
| G-14 | WARNING | Refresh tokens and reset tokens are never purged |
| G-15 | WARNING | No retention rule for matches, rooms and season results |
| G-16 | WARNING | No data export path (Art. 15/20) |
| G-17 | WARNING (owner decision) | No EU representative (Art. 27) for a non-EU controller targeting EU users |

The full list, including 12 RECOMMENDATIONs and the OK set, is in §11.

---

## 2. Data map

### 2.1 Personal data stored (PostgreSQL, 14 tables, schema version 38)

| Table | Personal data | Source | Purpose | On user delete |
|---|---|---|---|---|
| `users` | email, username (public), bcrypt password hash, preferences (language, deck, theme, audio), `allow_non_friend_invites`, coin balance, `last_login_at`, login streak, XP, `username_changed_at`, honor fields and score, `avatar_key`, timestamps, `deleted_at` | user / derived | account, login, game, reputation | soft delete only |
| `user_identities` | Google `sub`, Google account email | Google | Google sign-in and linking | CASCADE |
| `refresh_tokens` | token hash, family, expiry, rotated and revoked times (a login timeline; no IP or user agent) | system | sessions | CASCADE |
| `password_reset_tokens` | token hash, expiry, used_at | system | password reset | **NO ACTION** |
| `friendships` | requester, recipient, pending or accepted | user | friends | CASCADE |
| `rooms` | **free-text room name**, owner, rules, room password hash, min honor | user | game rooms | **NO ACTION** (`owner_id`) |
| `room_players` | user, seat, team | system | seating | **NO ACTION** |
| `matches` | 4 player ids, scores, winner, `abandoned_by`, `surrendered_by`, coin deltas, times | derived | match history, stats, honor | **NO ACTION** (6 FKs) |
| `hand_results` | per-hand team scores (linked only via match) | derived | scoring | via matches |
| `player_seasons` | season points, tier, division, games played, completed and won | derived | leaderboard | CASCADE |

**S3 (Garage, self-hosted, bucket `beljot-public`):** avatars at `avatars/<random-uuid>/{128,256}.webp`. They are re-encoded, so EXIF and GPS data is stripped. The bucket is publicly readable but cannot be listed. Avatars are served through Cloudflare with a 1-year immutable cache.

### 2.2 Data NOT collected (verified)

- No IP, user-agent, device or location column anywhere in the database.
- No chat, whisper, emote or invite storage. These live in memory only and are never logged.
- No real name, date of birth, phone, address or payment data. No real-money purchase, payment provider or cash-out exists anywhere.
- No analytics, error tracking, ad or marketing scripts. No marketing email and no tracking pixels.
- No JSON blobs. Live match state is in memory.
- From Google, `picture` is not used. Only `sub`, `email`, `email_verified` and `name` are read; `name` only seeds the username.

### 2.3 Data visible to other users (all behind login)

Visible to other users:
- username
- avatar
- join date
- XP and level
- honor score, including completed and abandoned counts
- match history and stats
- leaderboard rank
- online status, which friends see
- with `allow_non_friend_invites` on (the default), "invitable" status, which room owners see

Email is never returned for another user. User search matches usernames only. `robots.txt` and the sitemap keep player pages out of search engines.

### 2.4 Browser storage (all strictly necessary, so no consent needed)

| Key | Type | Purpose |
|---|---|---|
| `refresh_token` | cookie, httpOnly, Secure, SameSite=Strict, path `/api/v1/auth` | keep the user logged in |
| `__cf_bm` | cookie set by Cloudflare Bot Fight Mode (may appear) | bot protection; strictly necessary per Cloudflare |
| `beljot.lang`, `beljot.theme` | localStorage | UI language and theme |
| `beljot.theme.picked`, `beljot:preload-error-reload`, `beljot:version-reloaded-for`, `beljot:lobby-idx` | sessionStorage | UI state and reload guards |

The access token lives in memory only. There is no IndexedDB, Cache API or service worker. Google sets its own cookies inside the Google sign-in iframe (see G-10).

---

## 3. Processors and connections

| Connection | Use | Personal data | Role | Location and transfer | Action |
|---|---|---|---|---|---|
| **Contabo GmbH** | VPS hosting everything: app, Postgres, Garage, logs, backups | everything | processor | Germany (owner to confirm the data centre) | accept the Contabo DPA in the customer panel |
| **Contabo** (old VPS) | previous host with 7-day dumps and a pre-migration snapshot | full DB copies | processor | Germany | confirm decommissioned and snapshot deleted (G-06) |
| **Cloudflare, Inc.** | DNS, proxy, TLS for beljot.online, www and assets.beljot.online | all traffic at the edge: IP, requests, credentials in transit, cached avatars | processor | US, global edge; EU-US DPF plus SCCs | name it in the policy (G-09) |
| **Resend (Plus Five Five, Inc.)** | password-reset email over SMTP, EU sending region (since 2026-11-28) | recipient email, reset link | processor (DPA in Resend's terms) | US company, EU region; DPF plus SCCs | list in policy |
| ~~Google: Gmail SMTP~~ | replaced by Resend on 2026-11-28 | old Sent-folder copies only | n/a | n/a | delete old Sent copies |
| **Google: Identity Services** | "Sign in with Google" button and script | IP, user agent, Google cookies | independent controller | US; DPF | disclose; load on click (G-10) |
| **Google: Fonts** | web fonts | visitor IP and user agent on every page | no legal basis | US | **remove** (G-01) |
| **Proton AG** (`pm.me`) | public contact and data-request inbox | whatever users email | processor for the owner's mailbox | Switzerland (adequacy decision) | name it as the rights channel |
| Dokploy, Traefik, Garage, Beszel, Dozzle | self-hosted on the same VPS | logs and data stay on the VPS | not third parties | Germany | protect Dozzle and Dokploy access |
| GitHub Actions, GHCR | CI and images | none (synthetic test DB) | n/a | US | none |
| UptimeRobot, Let's Encrypt, Namecheap | uptime, TLS certificates, registrar | none (owner data only) | n/a | n/a | none |
| Buy Me a Coffee, Instagram, Facebook, YouTube, LinkedIn | plain outbound links | none until clicked | n/a | n/a | none |
| ElevenLabs (`marketing/`, local) | voice-over for ads | none (made-up demo players) | n/a | n/a | none |

---

## 4. Legal bases

| Processing | Basis |
|---|---|
| Account, login, gameplay, match history, leaderboard, friends, avatar, preferences | Art. 6(1)(b), performance of the contract (the Terms) |
| Honor score (abandonment reputation, room gating) | Art. 6(1)(b) as part of the game's rules; disclosed as profiling (Art. 4(4)), with the Art. 21 objection right. Art. 22 does not apply: no legal or similarly significant effect. |
| Server request logs (IP, URI), security, rate limiting | Art. 6(1)(f), legitimate interest in security and debugging |
| Backups | Art. 6(1)(f), legitimate interest in availability (Art. 32) |
| Password-reset email | Art. 6(1)(b) |
| Replies to contact and data-request emails | Art. 6(1)(c), legal obligation (Arts. 12-22) |

No processing relies on consent, so no consent records are legally required. The "I agree" checkbox should change wording (G-18).

---

## 5. Data-subject rights status

| Right | Status | Minimum fix |
|---|---|---|
| Information (Arts. 13/14) | **missing**: placeholder pages | G-02 |
| Access (Art. 15) | partial: profile, career, matches, seasons, friends and identities endpoints already show most data | G-16: `export_user.sql` producing JSON, sent by email on request |
| Rectification (Art. 16) | self-service: username (30-day cooldown), avatar, preferences, Google link and unlink, password via reset. **Email change: none.** | handle email change manually; document it (G-22) |
| Erasure (Art. 17) | **missing** | G-03 |
| Restriction (Art. 18) | none; rare in practice | handle manually on request; mention it in the policy |
| Portability (Art. 20) | none | same JSON as access (G-16) |
| Objection (Art. 21) | applies to the legitimate-interest items and the honor profiling | policy text; handle manually |
| Complaint | not stated | name АЗЛП (North Macedonia) and the right to complain to the user's own EU supervisory authority |

Response deadline: one month (Art. 12(3)). Channel: a named email address in the policy.

---

## 6. Retention (current to proposed)

| Data | Today | Proposed (state it in the policy) |
|---|---|---|
| Account and profile | until erasure | until the user asks for deletion; optional inactive-account cleanup after N years (R-11) |
| Refresh tokens | **forever** (3,096 rows for 156 users locally in about a day) | daily purge of revoked or expired rows older than 30 days (G-14) |
| Password-reset tokens | linger until the next request | purge used or expired rows after 24 h (G-14) |
| Matches, hand results, season rows | forever | life of the service; on erasure, anonymised (tombstone id, no name) (G-15) |
| Rooms (free-text names) | forever (851 rows locally) | renamed on erasure; optional purge of completed rooms with no matches |
| Server and proxy logs (app, nginx, Traefik) | 14 days (owner, 2026-11-28) | state 14 days (G-07) |
| Backups (DB and avatars) | 14 days, nightly, same VPS (owner, 2026-11-28) | state 14 days; erasure reaches backups by rotation; recommended: encrypt and keep an off-site copy (G-06) |
| Resend sending logs | per Resend's retention | state it in the policy; old Gmail Sent copies deleted by the owner |
| Avatars | until replaced or removed (old pair deleted on replace) | deleted on erasure, plus a Cloudflare purge |

---

## 7. Erasure procedure (draft for G-03)

Anonymise in place, not hard delete. The `users.id` row stays as an inert tombstone so other players' match history, wins and honor stay correct. Every personal column is scrubbed and every satellite row deleted. No schema change is needed. Existing `deleted_at IS NULL` filters already hide the tombstone from the leaderboard, friends lists and lookups, and other players see an empty name for that seat.

> **Trap:** never set `matches.abandoned_by` or `surrendered_by` to NULL. `match/gorm_repo.go:70-78` reads NULL as an abandonment by every seat, which would corrupt the opponents' stats.

```sql
-- 0. Preconditions (read-only): abort while the user sits in a waiting or playing room.
SELECT r.id, r.status FROM room_players rp JOIN rooms r ON r.id = rp.room_id
 WHERE rp.user_id = :uid AND r.status IN ('waiting','playing');
SELECT avatar_key FROM users WHERE id = :uid;   -- capture for the S3 delete

BEGIN;
DELETE FROM refresh_tokens        WHERE user_id = :uid;
DELETE FROM password_reset_tokens WHERE user_id = :uid;
DELETE FROM user_identities       WHERE user_id = :uid;
DELETE FROM friendships           WHERE user_id = :uid OR friend_id = :uid;
DELETE FROM player_seasons        WHERE user_id = :uid;
DELETE FROM room_players          WHERE user_id = :uid;
UPDATE rooms SET name = 'room-' || id, password_hash = NULL WHERE owner_id = :uid;
UPDATE users SET
  email = 'erased+' || id || '@invalid', username = 'erased_' || id, password_hash = '',
  avatar_key = NULL, language_preference = 'en', card_deck_preference = 'french',
  theme_preference = 'brass', sound_enabled = true, music_enabled = true,
  sound_volume = 70, music_volume = 70, allow_non_friend_invites = true,
  wallet_balance = 0, last_login_at = NULL, login_streak_days = 0, total_xp = 0,
  username_changed_at = NULL, honor_completed_weight = 0, honor_abandoned_weight = 0,
  honor_decayed_at = NULL, honor_completed_total = 0, honor_abandoned_total = 0,
  honor_score = 80, created_at = now(), updated_at = now(), deleted_at = now()
WHERE id = :uid;
-- verify: every satellite count for :uid must be 0
COMMIT;
```

Outside the database:
1. **S3:** delete `<avatar_key>/256.webp` and `/128.webp`. Search the logs for `avatar: best-effort delete failed … userID=<uid>` to find orphans from earlier failed deletes.
2. **Cloudflare:** purge both avatar URLs from the cache.
3. **Gmail:** delete reset emails to that address from Sent.
4. **Proton:** delete or archive the request thread after the confirmation reply, keeping a minimal record that the request was handled.
5. **Logs and backups:** they age out on rotation (state this in the policy and in the reply).
6. **Live session:** the access JWT stays valid for up to 15 minutes, and an open WebSocket survives until reconnect. Refresh is already safe. Acceptable at bare minimum; optionally restart or add a kick hook.
7. **Reply** to the user within one month confirming the erasure.

Also make the SSO-signup compensation a hard delete (`Unscoped().Delete`, `server/internal/auth/sso_handler.go:105`) so it never leaves a soft-deleted row holding a Google email.

Implement it as a version-controlled script (`server/scripts/erase_user.sql` or a small `cmd/eraseuser`) plus a runbook. A self-service "Delete account" button is optional.

---

## 8. Security (Art. 32)

**OK:**
- bcrypt cost 10; refresh and reset tokens stored as SHA-256 hashes
- refresh-token rotation with family revocation
- JWT holds only `sub`; JWT secret enforced at start
- TLS through Cloudflare and Let's Encrypt
- HSTS, nosniff, X-Frame-Options and Referrer-Policy set
- CORS and WebSocket origin allowlist
- database not reachable from the internet
- every `/users/:id` write checks that the caller owns the account
- no `.env` file or OAuth client secret ever committed (`docs/temp/` is gitignored and was never committed)

**Gaps:** G-04, G-06, G-07, G-19, G-20, G-24, G-26, G-27. G-04 is the only urgent one.

---

## 9. Owner questions (cannot be verified from the repo)

**Answers received 2026-11-27/28:**

| # | Answer | Effect |
|---|---|---|
| 1 | Contabo region Germany (Munich or Frankfurt); Contabo DPA accepted | Policy: "Contabo GmbH, Germany". G-30 low-risk item stays a recommendation. |
| 2 | Old Contabo VPS, snapshot and backups gone | Old-host part of G-06 closed. |
| 3 | Nightly backups of the DB **and** Garage on the current VPS, kept 14 days, no off-site copy | G-06 resolved for disclosure: the policy states 14 days and that erasure reaches backups when they rotate out. Encryption at rest and an off-site copy remain RECOMMENDATIONS (Art. 32). |
| 4 | SMTP is a consumer Google account | G-08 confirmed. Decision: move to Resend over SMTP, as in Monexa. |
| 5 | Test string is not a real password | G-04 closed (false positive). |
| 6 | Owner asked how to check | Public checks on 2026-11-28 (below). Plan and Bot Fight Mode still need the dashboard. |
| 7 | Docker log rotation on, Traefik access log on, Dozzle behind a login | Traefik also logs client IPs, so add it to the policy's log section with the rotation period. Remaining G-07 work is the code changes and stating the retention number. |
| 8 | Proton plan is personal | Proton (Switzerland, adequacy decision) stays disclosed as the contact mailbox. No Art. 28 DPA on a personal plan; acceptable at bare minimum. |
| 9 | 0 soft-deleted users in production | G-03 needs no backfill of old soft-deleted rows; the `Unscoped()` fix is preventive. |
| 10 | All old deploy secrets revoked | Closed. |
| 11 | Both players shown in the landing screenshots agreed | Screenshot part of G-05 closed; real names in `_bmad-output` remain. |
| 12 | Controller is the owner personally; no EU representative | §10 controller block as in Monexa; G-17 accepted risk. |

**Public checks 2026-11-28:**
- `beljot.online` and `assets.beljot.online` resolve to Cloudflare and answer with `server: cloudflare` and `cf-ray`, so they are proxied.
- `storage.monexa.world` resolves straight to the VPS with no Cloudflare headers, so it is **not proxied**. Avatars still reach browsers through the proxied `assets.beljot.online`.
- The page HTML has no Cloudflare Web Analytics beacon (`cloudflareinsights`), so it is off. The only third-party reference in the HTML is Google Fonts (G-01).
- A plain request sets no `__cf_bm` cookie.

**Cloudflare dashboard (owner, 2026-11-28):**
- Free plan, so there is no Logpush.
- **Bot Fight Mode is ON.** It may set the `__cf_bm` cookie; Cloudflare classes it as strictly necessary, so it only needs a disclosure line in the cookie table.
- The DPA is in place.
- G-09 is resolved for disclosure.

**Log retention (owner, 2026-11-28):** Docker and Traefik logs are kept 14 days. G-07 is resolved for disclosure; the code items remain.

1. Contabo data centre region, and has the Contabo DPA been accepted?
2. Is the old Contabo VPS decommissioned, with its snapshot and `/var/backups/beljot` deleted? Are any local production dumps still kept?
3. Nightly backup job: storage path, retention, encryption, off-site copy? Is Garage backed up?
4. SMTP: consumer @gmail.com or Google Workspace? Which From address? Is the Sent folder kept?
5. Is the App-password-shaped test string (`server/internal/config/config_test.go:18`) now, or was it ever, a real App password?
6. Cloudflare: which plan? Are Web Analytics/RUM, Bot Fight Mode or Logpush on? Is `storage.monexa.world` proxied?
7. Docker log rotation in `/etc/docker/daemon.json`? Traefik access log in Dokploy? Is Dozzle behind authentication?
8. Proton plan for the contact address: personal or business?
9. Production check: `SELECT count(*) FROM users WHERE deleted_at IS NOT NULL;` (expected about 0).
10. Old secrets from the previous host (VPS keys, env file, GHCR pull token, Caddy Cloudflare token): revoked?
11. Are the two players named in the landing screenshots real accounts? Do you have their OK?
12. Controller identity for the policy: your full name and country, or a registered business?

---

## 10. Prerequisites for the Privacy Policy and Terms

- **Controller:** Emilijan Koteski, individual developer, with the postal address and the `pm.me` contact email exactly as in Monexa's Privacy Policy §1. No DPO, giving the Art. 37 reasoning used in Monexa.
- **Supervisory authorities:** АЗЛП (Agency for Personal Data Protection, North Macedonia), plus the user's right to complain to their own EU authority.
- **Art. 27:** an EU representative decision (G-17), and the matching Serbian ZZPL representative question for rs users.
- **Governing law** and jurisdiction for the Terms.
- **Minimum age 16** (HR and DE 16, RS 15, MK 14; a single 16+ bar covers all).
- **Terms content:**
  - virtual coins have no monetary value, cannot be bought or cashed out
  - chat and username conduct rules
  - a contact for reporting content (DSA Arts. 11, 12, 16)
  - account suspension
  - service provided "as is"
  - honor system explained
- **Policy content:**
  - every row of §2 and §3
  - the legal bases in §4
  - the retention periods in §6
  - the rights in §5 with the one-month response time
  - Google sign-in as an independent controller
  - what other users can see (§2.3)
  - the storage table (§2.4)
  - "no cookies beyond strictly necessary"
- **"Last updated" date and version** on both documents. All four locales (en, sr, hr, mk), using the terminology memory.

---

## 11. Findings

Severity is after verification. Source IDs from the scanner reports are in brackets.

### CRITICAL

**G-01 Remote Google Fonts on every page.** Art. 6(1), Arts. 44 ff.; LG München I 3 O 17493/20. [C-01, I-01]
- Evidence: `client/index.html:16-19` (preconnect plus `fonts.googleapis.com/css2?family=Cormorant+Garamond…Inter…JetBrains+Mono`). The link is also in the built `dist/index.html` that the production image ships. Playwright saw 4 Google requests on the landing page before any interaction.
- Fix: self-host the three families (`@fontsource/*` or files in `client/public/fonts`) and remove the links.

**G-02 Privacy Policy and Terms are placeholders, with no controller identity and no rights channel.** Arts. 12, 13. [C-02, R-02, C-08]
- Evidence: `client/src/features/legal/PrivacyPage.tsx`, `TermsPage.tsx`, and `legal.*` in all four locales: "Work in progress… testing purposes only". `RegisterPage.tsx:222-268` and `:288-312` and `LoginPage.tsx:198-222` require agreeing to them.
- Fix: write both documents after this audit is green (§10).

**G-03 No erasure mechanism.** Art. 17. [R-01, C-04, G-S06, G-S07, R-09, R-03, I-04]
- Evidence: no delete route in `server/cmd/api/main.go`. `user/gorm_repo.go:41-51` is a soft delete that keeps email and password hash. `pg_constraint` shows 8 NO ACTION FKs to `users` (`rooms.owner_id`, `room_players.user_id`, `matches.player1-4_id`, `abandoned_by`, `surrendered_by`, `password_reset_tokens.user_id`).
- Fix: §7.

### WARNING

**G-04 A Gmail App password-shaped string in a public repo.** Art. 32. [I-10, X-11]
- Evidence: `server/internal/config/config_test.go:18`: an App-password-shaped string, added in `b82c9f6f`. The GitHub repo is PUBLIC.
- Fix: if it is or was real, **revoke it in the Google account now**; removing it from the code does not remove it from history. Replace the fixture with an obviously fake value.
- **Closed 2026-11-27: FALSE POSITIVE.** The owner confirmed it is not a real password. Optional: swap it for an obviously fake value such as `abcd efgh ijkl mnop` so future secret scans don't flag it again.

**G-05 Real player data in the public repo.** Arts. 5(1)(b), 5(1)(f). [X-01, X-02]
- Evidence: `_bmad-output/archive/implementation-artifacts/spec-q3-season-recalculation.md:34,96` (real usernames with their season points and rank before and after the recalculation), `deferred-work.md:1236`, and real hands attributed to a real player in rule specs (7 hits). The landing screenshots `client/public/landing/lobby-desktop-*.webp` show a real username as online.
- Fix: replace real usernames in `_bmad-output` with placeholders, or make the repo private. Get consent for, or retake, the screenshots. Rewriting history is optional; the data is low-sensitivity.

**G-06 Production dumps and backups.** Arts. 5(1)(e), 5(1)(f), 32. [I-05, I-06, X-09]
- Evidence: plain-text production dumps (`.sql` and `.sql.gz`) in the owner's local Beljot briefs folder, containing users, emails, password hashes and identities. The old-host dumps and snapshot are possibly still present. Earlier production copies were restored into the local DBs `beljot_q3`, `beljot_q3_recalc` and `beljot_e13`; they are gone from the container now, so check other machines. Winterfell nightly backup retention and encryption are unknown.
- Fix: delete the local dumps (or encrypt them, e.g. `age`/`gpg`). Confirm the old VPS and snapshot are deleted. Set backup retention (7 to 30 days), encrypt backups and state the period in the policy.
- **Update 2026-11-27:**
  - The old VPS is deleted (owner).
  - The owner believes no backups exist, but the handoff brief documents a nightly host-side dump of every database. Check the VPS (crontab, systemd timers, Dokploy backup settings, the dump directory) and either state its retention in the policy or confirm none exists.
  - With truly no backups, nothing needs disclosing, but Art. 32 availability becomes a RECOMMENDATION to add a short-retention encrypted backup.
  - The two local migration dumps from 2026-09-27 are on the owner's personal drive. **Owner decision: out of scope, left as is** (the controller's own storage). Re-audits do not flag them again.

**G-07 Logs: IP plus user-linked URIs, no retention.** Arts. 5(1)(c), 5(1)(e), 13. [G-S01, I-07, R-12, G-S03, G-S05, I-08, I-09]
- Evidence:
  - The request logger at `server/cmd/api/main.go:133-155` is unconditional and logs IP, method, URI and status.
  - The nginx default access log records IP, user agent and `/reset-password?token=…`.
  - `auth/password_reset.go:117` logs the email address on SMTP failure.
  - The GORM logger (`main.go:54-62`) prints slow or failed SQL with values filled in.
  - The Docker json-file driver never rotates by default.
- Fix: set Docker log rotation (`max-size`/`max-file`, or a Dokploy setting) to about 14 days. Turn off or trim nginx access logs. Drop the email from the log line. Set `ParameterizedQueries: true`. Disclose IP logging, its purpose and retention.

**G-08 Gmail for transactional email.** Art. 28. [I-02]
- Evidence: `smtp.gmail.com:587` (`server/internal/mailer/smtp.go`, `main.go:91-98`). Google's Cloud Data Processing Addendum covers Workspace and Cloud only.
- Fix (owner choice):
  - (a) Google Workspace, which has a DPA, or
  - (b) an EU transactional provider with a free tier and DPA (Brevo, Mailjet, Scaleway TEM), or
  - (c) stay on Gmail, disclose it, and accept the residual risk. That is acceptable for a hobby project at bare minimum but not clean.

  Either way, keep the Sent folder clean.
- **Decision 2026-11-27:** the account is consumer Gmail. Move to **Resend**, the processor Monexa already uses, which has a DPA and can send from an EU region.
  - It is a config-only change: the existing SMTP mailer works with `smtp.resend.com:587`, username `resend` and an API key as the password, through `BELJOT_SMTP_*`.
  - It needs DNS records for `beljot.online` added in Cloudflare and Resend's DPA accepted.
  - Afterwards, revoke the Gmail App password and delete the reset emails from its Sent folder.
- **Closed 2026-11-28:** production now sends through Resend (EU region, `noreply@beljot.online`), tested end to end by the owner. The policy lists Resend as the email processor.

**G-09 Cloudflare not documented as a processor.** Arts. 13(1)(e)-(f), 28, 44 ff. [I-03]
- Fix: confirm that the self-serve DPA is part of the account terms. List Cloudflare (US, DPF plus SCCs) in the policy. Check whether Web Analytics or Bot Fight Mode is on and disclose it.

**G-10 Google Identity Services loads on page view.** Art. 13, ePrivacy 5(3). [C-03, I-16]
- Evidence: `client/src/shared/lib/googleIdentity.ts:35,50-86` loads the GIS script and iframe on mount of /login, /register and /settings. The earlier `g_state` cookie claim is unproven, because `prompt()`/One Tap is never called.
- Fix: preferred is a local "Sign in with Google" button that loads GIS only on click (`public/google.svg` exists). The minimum is to disclose it in the policy.

**G-11 Google real name becomes the public username.** Arts. 5(1)(c), 25(2). [G-S04]
- Evidence: `auth/sso_handler.go:254-268` builds it from the Google `name`, or else the email local part. The first rename is free and immediate.
- Fix: generate a neutral username (e.g. `player_ab12cd`) or show a pick-your-username step on first Google sign-in. The minimum is to disclose it.

**G-12 No minimum age.** Art. 8 and LPDP; contractual capacity. [C-05, R-13]
- Fix: extend the signup checkbox to "I am 16 or older and agree to the Terms…", include the Google path, and state 16+ in the Terms. No birthdate field.

**G-13 Honor score is undisclosed profiling.** Arts. 4(4), 13(2)(f), 21. [G-S10, R-14]
- Fix: policy text explaining the inputs (completed and abandoned matches, 90-day decay), that it is shown publicly, that it gates rooms whose owner set a minimum, and the right to object. No code change.

**G-14 Tokens never purged.** Art. 5(1)(e). [G-S08, R-07, R-08]
- Evidence: there is no delete path for `refresh_tokens` or `password_reset_tokens`. Locally there are 3,096 refresh rows for 156 users, up to 221 for one user.
- Fix: a daily ticker in the server deleting revoked or expired refresh tokens older than 30 days and used or expired reset tokens older than 24 h.

**G-15 No retention rule for game records and rooms.** Arts. 5(1)(e), 13(2)(a). [R-10, G-S09]
- Fix: policy states "kept for the life of the service; anonymised on account erasure". Optionally purge old completed rooms with no matches.

**G-16 No data export.** Arts. 15, 20. [R-04, C-04]
- Fix: a read-only `server/scripts/export_user.sql` producing one JSON document (profile, identities, friends, seasons, matches), sent by email on request. A self-service button is optional.

**G-17 EU representative (Art. 27).** Owner decision. [X-03]
- The controller is outside the EU and continuously offers the service to EU (hr, de) users, so the Art. 27(2) "occasional" exemption is hard to claim. Options:
  - appoint a low-cost EU representative service
  - accept and document the risk, which is common for hobby projects and rarely enforced
  - operate through an EU entity later

  Serbia's ZZPL has an analogous representative duty.
- **Decision 2026-11-27: ACCEPTED RISK.** No representative is appointed, consistent with Monexa. The policy names the controller's direct contact details and the users' right to complain to their own EU authority. Not a launch blocker.

### RECOMMENDATION

| ID | Finding | Fix | Source |
|---|---|---|---|
| G-18 | Checkbox says "agree to the Privacy Policy"; acceptance not stored server-side | wording: "I agree to the Terms and have read the Privacy Policy"; optional `terms_accepted_at`/`terms_version` | C-06 |
| G-19 | LogMailer fallback logs reset links if SMTP is unset in any environment | make missing SMTP fatal outside development | G-S02, I-12 |
| G-20 | No rate limit on login, register, forgot and reset (forgot can spam mail) | Echo rate limiter on the auth group, or a free Cloudflare rule | I-13 |
| G-21 | Logged-in users have no link to Privacy or Terms | footer or settings links | C-07 |
| G-22 | No email change; no logged-in password change | handle manually; document it in the policy | R-05 |
| G-23 | `allow_non_friend_invites` defaults to TRUE; room owners see which strangers are online | default to false, or disclose | X-06 |
| G-24 | Account enumeration: `EMAIL_TAKEN` on register; login timing differs | dummy bcrypt compare on unknown email | X-07 |
| G-25 | No report, block or notice mechanism for user content | Terms name a report email; full DSA tooling not needed now | X-08 |
| G-26 | No `middleware.Recover()`; panics log the IP and stack to stderr | add Echo Recover | X-10 |
| G-27 | No Content-Security-Policy | add one after G-01 and G-10 | I-14 |
| G-28 | No breach runbook or breach register (Arts. 33/34) | half-page runbook: assess, 72 h notice to АЗЛП, inform users if high risk | R-15 |
| G-29 | Deployment and migration docs deleted inside an unrelated commit | restore them, or keep the processor list in `docs/legal/` | I-17 |
| G-30 | The owner administers the German VPS from North Macedonia (a transfer under EDPB 05/2021) | check that the Contabo DPA covers SCC Module 4 or similar; low risk | X-04 |
| G-31 | Inactive accounts kept forever | optional: notify after 2 years of inactivity, then erase | R-11 |

### OK (re-check on every re-audit)

- Browser storage is strictly necessary only, so no cookie banner is needed.
- The refresh cookie is httpOnly, Secure, SameSite=Strict and path-scoped; the access token is in memory.
- No analytics, error tracking, ads or third-party scripts other than Google Fonts and GIS.
- Chat, whispers, emotes, invites and presence are kept in memory only and never logged.
- No IP, user agent, device or location in the database.
- Virtual coins only, with no payments or cash-out.
- bcrypt passwords; SHA-256 token hashes; the JWT carries `sub` only.
- Avatars are re-encoded (EXIF stripped); old pairs are deleted on replace or remove; the bucket cannot be listed.
- Every route is authenticated except health, auth and aggregate stats; no IDOR on `/users/:id` writes; email is never exposed to other users.
- WebSocket events carry no email or IP.
- `robots.txt` and the sitemap keep player pages unindexed; /privacy and /terms are routed and in the sitemap.
- Bots are not derived from real users; error responses are clean; username rules block email-like names.
- No `.env` file or OAuth client secret in git history; CORS and WebSocket origin allowlist; database not internet-exposed.
- Password reset is the only email sent, with no tracking.

---

## 12. Re-audit checklist

Re-run the `gdpr-legal-expert` agent and confirm:
- G-01 to G-17 are closed or accepted with a documented decision.
- No new personal-data column, outbound host or storage key has appeared since `fdb87af1`.
- The erasure script runs cleanly against a fresh copy of the local DB for a user with rooms, matches, friends, a season row and an avatar, and leaves the other players' stats unchanged.

Then draft the Privacy Policy and Terms (§10) in en, sr, hr and mk.

---

## 13. Fix plan (draft, 2026-11-28)

Every owner question is answered. These are the closures in order. "Policy" means the item is closed by text in the Privacy Policy or Terms, written after the re-audit.

### Owner tasks (no code)

| Task | Closes |
|---|---|
| Set up Resend for `beljot.online` (EU region): add its DNS records in Cloudflare, set `BELJOT_SMTP_HOST=smtp.resend.com`, `PORT=587`, `USERNAME=resend`, `PASSWORD=<API key>` and the From address in Dokploy, then test a password reset. Afterwards revoke the Gmail App password and delete the reset emails from Sent. | G-08 |

### PR 1: client privacy (`fix/client-privacy`): DONE 2026-11-28 (`b97ecd62`)

| Change | Closes |
|---|---|
| Self-host Inter, JetBrains Mono and Cormorant Garamond; remove the Google Fonts links | G-01 |
| Signup checkbox (email and Google paths): "I am 16 or older, I agree to the Terms and have read the Privacy Policy", in 4 locales | G-12, G-18 |
| Privacy and Terms links in the in-app footer or settings | G-21 |
| Optional: Content-Security-Policy in `client/nginx.conf` | G-27 |

### PR 2: erasure, export and retention: DONE 2026-11-28 (stacked on `fix/client-privacy`)

Implemented:
- `DELETE /api/v1/users/:id` with password or username confirmation, blocked while seated. One-transaction tombstone erase in `server/internal/account/`.
- An in-memory revocation list, so the still-valid access token is rejected immediately; it is lifted again if the erase fails.
- The user's WebSocket is disconnected, the avatar deleted and the refresh cookie cleared.
- Daily purge in `server/internal/retention/`.
- Generated `player_xxxxxx` usernames for Google sign-ups, and a hard delete on SSO rollback.
- `server/scripts/{erase,export}_user.sql` and `docs/legal/DATA_REQUESTS.md`.
- Known limit: revocation is per API instance and resets on restart. Acceptable with a single instance, because the refresh tokens are already gone and lookups hide the tombstone.

| Change | Closes |
|---|---|
| Self-service **Delete account** in Settings (owner decision 2026-11-28). It confirms by password, or by typing the username for Google-only accounts, and is blocked while the user is seated in a waiting or playing room. It runs the §7 anonymise transaction, deletes the avatar from S3, clears the refresh cookie and closes the user's sockets. Includes 4 locales and a test that other players' stats stay unchanged. The §7 SQL stays in the runbook as the admin fallback for emailed requests. | G-03 |
| `server/scripts/export_user.sql` producing one JSON document | G-16 |
| `docs/legal/DATA_REQUESTS.md`: erasure and export runbook (S3, Cloudflare purge, reply template, one-month deadline) plus a half-page breach runbook | G-03, G-16, G-28 |
| SSO sign-up rollback becomes a hard delete (`Unscoped()`) | G-03 (preventive) |
| Daily purge of old refresh and reset tokens | G-14 |
| Google sign-up gets a generated username (e.g. `player_4821`) instead of the real name; the first rename stays free | G-11 |

### PR 3: logging and hardening: DONE 2026-11-28 (`b5f76483`)

Also found and fixed:
- go-mail error text echoed the recipient address; it is now masked in `mailer/smtp.go`.
- The whisper lookup error logged the target username.

Limits:

| Endpoints | Limit |
|---|---|
| login, register, SSO | 10 per minute per IP |
| forgot-password, reset-password | 5 per 15 minutes |
| refresh | 60 per minute |
| delete account | 5 per 15 minutes per user |

The IP comes from the Cloudflare-aware extractor; the owner confirmed that Traefik forwards the real client IP.

| Change | Closes |
|---|---|
| Stop logging the email on SMTP failure; GORM `ParameterizedQueries: true`; nginx access log off, or with the query string stripped | G-07 |
| Missing SMTP config is fatal outside development | G-19 |
| Echo `Recover` middleware | G-26 |
| Rate limit on the auth endpoints | G-20 |
| Dummy bcrypt compare for unknown emails | G-24 |

### PR 4: repo hygiene: DONE 2026-11-28

- Real usernames and their ranking history in `_bmad-output` were replaced with placeholders: 6 files, including a mockup and a translation note.
- The fixture is now `abcd efgh ijkl mnop`.
- Test fixtures that use the bare name `kiro` with example.com emails carry no real data and are left as is.
- `docs/brand/source/landing.mjs` names real players for the landing screenshots with made-up stats; the owner decides (G-05).

| Change | Closes |
|---|---|
| Replace real usernames in `_bmad-output` with placeholders | G-05 |
| Optional: obviously fake App-password fixture | G-04 |

### Closed by the policy text

G-02, G-10 (Google's sign-in script loads on the login, register and settings pages; owner decision 2026-11-28: disclose, keep), G-06 (14-day backups), G-07 (14-day logs, including Traefik), G-09 (Cloudflare and `__cf_bm`), G-13 (honor), G-15 (match and room retention), G-17 (no representative), G-22 (manual email change), G-23 (invite visibility), G-25 (report contact), G-30 and G-31 (noted).

### Then

Re-run `gdpr-legal-expert` against this document. When it is all green, draft the Privacy Policy and Terms in en, sr, hr and mk.

---

## 14. Re-audit run 2 (2026-11-28)

**Branch:** `fix/client-privacy` @ `fcaf7416`. **Scope:** every change in `62606e78..HEAD` under `client/` and `server/`, plus a fresh sweep of schema, client build, storage and dependencies.

**Verdict: GREEN. Ready to draft the Privacy Policy and Terms.** No finding is OPEN. G-02 (placeholder legal pages) is still the one launch blocker, and the documents themselves close it. One new WARNING (G-32) is closed by either a Cloudflare cache rule or a single sentence in the policy.

**Counts (G-01 to G-31):** 16 CLOSED (code), 2 CLOSED (owner), 9 POLICY, 4 ACCEPTED, 0 OPEN.

### 14.1 Status of run 1 findings

Each status was checked against the code, not against the DONE notes in §13.

| ID | Status | Evidence (run 2) |
|---|---|---|
| G-01 | CLOSED (code) | Links removed from `client/index.html`; `client/src/fonts.css` imports `@fontsource/*`; a fresh `npm run build` `dist/` has no `fonts.googleapis`/`fonts.gstatic` |
| G-02 | POLICY | `PrivacyPage.tsx`/`TermsPage.tsx` and `legal.*` still show "Work in progress". Closes when both documents are published in en, sr, hr and mk and the WIP strings are removed |
| G-03 | CLOSED (code) | `account/gorm_repo.go` `Erase` (one transaction, row lock, seated recheck); `auth/account_delete_handler.go` (revocation, WS close, avatar delete, cookie clear); `DELETE /users/:id` is behind auth and the per-user limit. `TestErase_*` pass against `beljot_test`. The erase covers every `users` column and every FK table in `pg_constraint` |
| G-04 | CLOSED (owner) | False positive; the fixture is now obviously fake |
| G-05 | CLOSED (code) | `909b97f5` placeholders; the screenshot players consented (§9 #11). History not rewritten: accepted |
| G-06 | POLICY | Nightly DB and Garage backups, 14 days, same VPS (§9 #3) |
| G-07 | CLOSED (code) | The request logger now logs the path without the query string (`cmd/api/main.go` `useCoreMiddleware`); `access_log off` (`client/nginx.conf:11`); reset failures log `userID` (`password_reset.go:117`); SMTP errors mask the recipient (`mailer/smtp.go`); `ParameterizedQueries: true`. The 14-day log retention goes in the policy |
| G-08 | CLOSED (owner) | Resend, EU region |
| G-09 | POLICY | Cloudflare plus `__cf_bm` |
| G-10 | POLICY | `googleIdentity.ts:35,50` still loads GIS when the page opens (owner decision). `accounts.google.com` is the only third-party script host in `dist/` |
| G-11 | CLOSED (code) | `sso_handler.go` `generateSSOUsername` gives `player_` plus 6 random characters and never reads the Google name. Note: Google accounts created **before** this change keep name-derived usernames (1 of 1 locally). Owner: count them in production; their first rename is free |
| G-12 | CLOSED (code) | 16+ in the register checkbox and the Google note, in 4 locales (`auth.register.consent`, `auth.sso.consent`). The Terms state 16+ |
| G-13 | POLICY | Honor profiling text (§14.3) |
| G-14 | CLOSED (code) | `retention/purger.go` runs at start and every 24 h. `refreshtoken.PurgeStale`: `family_expires_at < now OR revoked_at < now-30d`. `passwordreset.PurgeStale`: `used_at IS NOT NULL OR expires_at < now-24h`. Purge tests pass |
| G-15 | POLICY | Matches kept for the life of the service, anonymised on erasure |
| G-16 | CLOSED (code) | `server/scripts/export_user.sql` ran read-only against a local user: profile, identities, friends, seasons, matches, rooms owned, sessions. The only email in the output is the user's own; other players appear by username, and erased players as null |
| G-17 | ACCEPTED | No EU representative (owner, §9 #12) |
| G-18 | CLOSED (code) | Wording "I agree to the Terms and have read the Privacy Policy". Storing acceptance stays optional (consent is not the legal basis) |
| G-19 | CLOSED (code) | `config.go` `missingSMTPVars` exits outside development, so the LogMailer is development-only |
| G-20 | CLOSED (code) | `cmd/api/ratelimits.go`: per-IP limits on login, register, SSO, forgot, reset and refresh; per-user limit on delete |
| G-21 | CLOSED (code) | `settings/components/PrivacySection.tsx:45-49` links /privacy and /terms |
| G-22 | POLICY | Email change by request |
| G-23 | POLICY | `allow_non_friend_invites` still defaults to `true` (DB default) |
| G-24 | CLOSED (code) | Dummy bcrypt compare on unknown or passwordless login (`handler.go`). `EMAIL_TAKEN` on register stays: accepted, and rate-limited |
| G-25 | POLICY | The Terms name the report email |
| G-26 | CLOSED (code) | `RecoverWithConfig` logs the method, path, panic and stack, never the query string or body |
| G-27 | ACCEPTED | No CSP; optional |
| G-28 | CLOSED (code) | Breach runbook in `docs/legal/DATA_REQUESTS.md` §6 |
| G-29 | CLOSED (code) | The processor inventory lives in `docs/legal/` (§3 of this file) |
| G-30 | ACCEPTED | Low risk, noted |
| G-31 | ACCEPTED | Accounts are kept until the user deletes them; the policy says so |

### 14.2 New findings

**G-32 WARNING: an erased avatar stays in Cloudflare's edge cache.** Art. 17.
- Evidence: avatar objects carry `public, max-age=31536000, immutable` (`avatar/handler.go:48`). Self-service erasure deletes the S3 pair, but the Cloudflare purge is manual and depends on the owner noticing the `account erased … avatarPrefix=` log line (`DATA_REQUESTS.md` §2). In practice nobody watches for it. The URL is a random UUID, so the exposure is small.
- Fix (smallest, owner, no code): a Cloudflare Cache Rule on `assets.beljot.online` with Edge TTL 1 day. It also covers existing objects. Browser caches of other players are out of reach and do not need action.
- Policy: "A deleted profile picture may remain in our CDN cache for up to 1 day [or: until purged] and in the browser cache of players who saw it."
- **Fixed 2026-11-28:** new avatar uploads carry `Cache-Control: public, max-age=86400`, so Cloudflare and browsers drop them within a day (`avatar/handler.go`). Avatars uploaded before this keep their 1-year header, so for those the runbook's manual Cloudflare purge still applies. The policy says a deleted picture can stay in caches for up to a day.

**G-33 RECOMMENDATION: erasure can drift from the schema.** Art. 17, Art. 25.
- Evidence: `scrubUser` and `erase_user.sql` list `users` columns by hand, and `TestErase_AnonymisesInPlaceAndKeepsOpponentsStats` asserts only some of them. It does not check `last_login_at`, `username_changed_at`, the preferences, the honor weights or `created_at`.
- A future personal column would survive erasure unnoticed. `export_user.sql` uses `to_jsonb`, so the export would pick the column up.
- Fix: a test that compares `information_schema.columns` for `users` (and the FK list to `users`) with a pinned list, so the next migration forces a decision. Also assert the remaining scrubbed columns.
- **Fixed 2026-11-28:** `server/internal/account/schema_coverage_test.go` reads the live schema. It fails if a `users` column is not reset by `scrubUser`, or if a foreign key to `users` is neither deleted by the erase nor on the kept-reference list.

**G-34 RECOMMENDATION: runbook lookup and wording.**
- `DATA_REQUESTS.md` §3 step 1 finds the user only by `users.email`. A request sent from a linked Google address misses. Add `OR id IN (SELECT user_id FROM user_identities WHERE email = '<email>')`.
- §6 step 5 says to keep the register "next to this file, outside the public repository". Say "outside the repository".
- §5 should name the 180-day absolute session limit.
- **Fixed 2026-11-28:** the runbook lookup matches both the account email and a linked Google email, case-insensitively.

### 14.3 OK, new in run 2 (re-check each audit)

- **Schema:** still version 38, with no migration after `000038`. No new table or column. `room_bots`, `seasons` and `season_recalculations` hold no personal data.
- **Dependencies:** the only new ones are `@fontsource/{inter,jetbrains-mono,cormorant-garamond}` (bundled, no network calls). No `go.mod` change.
- **Client build:** the only third-party hosts in the built bundle are `accounts.google.com` (GIS) and plain outbound social and donation links. Browser storage keys are unchanged from §2.4.
- **In-memory state:**
  - Rate-limit buckets hold `ip:<addr>` with timestamps for at most one window (15 min at most), are swept on use, and are lost on restart.
  - The revocation list holds user ids for the access-token TTL plus 1 minute.
  - Invite grants expire after 60 s.
  - The WebSocket client holds only the user id.
  - None of this is persisted.
- **Erasure:** blocked while seated, rechecked under a row lock. `abandoned_by`/`surrendered_by` keep the tombstone id. Opponents' stats are unchanged (asserted). A second erase returns not found.
- **Account deletion is safe:** the password from the delete request is never logged. The request logger has no body, and the recover log has no query string.
- **Account erased log line:** carries only the user id and the random avatar prefix.

### 14.4 Policy must state (checklist for drafting)

**Controller and contact**
- [ ] Controller: Emilijan Koteski, an individual, North Macedonia. [ACTION REQUIRED: postal address as in the Monexa policy]
- [ ] Contact and rights requests: emilijankoteski@pm.me (Proton AG, Switzerland).
- [ ] No DPO (Art. 37 reasoning as in Monexa).
- [ ] No EU representative (Art. 27).
- [ ] Complaints: АЗЛП (Агенција за заштита на личните податоци, North Macedonia), or the user's own EU/EEA supervisory authority.

**Data, purposes and legal bases**
- [ ] Data, purposes and legal bases from §2.1 and §4.
- [ ] Not collected: chat, whispers and emotes are never stored; no real name, birthdate, phone or payment data; no analytics, ads or tracking.
- [ ] Google sign-in: only `sub`, email and verification status are used. The Google name and picture are not used; the username is generated.

**Processors and recipients**
- [ ] Contabo GmbH, Germany: hosting, database, avatars, logs, backups.
- [ ] Cloudflare, Inc., US / global edge: DNS, proxy, TLS, avatar CDN, bot protection; EU-US DPF plus SCCs.
- [ ] Resend (Plus Five Five, Inc., US), EU sending region: password-reset email; DPF plus SCCs.
- [ ] Google (Google Ireland Ltd for EEA users): **independent controller** for Sign in with Google. Its script loads on the login, register and settings pages.
- [ ] Proton AG, Switzerland (adequacy decision): the contact mailbox.

**Retention**
- [ ] Account: until the user deletes it.
- [ ] Server and proxy logs (IP address, request path, time), including Traefik: 14 days.
- [ ] Backups (database and avatars), nightly, same German VPS: 14 days. Erasure reaches backups when they rotate out.
- [ ] Login sessions: deleted at the latest 180 days after sign-in, or 30 days after logout or revocation.
- [ ] Password-reset tokens: valid 1 hour, deleted within about 2 days.
- [ ] Rate-limit IP data: memory only, at most 15 minutes.
- [ ] Matches, hand results and season standings: life of the service. On erasure they are anonymised (no name, no link to the person).
- [ ] Rooms the user owned: renamed on erasure.
- [ ] Avatars: deleted on replace, remove or erasure; CDN cache note (G-32).
- [ ] Resend sending logs: per Resend's retention.

**Cookies and storage (strictly necessary, so no banner)**
- [ ] `refresh_token`: httpOnly cookie, up to 30 days idle and 180 days absolute; keeps the user signed in.
- [ ] `__cf_bm`: Cloudflare bot protection, about 30 minutes.
- [ ] localStorage `beljot.lang`, `beljot.theme`: UI preferences.
- [ ] sessionStorage `beljot.theme.picked`, `beljot:preload-error-reload`, `beljot:version-reloaded-for`, `beljot:lobby-idx`: UI state.
- [ ] Google's own cookies inside the sign-in frame: under Google's policy.

**Honor score (profiling, Art. 4(4))**
- [ ] Computed from completed and abandoned matches, with a 90-day half-life decay.
- [ ] Shown on the profile.
- [ ] Room owners may set a minimum honor to join.
- [ ] No legal or similarly significant effect, so Art. 22 does not apply.
- [ ] The right to object (Art. 21) by email.

**What other players see**
- [ ] Username, avatar, join date, level and XP, honor (including completed and abandoned counts), match history and stats, leaderboard rank.
- [ ] Online status to friends.
- [ ] "Invitable" status to room owners while `allow_non_friend_invites` is on, which is the default and can be switched off in Settings.
- [ ] Email is never shown.

**Rights and how to use them (reply within one month)**
- [ ] Access and portability: JSON export by email.
- [ ] Rectification: self-service username (30-day cooldown, the first change is free), avatar and preferences; password via reset; **email change by request**.
- [ ] Erasure: self-service **Settings → Danger zone → Delete account**, or by email. Not possible while seated in a room or match. Match results stay for the other players without a name.
- [ ] Restriction and objection: by email.
- [ ] Requests are acted on only from the account's email address.

**Security**
- [ ] bcrypt passwords, hashed tokens, TLS, access token held in memory only, rate limiting on sign-in and reset, and a breach procedure with 72-hour notice to АЗЛП.

**Age**
- [ ] 16+, confirmed at signup on both the email and Google paths. No birthdate is collected.

**Terms of Service**
- [ ] Minimum age 16.
- [ ] Virtual coins have no monetary value and cannot be bought, sold, transferred for value or cashed out (still no payment code: verified).
- [ ] Username and chat conduct.
- [ ] Report contact (DSA): emilijankoteski@pm.me.
- [ ] Honor system and penalties for abandoning.
- [ ] Suspension or termination.
- [ ] Self-service deletion.
- [ ] Service provided "as is".
- [ ] [ACTION REQUIRED: governing law and jurisdiction]
- [ ] "Last updated" date and version.

**Before publication**
- [ ] Have both documents reviewed by a qualified lawyer.
