import * as React from "react";

/**
 * Horizontal divider with the four suit glyphs in the middle.
 * Glyphs carry the U+FE0E text variation selector so they never render as larger emoji.
 */
export function SuitRule() {
  const glyphStyle: React.CSSProperties = {
    fontFamily: "var(--font-suit)",
    fontSize: 18,
    lineHeight: 1,
    display: "inline-block",
  };
  return (
    <div className="mt-1 mb-4 flex items-center gap-3.5" aria-hidden="true">
      <div className="bg-border h-px flex-1" />
      <div className="flex items-center gap-2.5 tracking-wider">
        <span style={{ ...glyphStyle, color: "var(--danger)" }}>{"♥︎"}</span>
        <span style={{ ...glyphStyle, color: "var(--ink)" }}>{"♠︎"}</span>
        <span style={{ ...glyphStyle, color: "var(--danger)" }}>{"♦︎"}</span>
        <span style={{ ...glyphStyle, color: "var(--ink)" }}>{"♣︎"}</span>
      </div>
      <div className="bg-border h-px flex-1" />
    </div>
  );
}
