// Off-theme coin gold — a DELIBERATE exception to the project's warm
// felt-green / parchment palette, with no shared design token, mirroring the
// frost-blue `ICE` exception in StreakCallout.tsx (the cold-streak callout).
//
// Why an exception: a coin reads as gold/yellow, and the theme has no gold-yellow
// token (the closest, `--brass-deep`, is a muted tan that doesn't say "coin").
// Centralising it here keeps the one off-palette hue in a single place so it
// never leaks into general theming, and both coin-icon sites (the header pill in
// TopBar and the profile pill in IdentityHero) stay in sync.
//
// Tuned to be a legible yellow on the white coin pill (light mode) while still
// reading as gold on the dark-felt surface (dark mode). If you change it, change
// it ONLY here.
export const COIN_GOLD = "#D4A017";
