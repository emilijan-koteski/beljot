export const BMC_URL = "https://buymeacoffee.com/emilijan";

export const BMC_MARK_SRC = "/support/bmc-cup.gif";

export const BMC_QR_SRC = "/support/bmc-qr.png";

export const BMC_STICKER_SRC: string | null = "/support/bmc-cup-anim.gif";

/** Picks the dialog's mark: the animated sticker unless it is unset or reduced motion is preferred. */
export function supportMarkSrc(
  prefersReducedMotion: boolean,
  sticker: string | null = BMC_STICKER_SRC,
): string {
  return sticker !== null && !prefersReducedMotion ? sticker : BMC_MARK_SRC;
}
