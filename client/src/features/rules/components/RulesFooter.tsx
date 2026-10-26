import { ArrowRight } from "lucide-react";
import { useNavigate } from "react-router";

import { useRules } from "@/features/rules/RulesContext";
import { Button } from "@/shared/components/ui/button";
import { useLobbyReturn } from "@/shared/hooks/useLobbyReturn";
import { useAuthStore } from "@/shared/stores/authStore";

/** "Ready for your first hand?" closer with a Play CTA back to the lobby. */
export function RulesFooter() {
  const { ui } = useRules();
  const returnToLobby = useLobbyReturn();
  const navigate = useNavigate();
  const isAuthed = useAuthStore((s) => s.token !== null);
  // Authed users pop/replace back to the lobby root. Guests keep the plain
  // push: ProtectedRoute bounces them to the landing page, and the push
  // preserves the rules page in history so back returns them to their reading.
  const handlePlay = () => {
    if (isAuthed) {
      returnToLobby();
    } else {
      void navigate("/lobby");
    }
  };
  return (
    <footer
      style={{
        marginTop: 36,
        padding: "24px 26px",
        background: "var(--surface)",
        border: "1px solid var(--border)",
        borderRadius: "var(--radius-lg)",
        display: "flex",
        alignItems: "center",
        gap: 18,
        flexWrap: "wrap",
      }}
    >
      <div style={{ display: "flex", flexDirection: "column", gap: 4, flex: 1, minWidth: 240 }}>
        <span className="" style={{ fontSize: 18, fontWeight: 600, color: "var(--ink)" }}>
          {ui.footerTitle}
        </span>
        <span style={{ fontSize: 14, color: "var(--ink-dim)" }}>{ui.footerBody}</span>
      </div>
      <Button size="cta" onClick={handlePlay} data-testid="rules-play-cta">
        {ui.footerCta}
        <ArrowRight data-icon="inline-end" strokeWidth={2.2} />
      </Button>
    </footer>
  );
}
