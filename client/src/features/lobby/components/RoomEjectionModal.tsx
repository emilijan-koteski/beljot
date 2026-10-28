import { Coins, DoorOpen, ShieldAlert } from "lucide-react";
import { useTranslation } from "react-i18next";
import { useNavigate } from "react-router";

import { AppDialog } from "@/shared/components/ui/app-dialog";
import { formatCoins } from "@/shared/lib/formatCoins";
import { useRoomStore } from "@/shared/stores/roomStore";

/**
 * Lobby notice explaining why the player was removed from a room, read from roomStore.roomEjection.
 * Copy branches on `reason`: insolvency, honor below the room's minimum, or room closed.
 */
export function RoomEjectionModal() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const ejection = useRoomStore((s) => s.roomEjection);
  const setRoomEjection = useRoomStore((s) => s.setRoomEjection);

  const roomClosed = ejection?.reason === "roomClosed";
  const honor = ejection?.reason === "honor";

  const close = () => setRoomEjection(null);
  const gotIt = {
    label: t("lobby.roomEjection.action"),
    onClick: close,
    testId: "room-ejection-action",
  };

  const icon = roomClosed ? <DoorOpen /> : honor ? <ShieldAlert /> : <Coins />;
  const title = roomClosed
    ? t("lobby.roomEjection.roomClosedTitle")
    : honor
      ? t("lobby.roomEjection.honorTitle")
      : t("lobby.roomEjection.insolventTitle");
  const body = roomClosed
    ? t("lobby.roomEjection.roomClosedBody")
    : honor
      ? t("lobby.roomEjection.honorBody", {
          minHonor: ejection?.minHonor ?? 0,
          honor: ejection?.honor ?? 0,
        })
      : t("lobby.roomEjection.insolventBody", {
          balance: formatCoins(ejection?.balance ?? 0),
          buyIn: formatCoins(ejection?.buyIn ?? 0),
        });

  return (
    <AppDialog
      open={ejection !== null}
      onClose={close}
      icon={icon}
      title={<span data-testid="room-ejection-title">{title}</span>}
      description={
        <span
          data-testid="room-ejection-body"
          data-min-honor={honor ? ejection?.minHonor : undefined}
          data-honor={honor ? ejection?.honor : undefined}
        >
          {body}
        </span>
      }
      primary={
        honor
          ? {
              label: t("lobby.roomEjection.browseQualifying"),
              onClick: () => {
                close();
                navigate("/lobby", { state: { lobbyFilter: "qualify" } });
              },
              testId: "room-ejection-browse",
            }
          : gotIt
      }
      secondary={honor ? gotIt : undefined}
      testId="room-ejection-modal"
    />
  );
}
