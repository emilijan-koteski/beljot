import { Plus } from "lucide-react";
import { useTranslation } from "react-i18next";

type Props = { onClick: () => void };

/**
 * Neutral surface tile that opens the existing <CreateRoomModal />. Mirrors
 * the design's CreateRoomTile — surface-3 icon disc + title + subtitle, with
 * brass border + sunken surface on hover.
 */
export function CreateRoomTile({ onClick }: Props) {
  const { t } = useTranslation();
  return (
    <button
      onClick={onClick}
      data-testid="create-room-card"
      className="bg-surface text-ink hover:bg-surface-sunken hover:border-brass flex items-center gap-3 rounded-lg border border-border px-4.5 py-4.5 text-left transition-colors"
    >
      <span className="bg-surface-sunken text-ink inline-flex size-9 items-center justify-center rounded-md border border-brass">
        <Plus className="size-4.5" strokeWidth={2} />
      </span>
      <span className="flex flex-col gap-0.5">
        <span className="font-display text-sm font-semibold">
          {t("lobby.actions.createRoom.title")}
        </span>
        <span className="text-ink-dim text-xs">{t("lobby.actions.createRoom.subtitle")}</span>
      </span>
    </button>
  );
}
