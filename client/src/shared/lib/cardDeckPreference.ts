import { updatePreferences } from "@/shared/api/profile";
import { useAuthStore } from "@/shared/stores/authStore";
import type { CardDeck } from "@/shared/types/matchTypes";

/**
 * Monotonic token for in-flight deck writes.
 *
 * Module-level rather than per-component on purpose: the two entry points (the
 * in-match Settings dialog and the profile sidebar panel) are different
 * components writing the *same* server field, so a per-component counter would
 * not order them against each other.
 */
let latestRequest = 0;

/**
 * Persist the player's card deck, optimistically.
 *
 * The auth store is the single value every `PlayingCard` and every trump surface
 * renders from, so writing it before the request is what makes a deck change
 * apply instantly, mid-hand, with no reload. A failed request rolls the store
 * back; the failure is silent, matching `LanguageSelector`, because there is
 * nothing the player can do about it and the visible state is already correct.
 *
 * **Latest-wins.** Two fast toggles whose responses arrive out of order used to
 * leave the store and the server disagreeing silently: each handler captured its
 * own `previousDeck` before its own request, so a late failure from the FIRST
 * toggle would revert to a value the player had already moved past — or worse,
 * a late *success* from the first would be the server's final state while the
 * store showed the second. The sequence check makes every superseded response a
 * no-op, so the last toggle the player actually made is the only one that
 * decides both store and server.
 *
 * @param next the deck to write
 * @param previous the deck to restore on failure — pass the RESOLVED current
 *   deck (`resolveCardDeck(...)`), not the raw stored string, or a rollback can
 *   reinstate an unrecognised value that renders as a missing asset.
 */
export async function persistCardDeck(next: CardDeck, previous: CardDeck): Promise<void> {
  const user = useAuthStore.getState().user;
  if (!user) return;

  const request = ++latestRequest;
  useAuthStore.getState().setUser({ ...user, cardDeckPreference: next });

  try {
    // Deck-only body: resending the language would let a deck change clobber a
    // language picked in another tab.
    await updatePreferences(user.id, { cardDeckPreference: next });
  } catch {
    // Superseded by a later toggle — that one owns the outcome now, and
    // reverting here would fight it.
    if (request !== latestRequest) return;
    const current = useAuthStore.getState().user;
    if (current?.id === user.id) {
      useAuthStore.getState().setUser({ ...current, cardDeckPreference: previous });
    }
  }
}

/** Test seam: reset the in-flight sequence between cases. */
export function resetCardDeckRequestSequence(): void {
  latestRequest = 0;
}
