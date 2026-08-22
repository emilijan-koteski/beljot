import { useTranslation } from "react-i18next";
import { Link } from "react-router";

import type { PartnerStat } from "@/shared/api/career";
import { Avatar } from "@/shared/components/ui/avatar";

import { SidePanel } from "./SidePanel";
import { WinLoseBar } from "./WinLoseBar";

type PartnerSpotlightProps = {
  partners: PartnerStat[];
  /** False on ANOTHER player's profile (Story 11.3): the empty state tells the
   *  reader to go play a few matches, which only makes sense when the reader is
   *  the subject. Selects the third-person `emptyPublic` copy. */
  subjectIsSelf?: boolean;
};

/** Win rate (%) over matches played together; 0 when none. */
function winRate(wins: number, total: number): number {
  return total === 0 ? 0 : Math.round((wins / total) * 100);
}

/** Avatar + name + together-stats for the featured partner — shared by the
 *  linked wrapper and the non-linked fallback for soft-deleted users. The
 *  hover underline only fires inside the `group`-classed Link. */
function FeaturedPartner({ featured }: { featured: PartnerStat }) {
  const { t } = useTranslation();
  return (
    <>
      <Avatar name={featured.username} team="A" size={56} />
      <div className="min-w-0">
        <div className="text-ink font-display truncate text-lg font-semibold tracking-[-0.1px] underline-offset-2 group-hover:underline">
          {featured.username}
        </div>
        <div className="text-ink-dim text-xs">
          {t("profile.partners.matchesTogether", { count: featured.played })}
          <span className="text-ink-off"> · </span>
          <span className="text-ink font-semibold tabular-nums">
            {winRate(featured.wins, featured.played)}%
          </span>
        </div>
      </div>
    </>
  );
}

/**
 * Sidebar panel highlighting the viewer's most-played teammate (featured) plus
 * a short list of other regular partners. Partner avatars use the gold "Us"
 * palette to read as the viewer's side of the table.
 */
export function PartnerSpotlight({ partners, subjectIsSelf = true }: PartnerSpotlightProps) {
  const { t } = useTranslation();

  const featured = partners[0];
  const rest = partners.slice(1);

  if (!featured) {
    return (
      <SidePanel
        eyebrow={t("profile.partners.eyebrow")}
        title={t("profile.partners.title")}
        testId="profile-partners"
      >
        <p className="text-ink-mute text-[13px]">
          {t(subjectIsSelf ? "profile.partners.empty" : "profile.partners.emptyPublic")}
        </p>
      </SidePanel>
    );
  }

  return (
    <SidePanel
      eyebrow={t("profile.partners.eyebrow")}
      title={t("profile.partners.title")}
      testId="profile-partners"
    >
      <div className="mb-3.5 flex flex-col gap-2">
        {/* Bots ARE excluded server-side (NULL seat ids never reach this list),
            but soft-deleted users are not: they arrive with a valid userId and
            an empty username. Linking them would 404, so the link is gated on
            a non-empty username. */}
        {featured.username !== "" ? (
          <Link
            to={`/players/${featured.userId}`}
            aria-label={t("friends.viewProfileAria", { username: featured.username })}
            className="group focus-visible:ring-ring/50 flex items-center gap-3.5 rounded-md focus-visible:ring-3 focus-visible:outline-none"
          >
            <FeaturedPartner featured={featured} />
          </Link>
        ) : (
          <div className="flex items-center gap-3.5">
            <FeaturedPartner featured={featured} />
          </div>
        )}
        <WinLoseBar winPct={winRate(featured.wins, featured.played)} />
      </div>

      {rest.length > 0 && (
        <ul className="m-0 flex list-none flex-col gap-1.5 p-0">
          {rest.map((p) => (
            <li
              key={p.userId}
              className="bg-surface-elevated border-border flex flex-col gap-1.5 rounded-[10px] border px-2.5 py-2"
            >
              <div className="flex items-center gap-2.5">
                {/* Same soft-deleted guard as the featured partner above. */}
                {p.username !== "" ? (
                  <Link
                    to={`/players/${p.userId}`}
                    aria-label={t("friends.viewProfileAria", { username: p.username })}
                    className="focus-visible:ring-ring/50 flex min-w-0 flex-1 items-center gap-2.5 rounded-md underline-offset-2 hover:underline focus-visible:ring-3 focus-visible:outline-none"
                  >
                    <Avatar name={p.username} team="A" size={24} />
                    <span className="text-ink truncate text-[13px] font-medium">{p.username}</span>
                  </Link>
                ) : (
                  <div className="flex min-w-0 flex-1 items-center gap-2.5">
                    <Avatar name={p.username} team="A" size={24} />
                    <span className="text-ink truncate text-[13px] font-medium">{p.username}</span>
                  </div>
                )}
                <span className="text-ink-dim text-[12px] tabular-nums">
                  {p.played}
                  <span className="text-ink-off"> · </span>
                  <span className="text-ink font-semibold">{winRate(p.wins, p.played)}%</span>
                </span>
              </div>
              <WinLoseBar winPct={winRate(p.wins, p.played)} />
            </li>
          ))}
        </ul>
      )}
    </SidePanel>
  );
}
