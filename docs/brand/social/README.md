# Social media kit

Profile images, covers, launch posts and copy for the Beljot.online profiles. Macedonian (Cyrillic) comes first, Croatian second, and English only where a field is global.

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
| `instagram-grid/post-N-of-9.jpg` | 1080×1440 each | The mosaic cut into 3:4 tiles, numbered in posting order. |

The link preview (`client/public/og-image.jpg`) stays in English because it shows wherever the site is shared.

## Regenerate

`docs/brand/source/social.py` lays out every image as SVG, and `social.mjs` renders it with Playwright. `landing.mjs --social` captures the two match screenshots for post 2, with a Macedonian UI and the sample players Elena, Ana, Luka and Marko. Taglines use Cormorant Garamond, the site's display serif, because Cinzel has no Cyrillic.

```bash
cd docs/brand/source
./fetch_fonts.sh && pip install -r requirements.txt
python3 social.py
cd ../../../client
node ../docs/brand/source/landing.mjs --social
node ../docs/brand/source/social.mjs
```

## Bios

Tone: inviting. Every profile links to the site with a UTM tag for its platform.

**Instagram** (150-character limit, this is 116)

```text
Седни на масата. Бељот онлајн, 2 на 2, бесплатно 🃏
Sjedni za stol. Bela online, 2 na 2, besplatno ♥️
Играј / Igraj 👇
```

Website: `https://beljot.online/?utm_source=instagram&utm_medium=social`

**Facebook bio** (101-character limit, this is 89)

```text
Седни на масата. Бељот онлајн, 2 на 2, бесплатно. Sjedni za stol. Bela online, besplatno.
```

Website: `https://beljot.online/?utm_source=facebook&utm_medium=social`

**YouTube channel description**

```text
Седни на масата. Бељот онлајн: битолски и хрватски бељот за четворица, 2 на 2, директно во прелистувачот. Играј со пријатели, непознати или ботови и искачи се на сезонската ранг-листа. Бесплатно и без инсталација.

Sjedni za stol. Bela online: bitolska i hrvatska bela za četvero, 2 na 2, izravno u pregledniku. Igraj s prijateljima, strancima ili botovima i penji se na sezonskoj ljestvici. Besplatno i bez instalacije.

Free Belot for four in your browser.

▶ https://beljot.online/?utm_source=youtube&utm_medium=social
Instagram: https://www.instagram.com/beljot.online
Facebook: https://www.facebook.com/beljot.online
```

Add the links again as channel links: Beljot.online, Instagram, Facebook.

**TikTok** (80-character limit, this is 55)

```text
Седни на масата 🃏 Sjedni za stol ♥️ Бељот / Bela online
```

Website: `https://beljot.online/?utm_source=tiktok&utm_medium=social`

## Launch posts

Post all three to Instagram and Facebook together from Meta Business Suite, a day or two apart, before any promotion.

### 1. Welcome: `post-01-welcome.jpg`

```text
Седни на масата. 🃏
Бељот онлајн е отворен: 2 на 2, бесплатно, директно во прелистувачот. Без инсталација, само повикај ги своите.

Sjedni za stol. ♥️
Bela online je otvorena: 2 na 2, besplatno, izravno u pregledniku. Bez instalacije, samo pozovi ekipu.

👉 beljot.online (линк во биото / link u biografiji)

#бељот #beljot #bela #belot #картање #kartanje #kartaškeigre #bitola #makedonija #hrvatska
```

Alt text: Дама, домаќинката на Beljot.online, во бордо фустан покажува кон месинганиот жетон со буквата B на зелена чоја. Натпис: Седни на масата. Sjedni za stol.

### 2. Two decks: `post-02-decks.jpg`

```text
Француски или хрватски карти? Ти избираш. 🂡
Истата маса, истиот меч, само смени го шпилот во поставките.

Francuske ili hrvatske karte? Ti biraš. 🌰
Isti stol, isti meč, samo promijeni špil u postavkama.

#бељот #beljot #bela #belot #hrvatskekarte #mađarice #картање #kartanje #beljotonline
```

