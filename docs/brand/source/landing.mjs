import { mkdirSync, readFileSync, writeFileSync } from "node:fs";
import { createRequire } from "node:module";
import { dirname, join } from "node:path";
import { fileURLToPath, pathToFileURL } from "node:url";

const here = dirname(fileURLToPath(import.meta.url));
const clientDir = join(here, "..", "..", "..", "client");
const require = createRequire(join(clientDir, "package.json"));
const { chromium } = require("playwright");
const { createServer } = await import(pathToFileURL(require.resolve("vite")).href);

const landingOut = join(clientDir, "public", "landing");
const mascotOut = join(clientDir, "public", "mascot");
const mascotSrc = join(here, "..", "mascot");
mkdirSync(landingOut, { recursive: true });
mkdirSync(mascotOut, { recursive: true });

const DAY = 86400000;
const now = Date.now();
const user = (theme, deck = "french") => ({
  token: "t", id: 1, username: "emilijan", email: "emilijan@example.com", languagePreference: "en",
  cardDeckPreference: deck, themePreference: theme, soundEnabled: false, musicEnabled: false,
  soundVolume: 0, musicVolume: 0, allowNonFriendInvites: true, walletBalance: 171554,
  loginStreakDays: 6, totalXp: 3400, level: 20, honorScore: 95, honorTier: "exemplary",
  isNewPlayer: false, avatarUrl: null, createdAt: "2026-03-04T10:00:00Z",
});
const rank = (tier, division, sp, position, played, won) => ({
  seasonName: "2026 Q4", tier, division, sp, position, players: 18, gamesPlayed: played, gamesWon: won,
});
const profile = (theme) => ({
  id: 1, username: "emilijan", avatarUrl: null, avatarLargeUrl: null, usernameChangedAt: null,
  languagePreference: "en", cardDeckPreference: "french", themePreference: theme, soundEnabled: false,
  musicEnabled: false, soundVolume: 0, musicVolume: 0, allowNonFriendInvites: true,
  createdAt: "2026-03-04T10:00:00Z", totalGamesPlayed: 214, wins: 118, losses: 92, abandoned: 4,
  totalXp: 3400, level: 20, xpIntoLevel: 340, xpForNextLevel: 900, honorScore: 95,
  honorTier: "exemplary", honorCompletedTotal: 66, honorAbandonedTotal: 2, isNewPlayer: false,
  honorTrendDelta: 3, honorTrendDirection: "up", seasonRank: rank("bronze", 2, 222, 6, 68, 30),
});
const career = {
  capots: 9, avgMatchSeconds: 1440, careerPoints: 61240, streak: { kind: "win", length: 4 },
  bestHand: { points: 252, handNumber: 7, completedAt: "2026-10-12T20:00:00Z" },
  lastPlayedAt: new Date(now).toISOString(),
  topPartners: [{ userId: 7, username: "Natalija", avatarUrl: null, played: 41, wins: 26 }],
  topRivals: [{ userId: 10, username: "Cece", avatarUrl: null, wins: 14, losses: 19 }],
};
const seasons = {
  items: [
    { id: 8, name: "2026 Q4", startedAt: new Date(now - 30 * DAY).toISOString(), endsAt: new Date(now + 62 * DAY).toISOString() },
    { id: 7, name: "2026 Q3", startedAt: new Date(now - 122 * DAY).toISOString(), endsAt: new Date(now - 30 * DAY).toISOString() },
  ],
};
const current = {
  seasonName: "2026 Q4", endsAt: seasons.items[0].endsAt, sp: 222, rankTier: "bronze", rankDivision: 2,
  spIntoDivision: 22, spForNextDivision: 50, gamesPlayed: 68, gamesCompleted: 68, gamesWon: 30,
};
const tiers = [[1400, "grandmaster", null], [1200, "master", null], [1000, "diamond", 3], [800, "platinum", 2], [600, "gold", 2], [300, "silver", 2], [150, "bronze", 1], [0, "iron", 1]];
const names = ["marko_k", "ana.b", "zoki77", "petar", "ivana_m", "emilijan", "sneza", "boban", "lile", "kire", "goce", "vesna"];
const rowAt = (pos) => {
  const sp = Math.max(0, 1620 - pos * 120);
  const [, tier, division] = tiers.find(([floor]) => sp >= floor);
  const played = 12 + ((pos * 7) % 40);
  return { position: pos, userId: pos === 6 ? 1 : 100 + pos, username: names[pos - 1], avatarUrl: null, sp, tier, division, gamesPlayed: played, gamesWon: Math.round(played * (0.75 - pos * 0.02)) };
};
const seat = (roomId, s, userId, username, isBot = false) => ({
  id: roomId * 10 + s, roomId, userId: isBot ? 0 : userId, username: isBot ? "" : username, avatarUrl: null,
  seat: s, team: s % 2 === 0 ? "teamA" : "teamB", isBot, createdAt: "",
});
const room = (id, name, extra) => ({
  id, name, code: `K7P2Q${id}`, ownerId: 20 + id, ownerUsername: `owner${id}`, ownerAvatarUrl: null,
  variant: id % 2 ? "croatia" : "bitola", matchMode: id % 2 ? "1001" : "501", timerStyle: "per-move",
  timerDurationSeconds: 20, coinBuyIn: id * 250, isPrivate: id === 3, minHonor: id === 2 ? 70 : 0,
  allowNewPlayers: true, declarationsEnabled: true, stopAtTarget: true, declarationsTransfer: true,
  clockwise: false, status: "waiting", playerCount: 0, isQuickPlay: false,
  createdAt: "2026-10-29T07:00:00Z", updatedAt: "2026-10-29T07:00:00Z", ...extra,
});
const rooms = [
  room(1, "Soba - Natalija", { playerCount: 3, players: [seat(1, 0, 7, "Natalija"), seat(1, 1, 10, "Cece"), seat(1, 2, 9, "stef")] }),
  room(2, "Večerna partija", { playerCount: 2, players: [seat(2, 0, 21, "marko_k"), seat(2, 3, 22, "ana.b")] }),
  room(3, "Privatna", { playerCount: 3, players: [seat(3, 0, 23, "zoki77"), seat(3, 1, 24, "petar"), seat(3, 2, 0, "", true)] }),
  room(5, "Brza 501", { playerCount: 1, variant: "bitola", matchMode: "501", timerStyle: "relaxed", players: [seat(5, 0, 25, "ivana_m")] }),
  room(6, "Kafanska", { playerCount: 2, isPrivate: false, minHonor: 0, players: [seat(6, 0, 26, "dragan"), seat(6, 2, 27, "sneza")] }),
  room(4, "Turnir", { status: "in_progress", playerCount: 4, players: [seat(4, 0, 28, "boban"), seat(4, 1, 29, "lile"), seat(4, 2, 30, "kire"), seat(4, 3, 31, "goce")] }),
];
const friends = ["Natalija", "Cece", "stef", "DiC"].map((n, i) => ({ id: i + 7, username: n, avatarUrl: null, online: i < 2 }));

