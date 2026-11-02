import {
  Calculator,
  Hand,
  Layers,
  type LucideIcon,
  Megaphone,
  SlidersHorizontal,
  Spade,
} from "lucide-react";

import type { SectionId } from "./content/types";

export const CHAPTER_ICON: Record<SectionId, LucideIcon> = {
  deal: Layers,
  cards: Spade,
  play: Hand,
  melds: Megaphone,
  scoring: Calculator,
  options: SlidersHorizontal,
};
