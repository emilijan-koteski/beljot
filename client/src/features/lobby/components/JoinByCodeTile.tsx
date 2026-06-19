import { KeyRound } from "lucide-react";
import { useState } from "react";
import { useTranslation } from "react-i18next";
import { useNavigate } from "react-router";
import { toast } from "sonner";

import { FetchError } from "@/shared/api/axiosClient";
import { useJoinByCodeMutation } from "@/shared/hooks/mutations/useRooms";
import { cn } from "@/shared/lib/utils";

const CODE_LENGTH = 6;

/**
 * 6-char uppercase code input + Join button inline. Replaces the older
 * <JoinByCode> sidebar tile; mirrors the design's hero action rail. Submits
 * on Enter and on button click.
 */
export function JoinByCodeTile() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const [code, setCode] = useState("");
  const joinByCodeMutation = useJoinByCodeMutation();
  const isValid = code.trim().length === CODE_LENGTH;

  async function submit() {
    const trimmed = code.trim().toUpperCase();
    if (trimmed.length !== CODE_LENGTH || joinByCodeMutation.isPending) return;

    try {
      const room = await joinByCodeMutation.mutateAsync(trimmed);
      navigate(`/rooms/${room.id}`);
    } catch (err) {
      const code = err instanceof FetchError ? err.code : null;
      if (code === "ROOM_NOT_FOUND") toast.error(t("lobby.errors.roomNotFound"));
      else if (code === "ROOM_FULL") toast.error(t("lobby.errors.roomFull"));
      else if (code === "INSUFFICIENT_COINS")
        // Join-by-code has no room object in scope, so it can't compose the
        // {{buyIn}}/{{balance}} message — use the param-less generic variant.
        toast.error(t("room.errors.insufficientCoinsGeneric"));
      else if (code === "ALREADY_IN_ROOM") toast.error(t("lobby.errors.alreadyInRoom"));
      else toast.error(t("lobby.errors.joinFailed"));
    }
  }

  return (
    <div
      className="bg-surface flex items-center gap-2.5 rounded-lg border border-border pr-2.5 pl-4.5"
      data-testid="join-by-code"
    >
      <KeyRound className="text-ink-dim size-4 shrink-0" strokeWidth={1.8} />
      <input
        value={code}
        onChange={(e) => setCode(e.target.value.toUpperCase().slice(0, CODE_LENGTH))}
        onKeyDown={(e) => e.key === "Enter" && submit()}
        placeholder={t("lobby.actions.joinByCode.placeholder")}
        maxLength={CODE_LENGTH}
        data-testid="join-by-code-input"
        className="text-ink min-w-0 flex-1 bg-transparent py-2.5 text-base font-semibold tracking-[2px] tabular-nums outline-none placeholder:font-normal placeholder:tracking-normal placeholder:text-ink-off"
      />
      <button
        onClick={submit}
        disabled={!isValid || joinByCodeMutation.isPending}
        data-testid="join-by-code-button"
        className={cn(
          "rounded-[10px] border border-transparent px-3.5 py-2 text-xs font-semibold transition-colors",
          isValid
            ? "bg-ink text-bg cursor-pointer"
            : "bg-surface-sunken text-ink-mute cursor-default opacity-80",
        )}
      >
        {t("lobby.actions.joinByCode.cta")}
      </button>
    </div>
  );
}