const card = (id) => ({ rank: id[0], suit: id[1] });
const player = (s, userId, username, level, hand, handCount) => ({
  hand, seat: s, userId, username, avatarUrl: null, team: s % 2 === 0 ? "teamA" : "teamB",
  declarations: [], connected: true, isBot: false, level, faceDownCount: 0, handCount, declarationAnswered: true,
});
const matchState = () => ({
  id: 1, roomId: 100, variant: "croatia", matchMode: "1001", phase: "playing", handNumber: 4, dealerSeat: 3,
  trumpSuit: "H", trumpCallerSeat: 2, trumpCandidate: null, biddingRound: 1, biddingPassCount: 0,
  mustPickTrump: false, declarationsEnabled: true, stopAtTarget: false, declarationsTransfer: true,
  clockwise: false, activePlayerSeat: 0, trickNumber: 3,
  currentTrick: [{ playerSeat: 1, card: card("KD") }, { playerSeat: 2, card: card("AD") }, { playerSeat: 3, card: card("9D") }],
  leadSuit: "D", trickWinnerSeat: null, awaitingDeclaration: false, declarationsResolved: true,
  players: [
    player(0, 1, "emilijan", 20, ["JH", "9H", "AH", "TS", "KC", "8D"].map(card), 6),
    player(1, 11, "Kiro", 32, [], 5), player(2, 7, "Natalija", 24, [], 5), player(3, 12, "Irena", 28, [], 5),
  ],
  teamScores: [462, 318], handPoints: [34, 21], declarationPoints: [0, 0], belotPoints: [0, 0], tricksWon: [1, 1],
  pendingBelotSeat: null, belotAnnounced: false, winnerTeam: null, lastHandResult: null,
  turnExpiresAt: new Date(Date.now() + 13000).toISOString(), timerDurationSec: 20, previousPhase: "",
  pausedPlayers: [false, false, false, false], pauseUsed: [false, false, false, false],
  playerPauseExpiresAt: [null, null, null, null], pauseWindowSec: 120, surrenderProposerSeat: null,
  surrenderUsed: [false, false, false, false], turnTimeRemaining: 13, ownerSeat: 0, disconnectedSeat: -1,
  reconnectExpiresAt: null, playerReconnectExpiresAt: [null, null, null, null],
});

/** Answers every API call the captured pages make with the sample data above. */
function mockApi(page, theme, deck) {
  return page.route("**/api/v1/**", (route) => {
    const url = new URL(route.request().url());
    const path = url.pathname.replace("/api/v1", "");
    const ok = (data) => route.fulfill({ json: { data } });
    if (path === "/auth/refresh") return ok(user(theme, deck));
    if (path === "/rooms") return ok(rooms);
    if (path === "/lobby/stats") return ok({ inLobby: 12, inRoom: 5, inMatch: 8, online: 25, registered: 400 });
    if (path === "/friends") return ok(friends);
    if (path === "/friends/requests") return ok([]);
    if (path === "/users/1/profile") return ok(profile(theme));
    if (/^\/users\/\d+\/career$/.test(path)) return ok(career);
    if (/^\/users\/\d+\/matches$/.test(path)) return ok({ items: [], total: 214, limit: 20, offset: 0 });
    if (/^\/users\/\d+\/seasons$/.test(path)) return ok({ items: [] });
    if (path === "/seasons") return ok(seasons);
    if (path === "/seasons/current") return ok(current);
    if (path === "/leaderboard") {
      const items = Array.from({ length: 12 }, (_, i) => rowAt(i + 1));
      return ok({ seasonId: 8, seasonName: "2026 Q4", items, total: 12, limit: 25, offset: 0, viewer: rowAt(6) });
    }
    if (path.includes("chat") || path.includes("messages")) return ok([]);
    return route.fulfill({ status: 404, json: { error: { code: "NOT_FOUND", message: "not mocked" } } });
  });
}

