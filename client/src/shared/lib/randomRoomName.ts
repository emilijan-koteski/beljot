import type { TFunction } from "i18next";

/**
 * Adjective + noun combo, drawn from the host's own language pack
 * (`lobby.createRoomModal.randomName.*` in each locale file) rather than one
 * fixed list — so a Serbian host never sees a Macedonian-flavoured suggestion.
 *
 * Pre-fills the Create Room dialog's name field so "Create room" is enabled
 * the moment it opens; the shuffle button calls this again to reroll,
 * excluding whatever name is already showing.
 */
export function randomRoomName(t: TFunction, prev?: string): string {
  const adjectives = t("lobby.createRoomModal.randomName.adjectives", {
    returnObjects: true,
  }) as string[];
  const nouns = t("lobby.createRoomModal.randomName.nouns", { returnObjects: true }) as string[];

  if (!Array.isArray(adjectives) || !Array.isArray(nouns) || !adjectives.length || !nouns.length) {
    return prev ?? "";
  }

  let name = prev;
  do {
    const adjective = adjectives[Math.floor(Math.random() * adjectives.length)];
    const noun = nouns[Math.floor(Math.random() * nouns.length)];
    name = `${adjective} ${noun}`;
  } while (name === prev && adjectives.length * nouns.length > 1);
  return name;
}
