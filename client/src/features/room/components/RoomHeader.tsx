import { ChevronDown, Coins, Crown, DoorOpen } from "lucide-react";
import type { ReactNode } from "react";
import { Trans, useTranslation } from "react-i18next";

import { HonorShield } from "@/shared/components/HonorShield";
import { RULE_ICONS } from "@/shared/components/roomRules/roomRuleLines";
import { RoomRulesPopover } from "@/shared/components/roomRules/RoomRules";
import { Button } from "@/shared/components/ui/button";
import { CodeChip } from "@/shared/components/ui/code-chip";
import { COIN_GOLD } from "@/shared/lib/coinGold";
import { formatCoins } from "@/shared/lib/formatCoins";
import { honorFloorLabel, honorTierForScore } from "@/shared/lib/honor";
import { modeLabel, variantLabel } from "@/shared/lib/roomLabels";
import { cn } from "@/shared/lib/utils";
import type { Room, RoomPlayer } from "@/shared/types/apiTypes";

interface RoomHeaderProps {
  room: Room;
  players: RoomPlayer[];
  ownerName: string;
  canEditPrivacy: boolean;
  copied: boolean;
  onCopy: () => void;
  onPrivacy: () => void;
}

const CARD = "bg-surface border-border flex flex-col rounded-xl border shadow-xs";

/** The room's settings in one line: variant, target, move time, buy-in and the honor floor. */
function SpecStrip({ room, size }: { room: Room; size: "sm" | "md" }) {
  const { t } = useTranslation();
  const relaxed = room.timerStyle === "relaxed";
  const TimerIcon = relaxed ? RULE_ICONS.relaxedTimer : RULE_ICONS.moveTimer;
  const items: Array<{ key: string; node: ReactNode }> = [
    {
      key: "variant",
      node: <span data-testid="badge-variant">{variantLabel(t, room.variant)}</span>,
    },
    {
      key: "mode",
      node: <span data-testid="badge-match-mode">{modeLabel(t, room.matchMode)}</span>,
    },
    {
      key: "timer",
      node: (
        <span className="inline-flex items-center gap-1" data-testid="badge-timer">
          <TimerIcon className="text-ink-mute size-3.5" aria-hidden="true" />
          {relaxed
            ? t("lobby.card.relaxed")
            : t("lobby.card.timerShort", { seconds: room.timerDurationSeconds ?? 0 })}
        </span>
      ),
    },
    {
      key: "buyIn",
      node: (
        <span className="inline-flex items-center gap-1" data-testid="badge-buy-in">
          <Coins className="size-3.5" style={{ color: COIN_GOLD }} aria-hidden="true" />
          {room.coinBuyIn > 0 ? formatCoins(room.coinBuyIn) : t("lobby.card.free")}
        </span>
      ),
    },
  ];
  if (room.minHonor > 0) {
    items.push({
      key: "honor",
      node: (
        <span
          className="inline-flex items-center gap-1"
          data-testid="badge-min-honor"
          data-min-honor={room.minHonor}
        >
          <HonorShield tier={honorTierForScore(room.minHonor)} size={12} />
          {honorFloorLabel(room.minHonor)}
        </span>
      ),
    });
  }
  return (
    <div
      className={cn(
        "text-ink flex min-w-0 items-center overflow-x-auto font-semibold whitespace-nowrap tabular-nums [scrollbar-width:none]",
        size === "sm" ? "gap-2.5 text-xs" : "gap-3.5 text-sm",
      )}
      data-testid="room-info-badges"
    >
      {items.map((item, i) => (
        <span key={item.key} className="inline-flex shrink-0 items-center gap-2.5">
          {i > 0 && <span className="bg-border h-3.5 w-px" aria-hidden="true" />}
          {item.node}
        </span>
      ))}
    </div>
  );
}

function PrivacyMark({
  room,
  canEdit,
  onPrivacy,
}: {
  room: Room;
  canEdit: boolean;
  onPrivacy: () => void;
}) {
  const { t } = useTranslation();
  const Icon = room.isPrivate ? RULE_ICONS.private : RULE_ICONS.public;
  const label = room.isPrivate ? t("room.privacy.privateBadge") : t("room.privacy.publicBadge");
  if (canEdit) {
    return (
      <Button
        type="button"
        variant="outline"
        size="icon-sm"
        onClick={onPrivacy}
        aria-label={t("room.privacy.dialogTitle")}
        title={label}
        data-testid="room-privacy-control"
        className="shrink-0"
      >
        <Icon className="size-3.5" />
      </Button>
    );
  }
  if (!room.isPrivate) return null;
  return <Icon className="text-ink-mute size-4 shrink-0" role="img" aria-label={label} />;
}

