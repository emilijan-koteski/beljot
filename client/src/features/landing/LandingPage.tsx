import { ClosingCta } from "@/features/landing/components/ClosingCta";
import { FactStrip } from "@/features/landing/components/FactStrip";
import { LandingFooter } from "@/features/landing/components/LandingFooter";
import { LandingHero } from "@/features/landing/components/LandingHero";
import { LobbySection } from "@/features/landing/components/LobbySection";
import { RulesSection } from "@/features/landing/components/RulesSection";
import { SeasonsSection } from "@/features/landing/components/SeasonsSection";
import { TableSection } from "@/features/landing/components/TableSection";

/** Public full-bleed landing page at `/`, mounted directly under GuestRoute. */
export function LandingPage() {
  return (
    <div className="bg-background min-h-screen" data-testid="landing-page">
      <LandingHero />
      <FactStrip />
      <LobbySection />
      <RulesSection />
      <TableSection />
      <SeasonsSection />
      <ClosingCta />
      <LandingFooter />
    </div>
  );
}
