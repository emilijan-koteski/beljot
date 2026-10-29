import { useTranslation } from "react-i18next";

import { RoomCard } from "@/features/lobby/components/RoomCard";
import { type RoomFormValues, roomFromForm } from "@/features/room/lib/roomFromForm";
import { RoomRulesContent, RULES_PANEL } from "@/shared/components/roomRules/RoomRules";
import { POPUP_SURFACE } from "@/shared/lib/popupSurface";
import { cn } from "@/shared/lib/utils";

/**
 * The create-room dialog's lobby preview: the lobby's own room card, live with the form, and under it
 * the Rules exactly as players read them when they hover the card's rule icons.
 */
export function CreateRoomPreview(props: RoomFormValues) {
  const { t } = useTranslation();
  const room = roomFromForm(props);
  return (
    <div data-testid="create-room-preview" className="flex flex-col items-center gap-5">
      <div className="w-56">
        <RoomCard room={room} preview />
      </div>
      <div className="flex w-70 flex-col gap-1.5">
        <span className="text-ink-mute text-2xs">{t("roomRules.previewCaption")}</span>
        <div className={cn(POPUP_SURFACE, RULES_PANEL)} data-testid="create-room-preview-rules">
          <RoomRulesContent room={room} />
        </div>
      </div>
    </div>
  );
}
