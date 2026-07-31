import type { TFunction } from "i18next";
import {
  ArrowRight,
  Bot,
  Clock,
  Coins,
  KeyRound,
  Lock,
  LockOpen,
  UserCheck,
  Users,
  Zap,
} from "lucide-react";
import { useTranslation } from "react-i18next";

import { SeatChip } from "@/features/lobby/components/SeatChip";
import { modeLabel, variantLabel } from "@/features/lobby/lib/roomLabels";
import { HonorShield } from "@/shared/components/HonorShield";
import { RelativeTime } from "@/shared/components/RelativeTime";
import { botDisplayName } from "@/shared/lib/botName";
import { COIN_GOLD } from "@/shared/lib/coinGold";
import { formatCoins } from "@/shared/lib/formatCoins";
import {
  honorFloorLabel,
  honorQualifies,
  honorScoreOrPrior,
  honorTierForScore,
} from "@/shared/lib/honor";
import { cn } from "@/shared/lib/utils";
import { useAuthStore } from "@/shared/stores/authStore";
import type { Room, RoomPlayer } from "@/shared/types/apiTypes";

type Props = {
  room: Room;
  onJoin: (room: Room) => void;
  /** 0-based render index for the staggered card-in animation. */
  index?: number;
};

function seatOf(
  t: TFunction,
  players: RoomPlayer[] | undefined,
  seat: number,
): { username: string | null; bot: boolean } {
  const found = players?.find((p) => p.seat === seat);
  if (!found) return { username: null, bot: false };
  // Bot identity is seat-derived and localized — an empty wire username must
  // never render as a blank chip.
  if (found.isBot === true) return { username: botDisplayName(t, seat), bot: true };
  return { username: found.username || null, bot: false };
}

/**
 * Single lobby room tile. Title row + meta row (variant · mode · timer ·
 * relative age) + 2×2 seat preview + footer (host · occupancy · Join).
 */
