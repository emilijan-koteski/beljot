import { ArrowRight } from "lucide-react";
import { useNavigate } from "react-router";

import { useRules } from "@/features/rules/RulesContext";
import { Button } from "@/shared/components/ui/button";
import { useLobbyReturn } from "@/shared/hooks/useLobbyReturn";
import { useAuthStore } from "@/shared/stores/authStore";

/**
 * "Ready for your first hand?" closer with a Play CTA back to the lobby. Signed-in players return to the
 * lobby root; guests push `/lobby` so the back button still returns them to the rules.
 */
export function RulesFooter() {
  const { ui } = useRules();
  const returnToLobby = useLobbyReturn();
  const navigate = useNavigate();
  const isAuthed = useAuthStore((s) => s.token !== null);
  const handlePlay = () => {
    if (isAuthed) {
      returnToLobby();
    } else {
      void navigate("/lobby");
    }
  };
  return (
    <footer className="bg-surface border-border mt-9 flex flex-wrap items-center gap-4.5 rounded-lg border px-6.5 py-6">
      <div className="flex min-w-60 flex-1 flex-col gap-1">
        <span className="text-ink text-lg font-semibold">{ui.footerTitle}</span>
        <span className="text-ink-dim text-sm">{ui.footerBody}</span>
      </div>
      <Button size="cta" onClick={handlePlay} data-testid="rules-play-cta">
        {ui.footerCta}
        <ArrowRight data-icon="inline-end" strokeWidth={2.2} />
      </Button>
    </footer>
  );
}
