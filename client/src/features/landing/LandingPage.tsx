import { ClosingCta } from "@/features/landing/components/ClosingCta";
import { HowItWorks } from "@/features/landing/components/HowItWorks";
import { LandingFeatures } from "@/features/landing/components/LandingFeatures";
import { LandingFooter } from "@/features/landing/components/LandingFooter";
import { LandingHero } from "@/features/landing/components/LandingHero";

/** Public full-bleed marketing landing page at `/`, mounted directly under GuestRoute. */
export function LandingPage() {
  return (
    <div className="bg-background min-h-screen" data-testid="landing-page">
      <LandingHero />
      <HowItWorks />
      <LandingFeatures />
      <ClosingCta />
      <LandingFooter />
    </div>
  );
}
