import { Swords } from "lucide-react";
import { useTranslation } from "react-i18next";

import type { RivalStat } from "@/shared/api/career";
import { SeasonRecord } from "@/shared/components/season/LeaderboardRow";
import { Panel } from "@/shared/components/ui/panel";

import { PersonRow } from "./PersonRow";

type RivalsCardProps = {
  rivals: RivalStat[];
  subjectIsSelf?: boolean;
};

/** The most-faced opponents, each with the subject's wins and losses against them. */
export function RivalsCard({ rivals, subjectIsSelf = true }: RivalsCardProps) {
  const { t } = useTranslation();

  return (
    <Panel icon={<Swords />} title={t("profile.rivals.title")} testId="profile-rivals">
      {rivals.length === 0 ? (
        <p className="text-ink-mute m-0 text-sm">
          {t(subjectIsSelf ? "profile.rivals.empty" : "profile.rivals.emptyPublic")}
        </p>
      ) : (
        <ul className="m-0 flex list-none flex-col gap-2.5 p-0">
          {rivals.map((r) => (
            <PersonRow
              key={r.userId}
              userId={r.userId}
              username={r.username}
              avatarUrl={r.avatarUrl}
              team="B"
              sub={t("season.leaderboard.played", { count: r.wins + r.losses })}
            >
              <SeasonRecord won={r.wins} played={r.wins + r.losses} />
            </PersonRow>
          ))}
        </ul>
      )}
    </Panel>
  );
}
