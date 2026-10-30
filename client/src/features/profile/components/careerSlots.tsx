import type { UseQueryResult } from "@tanstack/react-query";
import type { ReactNode } from "react";

import type { CareerResponse } from "@/shared/api/career";
import { Skeleton } from "@/shared/components/ui/skeleton";

import { HighlightsCard } from "./HighlightsCard";
import { PartnersCard } from "./PartnersCard";
import { RivalsCard } from "./RivalsCard";

type CareerSlots = { highlights: ReactNode; partners: ReactNode; rivals: ReactNode };

/**
 * The three career-driven cards for a profile layout: skeletons while loading, one error line if it fails.
 * The error takes the highlights slot so it shows where the cards would be.
 */
export function careerSlots(
  career: UseQueryResult<CareerResponse>,
  subjectIsSelf: boolean,
  error: { text: string; testId: string },
): CareerSlots {
  if (career.isError) {
    return {
      highlights: (
        <p className="text-danger m-0 text-sm" data-testid={error.testId}>
          {error.text}
        </p>
      ),
      partners: null,
      rivals: null,
    };
  }
  if (!career.data) {
    return {
      highlights: <Skeleton className="h-36 rounded-xl" />,
      partners: <Skeleton className="h-36 rounded-xl" />,
      rivals: <Skeleton className="h-36 rounded-xl" />,
    };
  }
  return {
    highlights: (
      <HighlightsCard
        capots={career.data.capots}
        careerPoints={career.data.careerPoints}
        bestHand={career.data.bestHand}
        avgMatchSeconds={career.data.avgMatchSeconds}
      />
    ),
    partners: <PartnersCard partners={career.data.topPartners} subjectIsSelf={subjectIsSelf} />,
    rivals: <RivalsCard rivals={career.data.topRivals} subjectIsSelf={subjectIsSelf} />,
  };
}
