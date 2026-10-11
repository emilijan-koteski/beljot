import type { TFunction } from "i18next";

/**
 * Random adjective + noun room name from the active locale's word lists,
 * differing from `prev` when possible.
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
