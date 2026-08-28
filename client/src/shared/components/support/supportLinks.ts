/**
 * Buy Me a Coffee links, assets, and the one piece of logic that picks between
 * them.
 *
 * Module constants rather than i18n values on purpose: the URL and the asset
 * paths are identical in every language, and keeping them out of the locale
 * files keeps one more Latin string out of mk.json's Cyrillic prose. Mirrors
 * how ContactDialog holds EMAIL / LINKEDIN_URL.
 */

/**
 * Canonical page URL. The `www.` form the BMC dashboard hands out
 * 301-redirects here, so link the target and skip the hop.
 */
export const BMC_URL = "https://buymeacoffee.com/emilijan";

/**
 * Static first frame of the sticker below. Two jobs: the dialog's mark under
 * prefers-reduced-motion, and the tiny inline icon SupportNote puts at the end
 * of its sentence — a 13px animation in a footer line would be exactly the
 * nagging this feature is built to avoid.
 */
export const BMC_MARK_SRC = "/support/bmc-cup.gif";

/**
 * The branded QR exported from the BMC dashboard, downscaled from its 3000px
 * original (514 KB -> 80 KB at 384px, still 2x the size it renders at). Fetched
 * only when someone expands the QR panel, which is the rarest path here.
 */
export const BMC_QR_SRC = "/support/bmc-qr.png";

/**
 * The animated sticker from Buy Me a Coffee's own Giphy channel, re-encoded
 * down from the 480px/100-frame original: 176px, every second frame kept at
 * double the delay so the 5 s loop is unchanged. 569 KB -> 111 KB.
 *
 * Only ever fetched when someone opens the dialog: a closed base-ui Dialog
 * renders no portal content, so the <img> does not exist until then.
 *
 * Kept as `string | null` so dropping the animation is a one-line change.
 */
export const BMC_STICKER_SRC: string | null = "/support/bmc-cup-anim.gif";

/**
 * Which image the dialog shows as its mark. The animated sticker is used only
 * when one is configured AND the viewer hasn't asked for reduced motion;
 * everything else falls back to the static brand mark.
 *
 * A function rather than an inline ternary so the reduced-motion contract stays
 * testable while BMC_STICKER_SRC is still null — pass `sticker` explicitly to
 * exercise the animated branch.
 */
export function supportMarkSrc(
  prefersReducedMotion: boolean,
  sticker: string | null = BMC_STICKER_SRC,
): string {
  return sticker !== null && !prefersReducedMotion ? sticker : BMC_MARK_SRC;
}
