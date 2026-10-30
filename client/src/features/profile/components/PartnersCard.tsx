import { Handshake } from "lucide-react";
import { useTranslation } from "react-i18next";

import type { PartnerStat } from "@/shared/api/career";
import { Panel } from "@/shared/components/ui/panel";

import { PersonRow } from "./PersonRow";

type PartnersCardProps = {
  partners: PartnerStat[];
  subjectIsSelf?: boolean;
};

/** Win rate (%) over matches played together; 0 when none. */
function winRate(wins: number, total: number): number {
  return total === 0 ? 0 : Math.round((wins / total) * 100);
}

/** The most-played partners, each with the matches played together and the win rate together. */
export function PartnersCard({ partners, subjectIsSelf = true }: PartnersCardProps) {
  const { t } = useTranslation();

  return (
    <Panel icon={<Handshake />} title={t("profile.partners.title")} testId="profile-partners">
      {partners.length === 0 ? (
        <p className="text-ink-mute m-0 text-sm">
          {t(subjectIsSelf ? "profile.partners.empty" : "profile.partners.emptyPublic")}
        </p>
      ) : (
        <ul className="m-0 flex list-none flex-col gap-2.5 p-0">
          {partners.map((p) => (
            <PersonRow
              key={p.userId}
              userId={p.userId}
              username={p.username}
              avatarUrl={p.avatarUrl}
              team="A"
              sub={t("profile.partners.matchesTogether", { count: p.played })}
            >
              <span className="text-ink-dim text-sm font-semibold tabular-nums">
                {winRate(p.wins, p.played)}%
              </span>
            </PersonRow>
          ))}
        </ul>
      )}
    </Panel>
  );
}
