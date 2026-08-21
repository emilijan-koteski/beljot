import { Check } from "lucide-react";
import { useTranslation } from "react-i18next";

import type { CardDeck } from "@/features/match/lib/cardFace";
import { cardFaceUrl, resolveCardDeck } from "@/features/match/lib/cardFace";
import { persistCardDeck } from "@/shared/lib/cardDeckPreference";
import { cn } from "@/shared/lib/utils";
import { useAuthStore } from "@/shared/stores/authStore";

import { SidePanel } from "./SidePanel";

// Order matches the in-match SettingsDialog: French first, since it is the
// default and the deck an account that never chose is already looking at.
const DECKS = [
  { code: "french", labelKey: "cardDeck.french" },
  { code: "croatian", labelKey: "cardDeck.croatian" },
] as const satisfies ReadonlyArray<{ code: CardDeck; labelKey: string }>;

/**
 * The card ID each option previews.
 *
 * Deliberately the ace of DIAMONDS/BELLS. Hearts is the one suit that looks
 * nearly identical in both decks, so previewing it hid the very thing the
 * choice changes; `D` swaps a red diamond for a gold bell, which is the most
 * legible difference in the set at 45 px.
 */
const PREVIEW_CARD = "AD" as const;

/**
 * Profile-sidebar card-deck picker (Story 12.4), so the deck is settable outside
 * a match as well as from the in-game Settings dialog.
 *
 * Persistence is the same optimistic-write-with-silent-revert the language
 * selector uses: the auth store is the single source every `PlayingCard` reads,
 * so the choice applies immediately with no reload, and a failed PATCH rolls the
 * store back rather than surfacing an error the player cannot act on.
 *
 * SELF-ONLY. This is a private preference, absent from `PublicProfileResponse`,
 * so the panel mounts on `ProfilePage` and must never appear on
 * `PublicPlayerProfilePage`.
 */
export function CardDeckPanel() {
  const { t } = useTranslation();
  const active = resolveCardDeck(useAuthStore((s) => s.user?.cardDeckPreference));

  async function handleSelect(next: CardDeck) {
    if (next === active) return;
    // Same latest-wins helper the in-match Settings dialog uses, so a toggle
    // here and a toggle there are ordered against each other rather than each
    // racing on its own captured "previous". `active` is already resolved.
    await persistCardDeck(next, active);
  }

  return (
    <SidePanel
      eyebrow={t("profile.cardDeck.eyebrow")}
      title={t("profile.cardDeck.title")}
      testId="profile-card-deck"
    >
      <p className="text-ink-mute mb-2.5 text-[12px]">{t("profile.cardDeck.description")}</p>
      <div className="flex gap-2.5" role="radiogroup" aria-label={t("profile.cardDeck.title")}>
        {DECKS.map((deck) => {
          const selected = active === deck.code;
          const previewUrl = cardFaceUrl(PREVIEW_CARD, deck.code);
          return (
            <button
              key={deck.code}
              type="button"
              role="radio"
              aria-checked={selected}
              onClick={() => handleSelect(deck.code)}
              data-testid={`profile-deck-option-${deck.code}`}
              className={cn(
                "flex flex-1 cursor-pointer flex-col items-center gap-1.5 rounded-[10px] border px-2 py-2.5 transition-colors",
                selected
                  ? "border-accent bg-accent-soft"
                  : "border-border bg-surface-elevated hover:bg-surface-sunken",
              )}
            >
              {/* Decorative: the option's own label is the announcement, and the
                  radio's aria-checked carries the state. Same `onError` fallback
                  as PlayingCard — production answers a missing asset with 200
                  index.html, which browsers paint as a broken-image glyph. */}
              <img
                // Keyed by src for the same reason as PlayingCard's face: React
                // would otherwise reuse this node across deck changes and the
                // imperative hide below would outlive the failure that set it.
                key={previewUrl}
                src={previewUrl}
                alt=""
                aria-hidden="true"
                draggable={false}
                className="h-[63px] w-[45px] rounded-[2px] shadow-[0_2px_6px_-2px_rgba(0,0,0,0.35)]"
                style={{ background: "linear-gradient(180deg, #fdfaf0 0%, #f4ecd8 100%)" }}
                onError={(e) => {
                  e.currentTarget.style.visibility = "hidden";
                }}
                onLoad={(e) => {
                  e.currentTarget.style.visibility = "";
                }}
              />
              <span className="text-ink flex items-center gap-1 text-[12px] font-medium">
                {t(deck.labelKey)}
                {selected && <Check className="text-accent size-3" strokeWidth={2.6} />}
              </span>
            </button>
          );
        })}
      </div>
    </SidePanel>
  );
}
