import { cardFaceUrl } from "@/features/match/lib/cardFace";
import type { CardDeck, CardId } from "@/shared/types/matchTypes";

const HEARTS: readonly CardId[] = ["JH", "9H", "AH", "TH", "KH"];
const SPREAD_DEG = 26;

/** Five hearts from a real deck, fanned and peeking out of the bottom of their panel. */
export function DeckFan({ deck }: { deck: CardDeck }) {
  const step = SPREAD_DEG / (HEARTS.length - 1);
  return (
    <div className="-mb-9 flex justify-center pt-6.5" aria-hidden="true">
      {HEARTS.map((id, i) => (
        <img
          key={id}
          src={cardFaceUrl(id, deck)}
          alt=""
          loading="lazy"
          decoding="async"
          className="w-20 origin-[50%_120%] rounded-lg bg-[#fbf8ef] shadow-felt-md ring-1 ring-black/10 not-first:-ml-11 md:w-24 md:not-first:-ml-13"
          style={{ transform: `rotate(${-SPREAD_DEG / 2 + i * step}deg)` }}
        />
      ))}
    </div>
  );
}
