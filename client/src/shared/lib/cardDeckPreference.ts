import { updatePreferences } from "@/shared/api/profile";
import { useAuthStore } from "@/shared/stores/authStore";
import type { CardDeck } from "@/shared/types/matchTypes";

let latestRequest = 0;

/**
 * Optimistically writes the deck to the auth store, then persists it; a failure
 * silently restores `previous` (a resolved deck) unless a later toggle superseded it.
 */
export async function persistCardDeck(next: CardDeck, previous: CardDeck): Promise<void> {
  const user = useAuthStore.getState().user;
  if (!user) return;

  const request = ++latestRequest;
  useAuthStore.getState().setUser({ ...user, cardDeckPreference: next });

  try {
    await updatePreferences(user.id, { cardDeckPreference: next });
  } catch {
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
