const FULL_INSET = 4;
const MIN_INSET = 2;
const INSET_RATIO = 0.08;
const MONOGRAM_RATIO = 0.3;
const MIN_MONOGRAM_PX = 6;

/** Inner frame inset for a card back of this width: 4px on hand-size cards, tighter on seat stacks. */
function insetFor(width: number): number {
  return Math.min(FULL_INSET, Math.max(MIN_INSET, Math.round(width * INSET_RATIO)));
}

/**
 * The inside of a face-down card: the hatched brass frame and the italic serif "B" monogram.
 * Render it in a box that already paints CARD_BACK_BACKGROUND, the brass border and the radius.
 */
export function CardBackFace({ width, radius }: { width: number; radius: number }) {
  const inset = insetFor(width);
  return (
    <>
      <div
        className="absolute"
        style={{
          inset,
          borderRadius: Math.max(0, radius - inset),
          border: "1px solid var(--brass-40)",
          backgroundImage:
            "repeating-linear-gradient(45deg, transparent 0 6px, var(--brass-10) 6px 7px)",
        }}
      />
      <div
        className="absolute inset-0 flex items-center justify-center"
        style={{
          color: "var(--brass)",
          fontFamily: "var(--font-suit)",
          fontSize: Math.max(MIN_MONOGRAM_PX, width * MONOGRAM_RATIO),
          fontStyle: "italic",
          fontWeight: 700,
          textShadow: "0 1px 2px rgba(0,0,0,0.6)",
        }}
      >
        B
      </div>
    </>
  );
}
