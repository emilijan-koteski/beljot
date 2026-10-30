import { type ReactNode, useState } from "react";
import { useTranslation } from "react-i18next";

import { PageShell } from "@/shared/components/layout/PageShell";
import { Segmented } from "@/shared/components/ui/segmented";
import { useLayoutBand } from "@/shared/hooks/useLayoutBand";

type ProfileTab = "overview" | "matches" | "seasons";

type ProfileLayoutProps = {
  testId: string;
  hero: (layout: "phone" | "wide") => ReactNode;
  season: ReactNode;
  career: ReactNode;
  honor: ReactNode;
  highlights: ReactNode;
  partners: ReactNode;
  rivals: ReactNode;
  matches: ReactNode;
  seasons: (compact: boolean) => ReactNode;
};

/** The Overview, Matches and Seasons switch under the hero on phones and tablets. */
function ProfileTabs({
  tab,
  onTab,
  className,
}: {
  tab: ProfileTab;
  onTab: (tab: ProfileTab) => void;
  className?: string;
}) {
  const { t } = useTranslation();
  return (
    <Segmented<ProfileTab>
      value={tab}
      onValueChange={onTab}
      ariaLabel={t("profile.tabs.label")}
      testId="profile-tabs"
      className={className}
      options={[
        { value: "overview", label: t("profile.tabs.overview") },
        { value: "matches", label: t("profile.tabs.matches") },
        { value: "seasons", label: t("profile.tabs.seasons") },
      ]}
    />
  );
}

/**
 * Lays a profile out per width: tabs over one column on a phone, tabs over two columns on a tablet,
 * and everything at once on a desktop, with season, career and matches beside a sidebar of the rest.
 */
export function ProfileLayout({
  testId,
  hero,
  season,
  career,
  honor,
  highlights,
  partners,
  rivals,
  matches,
  seasons,
}: ProfileLayoutProps) {
  const band = useLayoutBand();
  const [tab, setTab] = useState<ProfileTab>("overview");

  if (band === "desktop") {
    return (
      <PageShell
        testId={testId}
        stickySidebar={false}
        sidebar={
          <>
            {honor}
            {highlights}
            {partners}
            {rivals}
            {seasons(true)}
          </>
        }
      >
        {hero("wide")}
        <div className="grid grid-cols-2 gap-4 *:h-full">
          {season}
          {career}
        </div>
        {matches}
      </PageShell>
    );
  }

  const tablet = band === "tablet";
  let body: ReactNode;
  if (tab === "matches") {
    body = matches;
  } else if (tab === "seasons") {
    body = seasons(false);
  } else if (tablet) {
    body = (
      <div className="grid grid-cols-2 items-start gap-4">
        <div className="flex flex-col gap-4">
          {season}
          {honor}
          {partners}
        </div>
        <div className="flex flex-col gap-4">
          {career}
          {highlights}
          {rivals}
        </div>
      </div>
    );
  } else {
    body = (
      <>
        {season}
        {career}
        {honor}
        {highlights}
        {partners}
        {rivals}
      </>
    );
  }

  return (
    <PageShell testId={testId}>
      {hero(tablet ? "wide" : "phone")}
      <ProfileTabs tab={tab} onTab={setTab} className={tablet ? "w-96" : undefined} />
      {body}
    </PageShell>
  );
}
