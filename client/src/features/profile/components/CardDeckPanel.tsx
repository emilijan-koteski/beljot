import { Check } from "lucide-react";
import { useTranslation } from "react-i18next";

import type { CardDeck } from "@/features/match/lib/cardFace";
import { cardFaceUrl, resolveCardDeck } from "@/features/match/lib/cardFace";
import { persistCardDeck } from "@/shared/lib/cardDeckPreference";
import { cn } from "@/shared/lib/utils";
import { useAuthStore } from "@/shared/stores/authStore";

import { SidePanel } from "./SidePanel";

const DECKS = [
  { code: "french", labelKey: "cardDeck.french" },
  { code: "croatian", labelKey: "cardDeck.croatian" },
] as const satisfies ReadonlyArray<{ code: CardDeck; labelKey: string }>;

const ACE_OF_DIAMONDS_SHOWS_MOST_DECK_DIFFERENCE = "AD" as const;

/**
 * Self-only profile picker for the card deck, saved optimistically with a silent revert on failure.
 * It is a private preference, so it must never appear on the public profile page.
 */
export function CardDeckPanel() {
  const { t } = useTranslation();
  const active = resolveCardDeck(useAuthStore((s) => s.user?.cardDeckPreference));

  async function handleSelect(next: CardDeck) {
    if (next === active) return;
    await persistCardDeck(next, active);
  }

  return (
    <SidePanel
      eyebrow={t("profile.cardDeck.eyebrow")}
      title={t("profile.cardDeck.title")}
      testId="profile-card-deck"
    >
      <p className="text-ink-mute mb-2.5 text-xs">{t("profile.cardDeck.description")}</p>
      <div className="flex gap-2.5" role="radiogroup" aria-label={t("profile.cardDeck.title")}>
        {DECKS.map((deck) => {
          const selected = active === deck.code;
          const previewUrl = cardFaceUrl(ACE_OF_DIAMONDS_SHOWS_MOST_DECK_DIFFERENCE, deck.code);
          return (
            <button
              key={deck.code}
              type="button"
              role="radio"
              aria-checked={selected}
              onClick={() => handleSelect(deck.code)}
              data-testid={`profile-deck-option-${deck.code}`}
              className={cn(
                "flex flex-1 cursor-pointer flex-col items-center gap-1.5 rounded-md border px-2 py-2.5 transition-colors",
                selected
                  ? "border-accent bg-accent-soft"
                  : "border-border bg-surface-elevated hover:bg-surface-sunken",
              )}
            >
              <img
                key={previewUrl}
                src={previewUrl}
                alt=""
                aria-hidden="true"
                draggable={false}
                className="h-[63px] w-[45px] rounded-xs shadow-felt-sm"
                style={{
                  background:
                    "linear-gradient(180deg, var(--card-face) 0%, var(--card-face-shade) 100%)",
                }}
                onError={(e) => {
                  e.currentTarget.style.visibility = "hidden";
                }}
                onLoad={(e) => {
                  e.currentTarget.style.visibility = "";
                }}
              />
              <span className="text-ink flex items-center gap-1 text-xs font-medium">
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
