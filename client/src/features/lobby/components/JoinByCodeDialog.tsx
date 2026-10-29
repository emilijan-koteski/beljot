import { Hash } from "lucide-react";
import { useState } from "react";
import { useTranslation } from "react-i18next";

import { PasswordPromptDialog } from "@/features/lobby/components/PasswordPromptDialog";
import {
  normalizeRoomCode,
  ROOM_CODE_LENGTH,
  useJoinByCode,
} from "@/features/lobby/hooks/useJoinByCode";
import { AppDialog } from "@/shared/components/ui/app-dialog";
import { Input } from "@/shared/components/ui/input";

const ROOM_CODE_LARGE =
  "h-12 text-center font-mono text-xl font-semibold tracking-[0.4em] uppercase";

interface JoinByCodeDialogProps {
  open: boolean;
  onClose: () => void;
}

/**
 * The "Have a code?" sheet on phones and tablets: one large code field, and Join wakes up at six characters.
 * A private room closes the sheet and asks for its password.
 */
export function JoinByCodeDialog({ open, onClose }: JoinByCodeDialogProps) {
  const { t } = useTranslation();
  const [code, setCode] = useState("");
  const join = useJoinByCode();
  const ready = code.length === ROOM_CODE_LENGTH;

  function close() {
    setCode("");
    onClose();
  }

  async function submit() {
    if (await join.submit(code)) close();
  }

  return (
    <>
      <AppDialog
        open={open}
        onClose={close}
        icon={<Hash />}
        title={t("lobby.actions.joinByCode.dialogTitle")}
        description={t("lobby.actions.joinByCode.dialogDescription")}
        pending={join.busy}
        primary={{
          label: t("lobby.actions.joinByCode.cta"),
          disabled: !ready,
          testId: "join-by-code-button",
        }}
        secondary={{ label: t("lobby.actions.joinByCode.cancel"), onClick: close }}
        onSubmit={() => void submit()}
        testId="join-by-code-dialog"
      >
        <Input
          value={code}
          onChange={(e) => setCode(normalizeRoomCode(e.target.value))}
          maxLength={ROOM_CODE_LENGTH}
          autoComplete="off"
          spellCheck={false}
          placeholder={t("lobby.actions.joinByCode.codePlaceholder")}
          aria-label={t("lobby.actions.joinByCode.inputLabel")}
          data-testid="join-by-code-input"
          className={ROOM_CODE_LARGE}
        />
      </AppDialog>
      <PasswordPromptDialog {...join.passwordPrompt} />
    </>
  );
}