export function RoomCard({ room, onJoin, index = 0 }: Props) {
  const { t } = useTranslation();
  const seated = room.playerCount;
  const full = seated >= 4;
  const hasBots = room.players?.some((p) => p.isBot === true) ?? false;
  const delay = `${Math.min(index * 28, 320)}ms`;

  // Honour gate, cosmetic mirror of the server's. `honorQualifies` treats absent
  // room fields as ungated, which is what the QuickPlay system:room_created
  // payload needs (it is hand-built and omits both keys, and a synthesized
  // quick-play room genuinely is ungated).
  const viewer = useAuthStore((s) => s.user);
  const viewerHonor = honorScoreOrPrior(viewer?.honorScore);
  const qualifies = honorQualifies(room, viewer ?? {});
  const locked = !qualifies;
  // The shield's tone describes the REQUIREMENT's tier, not the viewer's.
  const requirementTier = honorTierForScore(room.minHonor);

  return (
    <article
      data-testid="room-card"
      style={{ animationDelay: delay }}
      className={cn(
        "bg-surface text-ink relative flex flex-col overflow-hidden rounded-lg border border-border transition-[transform,border-color,box-shadow]",
        "animate-[card-in_.35s_ease_both] hover:-translate-y-0.5 hover:border-border-2 hover:shadow-[0_18px_40px_-22px_rgba(14,58,36,0.30)]",
      )}
    >
      <div className="px-5 pt-4.5 pb-3">
        <div className="flex items-center gap-2.5">
          <span
            aria-hidden
            className={cn(
              "size-2 rounded-full",
              full ? "bg-warning" : "bg-accent",
              "animate-[pulse-dot_1.8s_ease-in-out_infinite]",
            )}
          />
          <h3 className="font-display text-ink m-0 flex-1 min-w-0 truncate text-base font-semibold">
            {room.name}
          </h3>
          {room.isQuickPlay && (
            <span
              data-testid="quick-play-badge"
              className="border-accent/30 bg-accent-soft text-accent-deep inline-flex shrink-0 items-center gap-1 rounded-full border px-2 py-0.5 text-[10px] font-bold tracking-[0.6px] uppercase"
            >
              <Zap className="size-2.5" strokeWidth={2.4} />
              {t("lobby.card.quickPlay")}
            </span>
          )}
          {hasBots && (
            <span
              data-testid="room-card-bots"
              className="border-bot-edge bg-bot-soft text-bot-deep inline-flex shrink-0 items-center gap-1 rounded-full border px-2 py-0.5 text-[10px] font-bold tracking-[0.6px] uppercase"
            >
              <Bot className="size-2.5" strokeWidth={2.4} />
              {t("bots.withBots")}
            </span>
          )}
          <CodeChip code={room.code} />
        </div>

        <div className="text-ink-dim mt-1.5 flex flex-wrap items-center gap-2 text-xs">
          <span>
            {variantLabel(t, room.variant)} · {modeLabel(t, room.matchMode)}
          </span>
          <Dot />
          <span className="inline-flex items-center gap-1">
            <Clock className="size-3" />
            {room.timerStyle === "relaxed"
              ? t("lobby.card.relaxed")
              : t("lobby.card.timerSeconds", { seconds: room.timerDurationSeconds })}
          </span>
          <Dot />
          <span className="inline-flex items-center gap-1" data-testid="room-card-buy-in">
            <Coins className="size-3" style={{ color: COIN_GOLD }} />
            {/* Icon-only unit — the coin glyph conveys "coins", so the card
                shows just the grouped number (or "Free" at 0). */}
            {room.coinBuyIn > 0 ? formatCoins(room.coinBuyIn) : t("lobby.card.buyInFree")}
          </span>
          <Dot />
          {room.isPrivate ? (
            <span
              className="inline-flex items-center gap-1"
              data-testid="room-card-lock"
              aria-label={t("lobby.card.privateLockAriaLabel")}
            >
              <Lock className="size-3" />
              {t("lobby.card.private")}
            </span>
          ) : (
            <span
              className="inline-flex items-center gap-1"
              data-testid="room-card-public"
              aria-label={t("lobby.card.publicAriaLabel")}
            >
              <LockOpen className="size-3" />
              {t("lobby.card.public")}
            </span>
          )}
          {/* Honor gate (Story 9.8 AC5). Both chips are CONDITIONAL: an ungated
              room's card is visually unchanged from before this story, and every
              ungated card would otherwise grow two chips carrying no information.
              Gated rooms stay LISTED and labelled — mirroring 9.6's "listed but
              locked" decision, the player sees the requirement and decides.
              Compared explicitly (> 0, === false), never by truthiness: a real 0
              and a real false are legitimate Go values. Each chip carries text
              plus a glyph, so colour is never the only signal. */}
          {room.minHonor > 0 && (
            <>
              <Dot />
              <span
                className="inline-flex items-center gap-1"
                data-testid="room-card-min-honor"
                data-min-honor={room.minHonor}
                aria-label={t("lobby.card.minHonorAriaLabel", { minHonor: room.minHonor })}
                // The numbers live in the tooltip, never as copy on the card — the
                // card says what the ROOM asks for; the button says whether you
                // pass. A viewer-specific sentence on every card would be noise on
                // the many they do qualify for.
                title={
                  qualifies
                    ? undefined
                    : t("lobby.card.minHonorLockedTitle", {
                        minHonor: room.minHonor,
                        honor: viewerHonor,
                      })
                }
              >
                {/* Tinted by the tier the REQUIREMENT falls in, not by the viewer's
                    standing: 85+ is Trusted, so this shield is felt green on every
                    card whether you hold 96 or 42. That keeps the chip a property
                    of the room, and stops the lobby looking like a scoreboard of
                    the viewer. The "85+" itself stays the same muted ink as the
                    timer and buy-in beside it, so the chip weighs what its
                    neighbours weigh. */}
                <HonorShield tier={requirementTier} size={12} />
                {t("lobby.card.minHonor", { minHonor: honorFloorLabel(room.minHonor) })}
              </span>
            </>
          )}
          {room.allowNewPlayers === false && (
            <>
              <Dot />
              <span
                className="inline-flex items-center gap-1"
                data-testid="room-card-veterans-only"
                aria-label={t("lobby.card.veteransOnlyAriaLabel")}
              >
                <UserCheck className="size-3" />
                {t("lobby.card.veteransOnly")}
              </span>
            </>
          )}
          <Dot />
          <RelativeTime iso={room.createdAt} variant="compact" />
        </div>
      </div>

      <div className="grid grid-cols-[auto_1fr_1fr] items-center gap-1.5 px-5 pb-3.5">
        <TeamLabel team="A" />
        <SeatChip {...seatOf(t, room.players, 0)} team="A" testId={`room-${room.id}-seat-0`} />
        <SeatChip {...seatOf(t, room.players, 2)} team="A" testId={`room-${room.id}-seat-2`} />
        <TeamLabel team="B" />
        <SeatChip {...seatOf(t, room.players, 1)} team="B" testId={`room-${room.id}-seat-1`} />
        <SeatChip {...seatOf(t, room.players, 3)} team="B" testId={`room-${room.id}-seat-3`} />
      </div>

      <div className="mt-auto flex items-center gap-2.5 border-t border-border bg-[rgba(14,58,36,0.03)] px-5 py-3">
        <span className="text-ink-dim flex items-center gap-1.5 text-xs">
          <span className="bg-surface-sunken text-ink inline-flex size-4.5 items-center justify-center rounded-full border border-border text-[10px] font-bold">
            {(room.ownerUsername || "?").charAt(0).toUpperCase()}
          </span>
          {room.ownerUsername || "—"}
        </span>
        <span className="text-ink-mute inline-flex items-center gap-1 text-xs">
          <Users className="size-3" />
          {t("lobby.card.occupancy", { seated })}
        </span>
        {/* The gate's verdict lives HERE, not on the card. Not qualifying changes
            exactly one thing: Join becomes Locked. The numbers sit in the honour
            chip's title and in the toast that still fires on a genuine race.

            Still cosmetic — the server re-validates every join, so this only
            spares the player a click that was always going to 409. `full` wins
            over `locked` because a full room cannot be joined for any reason. */}
        <button
          onClick={() => !full && !locked && onJoin(room)}
          disabled={full || locked}
          data-testid="room-card-join"
          data-locked={locked ? "true" : undefined}
          title={
            locked
              ? t("lobby.card.minHonorLockedTitle", {
                  minHonor: room.minHonor,
                  honor: viewerHonor,
                })
              : undefined
          }
          className={cn(
            "ml-auto inline-flex items-center gap-1.5 rounded-[10px] border border-transparent px-3.5 py-2 text-xs font-semibold transition-transform active:scale-[0.97]",
            full || locked
              ? "bg-surface-sunken text-ink-mute cursor-default"
              : "bg-accent text-accent-ink cursor-pointer",
          )}
        >
          {full
            ? t("lobby.card.full")
            : locked
              ? t("lobby.card.locked")
              : room.isQuickPlay
                ? t("lobby.card.joinQueue")
                : t("lobby.card.join")}
          {locked && <Lock className="size-3.5" strokeWidth={2.2} />}
          {!full &&
            !locked &&
            (room.isQuickPlay ? (
              <Zap className="size-3.5" strokeWidth={2.4} />
            ) : (
              <ArrowRight className="size-3.5" strokeWidth={2.2} />
            ))}
        </button>
      </div>
    </article>
  );
}

function TeamLabel({ team }: { team: "A" | "B" }) {
  return (
    <span
      className={cn(
        "pr-1 text-[10px] font-bold uppercase tracking-[1.2px]",
        team === "A" ? "text-team-a" : "text-team-b",
      )}
    >
      {team}
    </span>
  );
}

function CodeChip({ code }: { code: string }) {
  return (
    <span className="bg-surface-sunken text-ink-dim inline-flex items-center gap-1 rounded-md border border-border px-2 py-0.5 text-[11px] font-semibold tabular-nums tracking-[1.5px]">
      <KeyRound className="size-2.5" />
      {code}
    </span>
  );
}

function Dot() {
  return <span className="text-ink-off text-[10px]">·</span>;
}
