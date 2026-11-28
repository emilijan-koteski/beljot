# Social media kit

Brand rules and setup steps for the Beljot.online profiles.

## Brand rules

- **Language:** Macedonian in Cyrillic first, Croatian second, English only where a field is shown globally. In Croatian the game is "bela"; the brand stays "Beljot.online".
- **Tone:** inviting. The tagline is "Седни на масата." / "Sjedni za stol."
- **Type:** Macedonian and Croatian lines use Cormorant Garamond, because Cinzel has no Cyrillic. Macedonian is cream and Croatian brass, split by the suit divider, with BELJOT.ONLINE in cream Cinzel caps as the sign-off.
- **Links:** every profile links to the site with its platform's UTM tag: `https://beljot.online/?utm_source=<instagram|facebook|youtube|tiktok>&utm_medium=social`.
- **Profiles:**

  | Platform | Profile URL |
  | --- | --- |
  | Instagram | https://www.instagram.com/beljot.online |
  | Facebook | https://www.facebook.com/beljot.online |
  | YouTube | https://www.youtube.com/@beljot.online |
  | TikTok (later) | https://www.tiktok.com/@beljot.online |

  The same URLs appear in two other places. If a handle changes, update all three: `SOCIAL_PROFILES` in `client/src/features/landing/components/SocialLinks.tsx`, `sameAs` in the JSON-LD in `client/index.html`, and this table.

## Files

The images are not kept in git. Run the commands under **Regenerate** to build them into this folder, which ignores everything except this guide.

| File | Size | Use |
| --- | --- | --- |
| `profile-1080.png` | 1080×1080 | Instagram and Facebook profile picture. |
| `profile-800.png` | 800×800 | YouTube profile picture. |
| `watermark-150.png` | 150×150, transparent | YouTube video watermark. |
| `facebook-cover-1640x624.png` | 1640×624 | Facebook cover: the card fan with the chip, the taglines and the BELJOT.ONLINE sign-off. Text sits in the centre so the mobile crop keeps it. |
| `youtube-banner-2560x1440.jpg` | 2560×1440 | YouTube banner. The card fan with the chip, the taglines and the sign-off sit inside the 1546×423 safe area. |
| `post-01-welcome.jpg` | 1080×1350 | Launch post 1: welcome. |
| `post-02-decks.jpg` | 1080×1350 | Launch post 2: French or Croatian deck. |
| `post-03-bots.jpg` | 1080×1350 | Launch post 3: bots fill the empty seat. |
| `instagram-grid.jpg` | 3240×4320 | Preview of the 3×3 Instagram mosaic: Dama, the chip and the taglines. |
| `instagram-grid/post-N-of-9.jpg` | 1080×1440 each | The mosaic cut into 3:4 tiles. Post them as single photos in file order, part 1 (bottom-right) first, since the newest post sits top-left. |

The link preview (`client/public/og-image.jpg`) stays in English because it shows wherever the site is shared.

## Regenerate

`docs/brand/source/social.py` lays out every image as SVG, and `social.mjs` renders it with Playwright. `landing.mjs --social` captures the two match screenshots for post 2, with a Macedonian UI and the sample players Elena, Ana, Luka and Marko.

```bash
cd docs/brand/source
./fetch_fonts.sh && pip install -r requirements.txt
python3 social.py
cd ../../../client
node ../docs/brand/source/landing.mjs --social
node ../docs/brand/source/social.mjs
```

## Setup by platform

### Instagram (mobile app)

1. Profile → **Edit profile** → **Change picture** → `profile-1080.png`.
2. **Bio:** the tagline and one line in each language, within 150 characters.
3. **Links** → **Add external link** → the Instagram website URL, titled `Играј / Igraj`. The website link can only be set in the app.
4. **Contact options** → business email. **Category** → Games/Toys, and show it on the profile.
5. **Page** should show Beljot.online. If it doesn't, link it under **Accounts Center** → **Accounts** → **Add Facebook account**.

### Facebook Page

1. Switch to the Page profile, then **Edit profile picture** → `profile-1080.png`.
2. **Add cover photo** → `facebook-cover-1640x624.png`. Check it on a phone, which crops the sides.
3. **Edit details**: a bio of at most 101 characters in both languages, set the website to the Facebook URL, and confirm the category.
4. **Settings** → **Page setup** → **Username** → `beljot.online`. If it's taken, update the URL everywhere listed above.
5. Under the cover, **+ Add button** → **Play game** (or **Learn more**) → the Facebook website URL.
6. **Settings** → **Page access** → add a second admin with full control.
7. Run https://beljot.online through the [Sharing Debugger](https://developers.facebook.com/tools/debug/) and click **Scrape Again** after each deploy that changes meta tags.

### YouTube (Studio → Customization)

1. **Handle** → `@beljot.online`. If it's not available, use `@beljotonline` and update the URLs above.
2. **Branding** → Picture: `profile-800.png`; Banner: `youtube-banner-2560x1440.jpg` (check the preview for desktop, mobile and TV); Video watermark: `watermark-150.png`, shown for the whole video.
3. **Basic info** → a description in Macedonian, then Croatian, then one English line, with the site and profile links; add the three links, and add the contact email.
4. **Settings** → **Channel** → **Advanced settings** → Audience: **No, set this channel as not made for kids**.
5. **Settings** → **Upload defaults**: add the description footer with links, tags `бељот, beljot, bela, belot`, category Gaming, and language Macedonian.
6. https://www.youtube.com/verify → phone verification, which unlocks custom thumbnails and longer videos.
7. **Layout** → set the channel trailer once a 30–60 s gameplay clip exists.

### TikTok (later)

Use `profile-800.png` (TikTok scales it down), a bio of at most 80 characters and the TikTok website link. A Business account only gets the commercial music library. A Creator account gets trending sounds but has limits on promoting a product.

## Security

Turn on 2FA for the brand Google account, the owner's Facebook profile and Instagram. Keep the backup codes outside Gmail, for example in a password manager.
