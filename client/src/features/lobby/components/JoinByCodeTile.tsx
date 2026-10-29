import { Hash } from "lucide-react";
import { useState } from "react";
import { useTranslation } from "react-i18next";

import { PasswordPromptDialog } from "@/features/lobby/components/PasswordPromptDialog";
import {
  normalizeRoomCode,
  ROOM_CODE_LENGTH,
  useJoinByCode,
} from "@/features/lobby/hooks/useJoinByCode";
import { actionTileClasses } from "@/shared/components/ui/action-tile";
import { Button } from "@/shared/components/ui/button";
import { cn } from "@/shared/lib/utils";

export const ROOM_CODE_INLINE =
  "text-ink placeholder:text-ink-mute min-w-0 bg-transparent font-mono text-sm font-semibold tracking-[0.3em] uppercase outline-none placeholder:font-sans placeholder:font-normal placeholder:tracking-normal placeholder:normal-case";

/**
 * The desktop "Have a code?" tile: the room code is typed in place and Join wakes up at six characters,
 * submitting on Enter too. A private room asks for its password before joining.
 */
export function JoinByCodeTile() {
  const { t } = useTranslation();
  const [code, setCode] = useState("");
  const join = useJoinByCode();
  const ready = code.length === ROOM_CODE_LENGTH;

  return (
    <div
      className={cn(
        actionTileClasses("quiet"),
        "has-[input:focus-visible]:border-ring has-[input:focus-visible]:ring-3 has-[input:focus-visible]:ring-ring/70",
      )}
      data-testid="join-by-code"
    >
      <Hash className="text-brass-deep" aria-hidden="true" />
      <label className="flex min-w-0 flex-1 cursor-text flex-col leading-tight">
        <span className="text-ink-mute text-2xs">{t("lobby.actions.joinByCode.title")}</span>
        <input
          value={code}
          onChange={(e) => setCode(normalizeRoomCode(e.target.value))}
          onKeyDown={(e) => e.key === "Enter" && void join.submit(code)}
          maxLength={ROOM_CODE_LENGTH}
          autoComplete="off"
          spellCheck={false}
          placeholder={t("lobby.actions.joinByCode.placeholder")}
          aria-label={t("lobby.actions.joinByCode.inputLabel")}
          data-testid="join-by-code-input"
          className={ROOM_CODE_INLINE}
        />
      </label>
      <Button
        size="sm"
        onClick={() => void join.submit(code)}
        disabled={!ready || join.busy}
        data-testid="join-by-code-button"
      >
        {t("lobby.actions.joinByCode.cta")}
      </Button>
      <PasswordPromptDialog {...join.passwordPrompt} />
    </div>
  );
}
