import { Clock, Coins, Trophy, X } from "lucide-react";
import { useTranslation } from "react-i18next";

import { modeLabel, variantLabel } from "@/features/lobby/lib/roomLabels";
import { Avatar } from "@/shared/components/ui/avatar";
import { Badge } from "@/shared/components/ui/badge";
import { Button } from "@/shared/components/ui/button";
import { Eyebrow } from "@/shared/components/ui/eyebrow";
import { COIN_GOLD } from "@/shared/lib/coinGold";
import { formatCoins } from "@/shared/lib/formatCoins";
import type { Room, RoomPlayer } from "@/shared/types/apiTypes";

type Props = {
  /** The room being matched — drives the variant / mode / timer / stake chips. */
  room: Room;
  /** Number of seated players (1..4) — the viewer is always one of them. */
  found: number;
  /** All players currently in the room (seated and not). */
  players: RoomPlayer[];
  /** The viewer's seat index (0..3). */
  viewerSeat: number;
  /** The viewer's username (for the centre avatar). */
  currentUsername: string;
  /** Elapsed wait time, pre-formatted as mm:ss. */
  elapsed: string;
  onCancel: () => void;
  cancelDisabled?: boolean;
};

// Orbit slot geometry — west, north, east around the centred "You". Offsets are
// percentages of the (square) orbit box so the whole diagram scales down with
// the box on narrow screens; at the 380px design size these resolve to ±130px,
// matching MatchmakingSearching in room-flow-scenes-1.jsx.
const ORBIT_OFFSETS = [
  { x: -34.21, y: 0 },
  { x: 0, y: -34.21 },
  { x: 34.21, y: 0 },
] as const;

/**
 * The Quick Play "finding a table" diagram: the viewer sits at the centre of a
 * brass ring while the three other seats orbit around them, filling in with
 * real opponents as they join. Pure presentational — all room/WS plumbing
 * lives in MatchmakingPage. Team colour is viewer-relative (gold = your side,
 * silver = opponents), matching RoomPage's seat convention.
 */
