import { ArrowRight } from "lucide-react";
import { useNavigate } from "react-router";

import { useRules } from "@/features/rules/RulesContext";
import { Button } from "@/shared/components/ui/button";
import { useLobbyReturn } from "@/shared/hooks/useLobbyReturn";
import { useAuthStore } from "@/shared/stores/authStore";

/**
 * "Ready to play?" closer with a Play CTA back to the lobby. Signed-in players return to the lobby root;
 * guests push `/lobby` so the back button still returns them to the rules.
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
    <footer className="bg-surface border-border flex items-center gap-4 rounded-xl border px-4 py-3.5 shadow-xs">
      <div className="flex min-w-0 flex-1 flex-col gap-0.5">
        <span className="text-ink text-base font-semibold">{ui.footerTitle}</span>
        <span className="text-ink-mute text-xs">{ui.footerBody}</span>
      </div>
      <Button onClick={handlePlay} data-testid="rules-play-cta">
        {ui.footerCta}
        <ArrowRight data-icon="inline-end" />
      </Button>
    </footer>
  );
}
