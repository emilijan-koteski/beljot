# Data requests and breach runbook

How Beljot.online handles GDPR requests from players (access, export, erasure) and what to do after a personal-data breach. It complements `GDPR_COMPLIANCE_AUDIT.md` (§7 erasure procedure).

## 1. How players ask

- **Erasure:** the player deletes the account themselves in **Settings → Danger zone → Delete account**. It asks for the password, or for the username on Google-only accounts, and is refused while the player sits in a waiting or playing room.
- **Everything else** (export, access, correction, erasure for someone locked out): email **emilijankoteski@pm.me**.
- **Deadline:** reply within **one month** of receiving the request (Art. 12(3)). Log the date it arrived.
- **Identity check:** act only on a request sent from the account's email address, or one the player can otherwise prove they control (for example by signing in and changing their username to a word you give them). Never send data to a different address.

## 2. Self-service deletion: what the server does

`DELETE /api/v1/users/:id` runs the §7 anonymisation in one transaction, then deletes the avatar from S3, clears the refresh cookie, closes the player's WebSocket and rejects their remaining access token. Nothing else is needed from the admin, except:

- **Cloudflare cache:** avatars uploaded since 28 Nov 2026 drop out of the edge cache within a day; older uploads carry a 1-year cache header and may still be served from the edge until purged. The server logs `account erased` with `userID` and `avatarPrefix`; when the prefix is not empty, purge its two URLs (step 3 in §3). If the same request logged `avatar: best-effort delete failed`, delete the two objects from S3 by hand too.

## 3. Admin erasure (emailed request)

Use this when the player cannot use the Settings button. `server/scripts/erase_user.sql` runs exactly what the server code does.

1. **Find the user id.** Match the address against both the account email and a linked Google email, since a player may write from either:
   ```sql
   SELECT u.id, u.username, u.deleted_at FROM users u
   WHERE lower(u.email) = lower('<email>')
      OR u.id IN (SELECT user_id FROM user_identities WHERE lower(email) = lower('<email>'));
   ```
2. **Erase:**
   ```bash
   psql "$BELJOT_DB_URL" -X -v uid=<id> -f server/scripts/erase_user.sql
   ```
   - `ABORTED: the user is seated…`: they are in a waiting or playing room. Wait until the match ends and run it again.
   - `no rows returned for \gset`: no live user has that id (wrong id, or already erased).
   - On success it prints `Erased user <id>` and the two avatar object keys, if any.
3. **Avatar:** delete both objects from the `beljot-public` bucket (`<prefix>/256.webp` and `<prefix>/128.webp`), then purge both URLs from the Cloudflare cache: `https://assets.beljot.online/<prefix>/256.webp` and `https://assets.beljot.online/<prefix>/128.webp` (Cloudflare dashboard → Caching → Configuration → Custom Purge).
   Also search the logs for `avatar: best-effort delete failed … userID=<id>` and delete any older orphaned prefixes the same way.
4. **Email:** delete the request thread after sending the reply below, keeping only a one-line record (date received, date done, user id, request type). Delete any password-reset emails to that address that the mail provider kept.
5. **Logs and backups** are not edited. They age out within **14 days**; say so in the reply.
6. **Reply** (template):

   > Hello,
   >
   > Your Beljot.online account has been deleted on <date>. We removed your email, username, password, linked Google sign-in, profile picture, friends, season standings and settings. Results of matches you played stay in the other players' history, but without your name or any link to you.
   >
   > Server logs and backups that may still contain your data are deleted automatically within 14 days.
   >
   > Regards,
   > Beljot.online

## 4. Export (access and portability)

`server/scripts/export_user.sql` is read-only and prints one JSON document: profile (no password hash), linked identities, friends with usernames, season standings, matches with the other seats' usernames, rooms owned and login sessions.

```bash
psql "$BELJOT_DB_URL" -X -q -v uid=<id> -f server/scripts/export_user.sql > beljot-export-<id>.json
```

Send the file to the account's email address, then delete your local copy. Reply text: "Attached is all personal data Beljot.online holds about your account, as JSON. Server logs (kept 14 days) are not included."

## 5. Retention that runs by itself

- Refresh tokens: deleted daily once their session passed its absolute limit, or 30 days after revocation.
- Password-reset tokens: deleted daily once used, or one day after they expired.
- Server logs and backups: 14 days.

## 6. Breach runbook

A breach is any accidental or unlawful loss, change, disclosure of or access to personal data: a leaked database dump or backup, a leaked secret (JWT, database, S3 or SMTP credentials), unauthorised access to the server, or an avatar bucket exposed beyond its intended public reads.

1. **Assess (first hours).** Write down what happened, when it started, when it was noticed, which data and how many players are affected, and whether the data was encrypted or hashed (passwords and tokens are hashed).
2. **Contain.** Rotate the affected secrets (`BELJOT_JWT_SECRET` also signs every user out), revoke leaked credentials, close the access path, and restore from a clean backup if data was changed.
3. **Notify the authority within 72 hours** of becoming aware, unless the breach is unlikely to put anyone at risk. The authority is the **Agency for Personal Data Protection of North Macedonia (АЗЛП, Агенција за заштита на личните податоци)**, through the breach notification form on its website. If not everything is known yet, notify with what you have and send the rest later.
4. **Inform players** without delay if the risk to them is high (for example emails together with password hashes leaked): what happened, what data, what you did, and what they should do (change the password, watch for phishing). Use email to each affected address, plus a notice on the site if many are affected.
5. **Record it in the breach register,** even when no notification was needed: date, description, data and players affected, consequences, actions taken, and why the authority or players were or were not notified. Keep the register next to this file, outside the public repository.
