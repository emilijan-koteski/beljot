import { useTranslation } from "react-i18next";

import type { RivalStat } from "@/shared/api/career";
import { Avatar } from "@/shared/components/ui/avatar";

import { SidePanel } from "./SidePanel";
import { WinLoseBar } from "./WinLoseBar";

type RivalriesProps = {
  rivals: RivalStat[];
  /** False on ANOTHER player's profile (Story 11.3): the empty state tells the
   *  reader to go play a few matches, which only makes sense when the reader is
   *  the subject. Selects the third-person `emptyPublic` copy. */
  subjectIsSelf?: boolean;
};

/**
 * Sidebar panel of most-faced opponents with the viewer's head-to-head record
 * and a win-share bar. Rival avatars use the silver "Them" palette.
 */
export function Rivalries({ rivals, subjectIsSelf = true }: RivalriesProps) {
  const { t } = useTranslation();

  return (
    <SidePanel
      eyebrow={t("profile.rivals.eyebrow")}
      title={t("profile.rivals.title")}
      testId="profile-rivals"
    >
      {rivals.length === 0 ? (
        <p className="text-ink-mute text-[13px]">
          {t(subjectIsSelf ? "profile.rivals.empty" : "profile.rivals.emptyPublic")}
        </p>
      ) : (
        <ul className="m-0 flex list-none flex-col gap-1.5 p-0">
          {rivals.map((r) => {
            const total = r.wins + r.losses;
            const winPct = total === 0 ? 0 : Math.round((r.wins / total) * 100);
            return (
              <li
                key={r.userId}
                className="bg-surface-elevated border-border flex flex-col gap-1.5 rounded-[10px] border p-2.5"
              >
                <div className="flex items-center gap-2.5">
                  <Avatar name={r.username} team="B" size={24} />
                  <span className="text-ink flex-1 truncate text-[13px] font-medium">
                    {r.username}
                  </span>
                  <span className="text-ink-dim text-xs tabular-nums">
                    {total}
                    <span className="text-ink-off"> · </span>
                    <span className="text-ink font-semibold">{winPct}%</span>
                  </span>
                </div>
                <WinLoseBar winPct={winPct} />
              </li>
            );
          })}
        </ul>
      )}
    </SidePanel>
  );
}
