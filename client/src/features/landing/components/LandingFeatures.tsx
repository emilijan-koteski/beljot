import { useTranslation } from "react-i18next";

import { FeatureRow } from "@/features/landing/components/FeatureRow";
import { FeltVignette } from "@/features/landing/components/FeltVignette";
import { ProfileCard } from "@/features/landing/components/ProfileCard";
import { RoomCard } from "@/features/landing/components/RoomCard";

// Three alternating feature rows: the live lobby, the felt table, and the player profile.
export function LandingFeatures() {
  const { t } = useTranslation();

  const room = { joinLabel: t("landing.room.join"), emptyLabel: t("landing.room.empty") };
  const vig = {
    trump: t("landing.vig.trump"),
    opponent: t("landing.vig.opponent"),
    partner: t("landing.vig.partner"),
  };
  const prof = {
    member: t("landing.prof.member"),
    games: t("landing.prof.games"),
    winrate: t("landing.prof.winrate"),
    wins: t("landing.prof.wins"),
    recent: t("landing.prof.recent"),
    won: t("landing.prof.won"),
    lost: t("landing.prof.lost"),
    with: t("landing.prof.with"),
  };

  return (
    <section id="features" className="bg-surface border-border border-y py-[clamp(60px,9vw,92px)]">
      <div className="mx-auto flex max-w-7xl flex-col gap-[clamp(56px,8vw,84px)] px-[clamp(28px,5vw,72px)]">
        <FeatureRow
          reverse
          tag={t("landing.feat.f1tag")}
          title={t("landing.feat.f1title")}
          body={t("landing.feat.f1body")}
        >
          <div className="flex w-full max-w-90 flex-col gap-3.5">
            <RoomCard
              width={360}
              title={t("landing.room.r1title")}
              meta={t("landing.room.r1meta")}
              ago={t("landing.room.r1ago")}
              {...room}
            />
            <RoomCard
              filling
              width={360}
              code="K9D2M1"
              host="cvetanka"
              title={t("landing.room.r2title")}
              meta={t("landing.room.r2meta")}
              ago={t("landing.room.r2ago")}
              seats={[
                ["C", "cvetanka", "a"],
                [null, "", "a"],
                ["A", "ana", "b"],
                [null, "", "b"],
              ]}
              {...room}
            />
          </div>
        </FeatureRow>

        <FeatureRow
          tag={t("landing.feat.f2tag")}
          title={t("landing.feat.f2title")}
          body={t("landing.feat.f2body")}
        >
          <FeltVignette width={480} height={330} labels={vig} />
        </FeatureRow>

        <FeatureRow
          reverse
          tag={t("landing.feat.f3tag")}
          title={t("landing.feat.f3title")}
          body={t("landing.feat.f3body")}
        >
          <ProfileCard labels={prof} />
        </FeatureRow>
      </div>
    </section>
  );
}