/** Re-encodes an image as WebP at the given width with the browser's own encoder. */
async function encodeWebp(page, bytes, mime, width, quality) {
  const dataUrl = await page.evaluate(
    async ({ b64, mime, width, quality }) => {
      const img = new Image();
      img.src = `data:${mime};base64,${b64}`;
      await img.decode();
      const height = Math.round((img.height * width) / img.width);
      const canvas = document.createElement("canvas");
      canvas.width = width;
      canvas.height = height;
      const ctx = canvas.getContext("2d");
      ctx.imageSmoothingQuality = "high";
      ctx.drawImage(img, 0, 0, width, height);
      return canvas.toDataURL("image/webp", quality);
    },
    { b64: Buffer.from(bytes).toString("base64"), mime, width, quality },
  );
  return Buffer.from(dataUrl.split(",")[1], "base64");
}

const VIEWPORTS = { phone: [390, 844, 2], desktop: [1440, 900, 1], wide: [1280, 900, 2] };

/** Opens one app route as the sample player and returns its PNG screenshot. */
async function capture(browser, base, { path, viewport, theme = "parchment", deck = "french", match = false }) {
  const [width, height, scale] = VIEWPORTS[viewport];
  const page = await browser.newPage({ viewport: { width, height }, deviceScaleFactor: scale });
  await page.addInitScript(() => localStorage.setItem("beljot.lang", "en"));
  await mockApi(page, theme, deck);
  await page.goto(base + path);
  await page.waitForTimeout(1500);
  if (match) {
    await page.evaluate(async (state) => {
      const { useMatchStore } = await import("/src/shared/stores/matchStore.ts");
      useMatchStore.setState({ matchState: state, myPlayerSeat: 0, roomId: 100, isLoading: false });
    }, matchState());
    await page.waitForTimeout(2500);
  }
  const png = await page.screenshot();
  await page.close();
  return png;
}

/** Reads an image's natural size in the browser. */
async function imageSize(page, bytes, mime) {
  return page.evaluate(
    async ({ b64, mime }) => {
      const img = new Image();
      img.src = `data:${mime};base64,${b64}`;
      await img.decode();
      return [img.width, img.height];
    },
    { b64: bytes.toString("base64"), mime },
  );
}

const server = await createServer({ root: clientDir, logLevel: "silent", server: { port: 5199 } });
await server.listen();
const base = server.resolvedUrls.local[0].replace(/\/$/, "");
const browser = await chromium.launch();
const encoder = await browser.newPage();

const shots = [
  ["match-desktop", { path: "/match/100", viewport: "desktop", match: true }, 1440],
  ["match-phone", { path: "/match/100", viewport: "phone", match: true }, 600],
  ["match-phone-croatian", { path: "/match/100", viewport: "phone", deck: "croatian", match: true }, 380],
];
for (const theme of ["parchment", "brass"]) {
  shots.push([`lobby-desktop-${theme}`, { path: "/lobby", viewport: "wide", theme }, 1400]);
  shots.push([`lobby-phone-${theme}`, { path: "/lobby", viewport: "phone", theme }, 560]);
  shots.push([`profile-phone-${theme}`, { path: "/profile", viewport: "phone", theme }, 580]);
  shots.push([`leaderboard-phone-${theme}`, { path: "/leaderboard", viewport: "phone", theme }, 580]);
}
for (const [name, options, width] of shots) {
  const png = await capture(browser, base, options);
  const webp = await encodeWebp(encoder, png, "image/png", width, 0.82);
  writeFileSync(join(landingOut, `${name}.webp`), webp);
  console.log(`landing/${name}.webp`, `${Math.round(webp.length / 1024)} KB`);
}

const mascots = [["dama-signature", 1480], ["dama-win-half", 1400], ["dama-pointing", 680]];
for (const [name, height] of mascots) {
  const bytes = readFileSync(join(mascotSrc, `${name}.webp`));
  const [srcWidth, srcHeight] = await imageSize(encoder, bytes, "image/webp");
  const width = Math.round((srcWidth * height) / srcHeight);
  const webp = await encodeWebp(encoder, bytes, "image/webp", width, 0.86);
  writeFileSync(join(mascotOut, `${name}.webp`), webp);
  console.log(`mascot/${name}.webp`, `${width}x${height}`, `${Math.round(webp.length / 1024)} KB`);
}

await browser.close();
await server.close();
