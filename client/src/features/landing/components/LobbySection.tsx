import { useTranslation } from "react-i18next";

import { BrowserFrame } from "@/features/landing/components/BrowserFrame";
import { PhoneFrame } from "@/features/landing/components/PhoneFrame";
import { ScreenShot } from "@/features/landing/components/ScreenShot";
import { SectionHeading } from "@/features/landing/components/SectionHeading";

/** The real lobby, on a phone for phones and in a browser window from tablet up. */
export function LobbySection() {
  const { t } = useTranslation();
  return (
    <section className="py-18 md:py-30" data-testid="landing-lobby">
      <div className="mx-auto grid max-w-310 items-center gap-9 px-4 md:px-7 lg:grid-cols-[400px_1fr] lg:gap-16">
        <SectionHeading
          eyebrow={t("landing.lobby.eyebrow")}
          title={t("landing.lobby.title")}
          body={t("landing.lobby.body")}
        />
        <div>
          <PhoneFrame className="mx-auto w-70 md:hidden">
            <ScreenShot name="lobby-phone" themed alt={t("landing.lobby.alt")} />
          </PhoneFrame>
          <BrowserFrame
            url="beljot.online/lobby"
            className="hidden origin-left md:block lg:[transform:perspective(1800px)_rotateY(-4deg)_rotateX(1deg)]"
          >
            <ScreenShot name="lobby-desktop" themed alt={t("landing.lobby.alt")} />
          </BrowserFrame>
        </div>
      </div>
    </section>
  );
}
