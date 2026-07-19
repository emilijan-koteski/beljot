import { ArrowRight } from "lucide-react";
import { useNavigate } from "react-router";

import { useRules } from "@/features/rules/RulesContext";
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
        <span
          className="font-display"
          style={{ fontSize: 18, fontWeight: 600, color: "var(--ink)" }}
        >
          {ui.footerTitle}
        </span>
        <span style={{ fontSize: 13.5, color: "var(--ink-dim)" }}>{ui.footerBody}</span>
      </div>
      <button
        type="button"
        onClick={handlePlay}
        data-testid="rules-play-cta"
        className="inline-flex cursor-pointer items-center gap-2 rounded-[10px] border border-transparent px-4.5 py-2.75 text-sm font-semibold transition-transform active:scale-[0.98]"
        style={{
          background: "var(--accent)",
          color: "var(--accent-ink)",
          boxShadow: "0 8px 22px -10px rgba(25,101,54,0.55)",
        }}
      >
        {ui.footerCta}
        <ArrowRight className="size-4" strokeWidth={2.2} />
      </button>
    </footer>
  );
}