Alt text: Два телефони со иста партија бељот: лево со француски карти, десно со хрватски карти. Натпис: Француски или хрватски карти? Ти избираш.

### 3. Bots: `post-03-bots.jpg`

```text
Фали четврти? 🤖
Додај ботови на празните места и мечот почнува веднаш. Кога ќе дојде другарот, тргни го ботот и местото е негово.

Fali četvrti? 🤖
Dodaj botove na prazna mjesta i meč počinje odmah. Kad prijatelj stigne, makni bota i mjesto je njegovo.

#бељот #beljot #bela #belot #картање #kartanje #igrekarata #beljotonline
```

Alt text: Дама дели карти, а до неа е месингана икона на бот. Натпис: Фали четврти? Ботот седнува веднаш.

## Instagram grid mosaic

The profile grid shows 3:4 tiles, so the mosaic is cut into 1080×1440 parts that the grid shows whole. The newest post sits top-left, so post the parts in file order: `post-1-of-9.jpg` (bottom-right) first and `post-9-of-9.jpg` (top-left) last.

- Post them one by one as single photos, not as a carousel. In the crop step, pick the original 3:4 shape so nothing is trimmed.
- Post later content in groups of three so the mosaic moves down whole rows, and don't pin posts, because pinned posts jump to the top-left and break the picture.

## Setup by platform

### Instagram (mobile app)

1. Profile → **Edit profile** → **Change picture** → `profile-1080.png`.
2. Paste the Instagram bio into **Bio**.
3. **Links** → **Add external link** → the Instagram website URL, titled `Играј / Igraj`. The website link can only be set in the app.
4. **Contact options** → business email. **Category** → Games/Toys, and show it on the profile.
5. **Page** should show Beljot.online. If it doesn't, link it under **Accounts Center** → **Accounts** → **Add Facebook account**.

### Facebook Page

1. Switch to the Page profile, then **Edit profile picture** → `profile-1080.png`.
2. **Add cover photo** → `facebook-cover-1640x624.png`. Check it on a phone, which crops the sides.
3. **Edit details**: paste the bio, set the website to the Facebook URL, and confirm the category.
4. **Settings** → **Page setup** → **Username** → `beljot.online`. If it's taken, update the URL everywhere listed above.
5. Under the cover, **+ Add button** → **Play game** (or **Learn more**) → the Facebook website URL.
6. **Settings** → **Page access** → add a second admin with full control.
7. Run https://beljot.online through the [Sharing Debugger](https://developers.facebook.com/tools/debug/) and click **Scrape Again** after each deploy that changes meta tags.

### YouTube (Studio → Customization)

1. **Handle** → `@beljot.online`. If it's not available, use `@beljotonline` and update the URLs above.
2. **Branding** → Picture: `profile-800.png`; Banner: `youtube-banner-2560x1440.jpg` (check the preview for desktop, mobile and TV); Video watermark: `watermark-150.png`, shown for the whole video.
3. **Basic info** → paste the description, add the three links, and add the contact email.
4. **Settings** → **Channel** → **Advanced settings** → Audience: **No, set this channel as not made for kids**.
5. **Settings** → **Upload defaults**: add the description footer with links, tags `бељот, beljot, bela, belot`, category Gaming, and language Macedonian.
6. https://www.youtube.com/verify → phone verification, which unlocks custom thumbnails and longer videos.
7. **Layout** → set the channel trailer once a 30–60 s gameplay clip exists.

### TikTok (later)

Use `profile-800.png` (TikTok scales it down), the TikTok bio and website link. A Business account only gets the commercial music library. A Creator account gets trending sounds but has limits on promoting a product.

## Security

Turn on 2FA for the brand Google account, the owner's Facebook profile and Instagram. Keep the backup codes outside Gmail, for example in a password manager.