function HostLine({ ownerName }: { ownerName: string }) {
  return (
    <span className="inline-flex min-w-0 items-center gap-1.5">
      <Crown className="text-brass-deep size-3 shrink-0" aria-hidden="true" />
      <span className="truncate">
        <Trans
          i18nKey="room.ownerLine"
          values={{ owner: ownerName }}
          components={{ name: <span className="text-ink font-semibold" /> }}
        />
      </span>
    </span>
  );
}

function RoomCode({
  room,
  copied,
  onCopy,
  large,
}: Pick<RoomHeaderProps, "room" | "copied" | "onCopy"> & { large: boolean }) {
  const { t } = useTranslation();
  return (
    <CodeChip
      code={room.code}
      variant="compact"
      copied={copied}
      onCopy={onCopy}
      ariaLabel={t("room.copyLinkAriaLabel", { code: room.code })}
      testId="copy-link"
      codeTestId="room-code"
      className={large ? "px-3 py-2 text-sm" : "px-2.5 py-1 text-xs"}
    />
  );
}

/** The phone's room card: name and code, the host with the room count that opens the roster, then the settings. */
export function PhoneRoomCard({
  inRoom,
  onRoster,
  ...props
}: RoomHeaderProps & { inRoom: number; onRoster: () => void }) {
  const { t } = useTranslation();
  const { room, players } = props;
  return (
    <section className={cn(CARD, "gap-2.5 p-3")} data-testid="room-info-card">
      <div className="flex min-w-0 items-center gap-2">
        <PrivacyMark room={room} canEdit={props.canEditPrivacy} onPrivacy={props.onPrivacy} />
        <h1
          className="text-ink m-0 min-w-0 flex-1 truncate text-base font-semibold"
          data-testid="room-info-name"
        >
          {room.name}
        </h1>
        <RoomCode room={room} copied={props.copied} onCopy={props.onCopy} large={false} />
      </div>
      <div className="text-ink-dim flex min-w-0 items-center gap-1.5 text-xs">
        <HostLine ownerName={props.ownerName} />
        <span className="text-faint" aria-hidden="true">
          ·
        </span>
        <button
          type="button"
          onClick={onRoster}
          aria-label={t("room.roster.title")}
          data-testid="in-room-count"
          className="text-ink-dim hover:text-ink inline-flex shrink-0 cursor-pointer items-center gap-0.5 font-medium"
        >
          {t("room.inRoomCount", { count: inRoom })}
          <ChevronDown className="size-3" aria-hidden="true" />
        </button>
        <RoomRulesPopover
          room={{ ...room, players }}
          live={room.status !== "waiting"}
          className="mr-0 ml-auto"
        />
      </div>
      <div className="border-border border-t pt-2.5">
        <SpecStrip room={room} size="sm" />
      </div>
    </section>
  );
}

/** The tablet and desktop room header: name, code and Leave, then the host and the settings line. */
export function WideRoomHeader({ onLeave, ...props }: RoomHeaderProps & { onLeave: () => void }) {
  const { t } = useTranslation();
  const { room, players } = props;
  return (
    <section className={cn(CARD, "gap-3 px-5 py-4")} data-testid="room-info-card">
      <div className="flex min-w-0 items-center gap-3">
        <PrivacyMark room={room} canEdit={props.canEditPrivacy} onPrivacy={props.onPrivacy} />
        <h1
          className="text-ink m-0 min-w-0 flex-1 truncate text-2xl font-semibold tracking-tight"
          data-testid="room-info-name"
        >
          {room.name}
        </h1>
        <RoomCode room={room} copied={props.copied} onCopy={props.onCopy} large />
        <Button
          variant="ghost"
          onClick={onLeave}
          data-testid="leave-room"
          className="text-danger hover:text-danger shrink-0"
        >
          <DoorOpen className="size-4" />
          {t("room.leaveRoom")}
        </Button>
      </div>
      <div className="text-ink-dim flex min-w-0 flex-wrap items-center justify-between gap-x-4 gap-y-2 text-sm">
        <HostLine ownerName={props.ownerName} />
        <div className="flex min-w-0 items-center gap-3.5">
          <SpecStrip room={room} size="md" />
          <span className="bg-border h-3.5 w-px shrink-0" aria-hidden="true" />
          <RoomRulesPopover
            room={{ ...room, players }}
            live={room.status !== "waiting"}
            className="mr-0"
          />
        </div>
      </div>
    </section>
  );
}