export function MatchmakingDiagram({
  room,
  found,
  players,
  viewerSeat,
  currentUsername,
  elapsed,
  onCancel,
  cancelDisabled,
}: Props) {
  const { t } = useTranslation();

  // The three seats other than the viewer's, in ascending order, mapped onto
  // the orbit's west/north/east positions.
  const orbitSeats = [0, 1, 2, 3].filter((s) => s !== viewerSeat);
  const remaining = Math.max(0, 4 - found);

  return (
    <div
      data-testid="matchmaking-diagram"
      className="relative flex max-w-full flex-col items-center"
      style={{
        background: `
          radial-gradient(ellipse 70% 50% at 50% 35%, rgba(25,101,54,0.07), transparent 70%),
          radial-gradient(ellipse 60% 40% at 50% 115%, rgba(201,168,118,0.12), transparent 70%)`,
      }}
    >
      <Eyebrow tone="accent" className="gap-2">
        <span className="bg-accent inline-block size-2 rounded-full animate-[pulse-dot_1.6s_ease-in-out_infinite]" />
        {t("lobby.matchmaking.eyebrow")}
      </Eyebrow>

      <h1 className="font-display text-ink mt-2.5 mb-1.5 text-center text-[34px] font-bold tracking-[-0.8px]">
        {t("lobby.matchmaking.title")}
      </h1>
      <p className="text-ink-dim m-0 max-w-130 text-center text-[14.5px] leading-relaxed">
        {t("lobby.matchmaking.subtitle")}
      </p>

      {/* Match-rules + elapsed strip — derived from the actual room config
          (variant, mode, per-move timer, and the coin stake) rather than fixed
          assumptions, so it matches the room the player is queued into. */}
      <div className="mt-4 flex flex-wrap items-center justify-center gap-x-3 gap-y-2">
        <Badge tone="neutral" icon={<Trophy className="size-3 text-(--accent)" />}>
          {variantLabel(t, room.variant)}
        </Badge>
        <Badge tone="neutral">{modeLabel(t, room.matchMode)}</Badge>
        <Badge tone="accent" icon={<Clock className="size-3 text-(--accent)" />}>
          {room.timerStyle === "relaxed"
            ? t("lobby.card.relaxed")
            : t("lobby.card.timerSeconds", { seconds: room.timerDurationSeconds })}
        </Badge>
        <Badge tone="neutral" icon={<Coins className="size-3" style={{ color: COIN_GOLD }} />}>
          {room.coinBuyIn > 0 ? formatCoins(room.coinBuyIn) : t("lobby.card.buyInFree")}
        </Badge>
        <span className="text-ink-mute font-mono text-[11.5px] tracking-[0.5px]">
          {t("lobby.matchmaking.elapsed")}{" "}
          <strong className="text-ink tabular-nums" data-testid="matchmaking-elapsed">
            {elapsed}
          </strong>
        </span>
      </div>

      {/* Orbit diagram — an explicit square (not aspect-ratio: a flex-column
          item with only absolute children would collapse its height to 0).
          Capped at the 380px design size, shrinking to fit narrow screens; the
          inner geometry is percentage-based so it scales with the box. */}
      <div
        className="relative mt-7"
        style={{
          width: "min(380px, calc(100vw - 72px))",
          height: "min(380px, calc(100vw - 72px))",
        }}
      >
        {/* Brass ring — the table edge */}
        <div
          className="border-border-2 absolute rounded-full border-2 border-dashed opacity-70"
          style={{ inset: "10.53%" }}
        />
        {/* Felt halo */}
        <div
          className="absolute rounded-full"
          style={{
            inset: "15.79%",
            background: "radial-gradient(circle, rgba(25,101,54,0.10) 0%, transparent 70%)",
          }}
        />
        {/* Rotating sweep */}
        <div
          className="absolute rounded-full animate-[spin_2.4s_linear_infinite]"
          style={{
            inset: "10.53%",
            background:
              "conic-gradient(from 0deg, transparent 0%, transparent 70%, rgba(25,101,54,0.25) 90%, transparent 100%)",
            maskImage: "radial-gradient(circle, transparent 65%, #000 66%, #000 100%)",
            WebkitMaskImage: "radial-gradient(circle, transparent 65%, #000 66%, #000 100%)",
          }}
        />

        {/* You — centre */}
        <div className="absolute top-1/2 left-1/2 flex -translate-x-1/2 -translate-y-1/2 flex-col items-center gap-1.5">
          <div className="relative">
            <Avatar name={currentUsername} size={64} you />
            <span
              className="border-accent absolute rounded-full border-2 opacity-40 animate-[pulse-dot_1.6s_ease-in-out_infinite]"
              style={{ inset: -6 }}
            />
          </div>
          <span className="text-accent-deep font-mono text-[10px] font-semibold tracking-[1.5px] uppercase">
            {t("lobby.matchmaking.you")}
          </span>
        </div>

        {/* Three orbit seats — one per offset (west, north, east). */}
        {ORBIT_OFFSETS.map((offset, i) => {
          const seatIndex = orbitSeats[i];
          if (seatIndex === undefined) return null;
          const occupant = players.find((p) => p.seat === seatIndex);
          const isUs = seatIndex % 2 === viewerSeat % 2;
          const team = isUs ? "A" : "B";
          return (
            <div
              key={seatIndex}
              data-testid={`matchmaking-orbit-${seatIndex}`}
              className="absolute flex -translate-x-1/2 -translate-y-1/2 flex-col items-center gap-1.5"
              style={{ left: `calc(50% + ${offset.x}%)`, top: `calc(50% + ${offset.y}%)` }}
            >
              {occupant ? (
                <Avatar name={occupant.username} size={52} team={team} />
              ) : (
                <div
                  className="bg-surface-elevated flex size-13 items-center justify-center rounded-full border-2 border-dashed"
                  style={{
                    borderColor: isUs ? "var(--team-a-edge-soft)" : "var(--team-b-edge-soft)",
                  }}
                >
                  <span
                    className="size-2 rounded-full opacity-55 animate-[pulse-dot_1.4s_ease-in-out_infinite]"
                    style={{ background: isUs ? "var(--team-a)" : "var(--team-b)" }}
                  />
                </div>
              )}
              <span
                className="font-mono text-[9.5px] font-semibold tracking-[1.4px] uppercase"
                style={{
                  color: occupant ? (isUs ? "var(--team-a)" : "var(--team-b)") : "var(--ink-mute)",
                }}
              >
                {occupant ? occupant.username : t("lobby.matchmaking.searching")}
              </span>
            </div>
          );
        })}
      </div>

      {/* Progress line + cancel */}
      <div className="mt-7 flex w-105 max-w-full flex-col items-center gap-4">
        <div
          className="bg-surface-sunken border-border relative h-1.5 w-full overflow-hidden rounded-full border"
          role="progressbar"
          aria-valuenow={found}
          aria-valuemin={0}
          aria-valuemax={4}
        >
          <div
            className="absolute inset-0 rounded-full animate-[shimmer_2.2s_linear_infinite]"
            style={{
              width: `${(found / 4) * 100}%`,
              background: "linear-gradient(90deg, var(--brass) 0%, var(--accent) 100%)",
              backgroundSize: "200% 100%",
            }}
          />
        </div>
        <div className="text-ink-mute flex w-full items-center justify-between font-mono text-[11px] tracking-[1.4px] uppercase">
          <span data-testid="matchmaking-seated">{t("lobby.matchmaking.seated", { found })}</span>
          <span>{t("lobby.matchmaking.remaining", { count: remaining })}</span>
        </div>
        <Button
          variant="destructive"
          size="sm"
          onClick={onCancel}
          disabled={cancelDisabled}
          data-testid="matchmaking-cancel"
        >
          <X className="size-3.5" />
          {t("lobby.matchmaking.cancelQueue")}
        </Button>
      </div>
    </div>
  );
}
