import { Check } from "lucide-react";
import { useTranslation } from "react-i18next";

import type { CardDeck } from "@/features/match/lib/cardFace";
import { cardFaceUrl, resolveCardDeck } from "@/features/match/lib/cardFace";
import { persistCardDeck } from "@/shared/lib/cardDeckPreference";
import { cn } from "@/shared/lib/utils";
import { useAuthStore } from "@/shared/stores/authStore";

import { SettingsRow } from "./SettingsGroup";

const DECKS = [
  { code: "french", labelKey: "cardDeck.french" },
  { code: "croatian", labelKey: "cardDeck.croatian" },
] as const satisfies ReadonlyArray<{ code: CardDeck; labelKey: string }>;

const ACE_OF_DIAMONDS_SHOWS_MOST_DECK_DIFFERENCE = "AD" as const;

/**
 * The card deck setting, French or Croatian, shown as each deck's ace; saved optimistically with a silent revert.
 * `wide` puts the choice on the row's right instead of under its name.
 */
export function CardDeckRow({ wide = false }: { wide?: boolean }) {
  const { t } = useTranslation();
  const active = resolveCardDeck(useAuthStore((s) => s.user?.cardDeckPreference));

  async function handleSelect(next: CardDeck) {
    if (next === active) return;
    await persistCardDeck(next, active);
  }

  const choices = (
    <div
      className={cn("flex gap-2.5", wide && "w-64 shrink-0")}
      role="radiogroup"
      aria-label={t("profile.cardDeck.title")}
    >
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
              "flex flex-1 cursor-pointer flex-col items-center gap-1.5 rounded-lg border px-2 py-2.5 transition-colors",
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
            <span className="text-ink flex items-center gap-1 text-xs font-semibold">
              {selected && <Check className="text-accent size-3" strokeWidth={2.6} />}
              {t(deck.labelKey)}
            </span>
          </button>
        );
      })}
    </div>
  );

  return (
    <SettingsRow
      testId="profile-card-deck"
      label={t("profile.cardDeck.title")}
      hint={t("profile.cardDeck.description")}
      stack={!wide}
      control={wide ? choices : undefined}
    >
      {!wide && choices}
    </SettingsRow>
  );
}
