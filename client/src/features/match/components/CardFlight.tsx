import { useEffect, useId, useLayoutEffect, useRef, useState } from "react";

import { Z } from "@/shared/lib/zLayers";
import type { Card } from "@/shared/types/matchTypes";

import { PlayingCard } from "./PlayingCard";

/** Viewport-relative rect captured via `getBoundingClientRect()`. */
export interface FlightRect {
  left: number;
  top: number;
  width: number;
  height: number;
}

export interface CardFlightDescriptor {
  /** Stable id used for keyframe naming + onComplete dedup. */
  id: string;
  card: Card;
  fromRect: FlightRect;
  toRect: FlightRect;
  /**
   * Optional mid-flight waypoint. When provided, the keyframe gains a 50%
   * stop at this rect — used by the self-throw flight to arc through
   * viewport bottom-center instead of cutting diagonally from hand to slot.
   */
  waypointRect?: FlightRect;
  durationMs: number;
  /** CSS easing string. Defaults to a soft ease-out so the card "settles"
   *  rather than snapping at the destination. */
  easing?: string;
  /** Optional extra delay before the flight starts (e.g. resolve-pause for
   *  the collect flights). */
  delayMs?: number;
  /**
   * Opacity at the final keyframe stop. Throws end at 1 (the static slot
   * card takes over visually); the collect flights end at 0 so the cards
   * fade as they reach the winner's pile.
   */
  endOpacity?: number;
}

interface CardFlightProps {
  flights: CardFlightDescriptor[];
  onComplete: (id: string) => void;
}

// Single base size for all flying cards. The flight scales the rendered card
// to match the source rect at the start and the destination rect at the end,
// so this base value only sets the resolution at which the card glyphs are
// rasterized — it does not affect the on-screen size at any moment.
const BASE_W = 72; // matches PlayingCard size="md"
const BASE_H = 104;

interface FlightCardProps {
  flight: CardFlightDescriptor;
  animScope: string;
  /** Overlay's own viewport offset — subtracted from each flight rect so the
   *  flight renders correctly even if some ancestor of the overlay is a
   *  containing block (transform/perspective/filter break `position: fixed`'s
   *  viewport anchoring). When the overlay is a true viewport-fixed layer,
   *  this is `{0, 0}` and the math is a no-op. */
  overlayOffset: { x: number; y: number };
  onComplete: (id: string) => void;
}

/**
 * Single flying card. Keyframes are emitted inline in a per-card `<style>`
 * block so each flight gets its own scoped keyframe name. The animationend
 * listener is attached natively via a ref + addEventListener — React 19's
 * synthetic-event delegation for `onAnimationEnd` is brittle under jsdom
 * (it doesn't fire in our test harness), and going native sidesteps the
 * issue without changing real-browser behaviour.
 */
function FlightCard({ flight: f, animScope, overlayOffset, onComplete }: FlightCardProps) {
  const animatedElRef = useRef<HTMLDivElement | null>(null);
  const onCompleteRef = useRef(onComplete);
  onCompleteRef.current = onComplete;

  useEffect(() => {
    const el = animatedElRef.current;
    if (!el) return;
    const handler = (e: AnimationEvent) => {
      // Only fire onComplete for animationend events that actually came from
      // the flight wrapper itself — descendants with their own animations
      // (none today, but the guard is defensive) bubble up here too.
      if (e.target !== el) return;
      onCompleteRef.current(f.id);
    };
    el.addEventListener("animationend", handler);
    return () => el.removeEventListener("animationend", handler);
  }, [f.id]);

  const fromScale = f.fromRect.width / BASE_W;
  const toScale = f.toRect.width / BASE_W;
  // The keyframe targets the card's top-left corner. With transform-origin
  // top-left, scale shrinks/grows toward that corner — so we place top-left
  // at the source rect's top-left at 0% and the destination rect's top-left
  // at 100%, and let scale handle size. Subtract overlayOffset so a non-
  // viewport-anchored overlay (containing-block ancestor) still lands the
  // flight at the correct viewport coordinates.
  const fromX = f.fromRect.left - overlayOffset.x;
  const fromY = f.fromRect.top - overlayOffset.y;
  const toX = f.toRect.left - overlayOffset.x;
  const toY = f.toRect.top - overlayOffset.y;
  const safeId = f.id.replace(/[^a-zA-Z0-9]/g, "_");
  const keyframeName = `cardFlight_${animScope}_${safeId}`;
  const endOpacity = f.endOpacity ?? 1;

  let keyframes: string;
  if (f.waypointRect) {
    // Three-stop arc: fromRect → waypointRect (mid) → toRect. Used for the
    // self throw so the card visibly passes through viewport bottom-center
    // before continuing up to the south slot.
    const wpScale = f.waypointRect.width / BASE_W;
    const wpX = f.waypointRect.left - overlayOffset.x;
    const wpY = f.waypointRect.top - overlayOffset.y;
    keyframes = `@keyframes ${keyframeName} {
      0%   { transform: translate3d(${fromX}px, ${fromY}px, 0) scale(${fromScale}); opacity: 1; }
      50%  { transform: translate3d(${wpX}px, ${wpY}px, 0) scale(${wpScale}); opacity: 1; }
      85%  { transform: translate3d(${toX}px, ${toY}px, 0) scale(${toScale}); opacity: 1; }
      100% { transform: translate3d(${toX}px, ${toY}px, 0) scale(${toScale}); opacity: ${endOpacity}; }
    }`;
  } else {
    keyframes = `@keyframes ${keyframeName} {
      0%   { transform: translate3d(${fromX}px, ${fromY}px, 0) scale(${fromScale}); opacity: 1; }
      85%  { transform: translate3d(${toX}px, ${toY}px, 0) scale(${toScale}); opacity: 1; }
      100% { transform: translate3d(${toX}px, ${toY}px, 0) scale(${toScale}); opacity: ${endOpacity}; }
    }`;
  }

  return (
    <div data-testid={`card-flight-${f.id}`}>
      <style>{keyframes}</style>
      <div
        ref={animatedElRef}
        data-testid={`card-flight-${f.id}-animated`}
        style={{
          position: "absolute",
          left: 0,
          top: 0,
          width: BASE_W,
          height: BASE_H,
          transformOrigin: "top left",
          animation: `${keyframeName} ${f.durationMs}ms ${f.easing ?? "cubic-bezier(0.22, 1, 0.36, 1)"} ${f.delayMs ?? 0}ms both`,
          willChange: "transform, opacity",
        }}
      >
        <PlayingCard card={f.card} state="default" size="md" withTransition={false} />
      </div>
    </div>
  );
}

/**
 * Viewport-fixed overlay that animates cards between any two screen rects
 * with size morphing.
 *
 * Why an overlay (vs. animating in-place inside `HandCards` / `TrickArea` /
 * `PlayerSeat`)? Each of those lives in its own positioned coordinate system,
 * so a card travelling between them needs viewport coordinates to look like
 * a single continuous element. Doing it in-place would either double-paint
 * (one element exiting, another entering) or smuggle one component's DOM
 * into another's layout context.
 *
 * Coordinate model: every flight is described by two `FlightRect`s in
 * viewport space (i.e. what `getBoundingClientRect()` returns). The overlay
 * itself is `position: fixed; inset: 0`, so under normal circumstances its
 * top-left sits at (0,0) and the keyframes use the rects directly. If an
 * ancestor with `transform`/`perspective`/`filter` creates a containing
 * block, the overlay's actual top-left shifts away from (0,0) — we measure
 * the overlay's own rect on mount and subtract its offset from each flight
 * rect so the math stays correct in either case.
 *
 * Each flight gets its own `<style>` block with a uniquely-named `@keyframes`
 * — the keyframe interpolates `transform`, so width/height stay constant at
 * the base size and the visual size morphs through the scale factor. Using
 * transform-only animation keeps the flight on the compositor thread.
 */
export function CardFlight({ flights, onComplete }: CardFlightProps) {
  // Stable id used to namespace keyframe names so multiple overlay instances
  // (Strict-mode double-mount, multiple sessions) don't collide.
  const animScope = useId().replace(/:/g, "");
  const overlayRef = useRef<HTMLDivElement | null>(null);
  const [overlayOffset, setOverlayOffset] = useState<{ x: number; y: number }>({ x: 0, y: 0 });

  useLayoutEffect(() => {
    const el = overlayRef.current;
    if (!el) return;
    const r = el.getBoundingClientRect();
    // If `position: fixed` is anchored to the viewport, top-left is (0,0).
    // Anything else means an ancestor created a containing block; capture
    // the actual offset so our keyframe math compensates.
    setOverlayOffset({ x: r.left, y: r.top });
  }, [flights.length]);

  if (flights.length === 0) return null;

  return (
    <div
      ref={overlayRef}
      className="fixed inset-0 pointer-events-none"
      style={{ zIndex: Z.CARD_FLIGHT }}
      data-testid="card-flight-overlay"
    >
      {flights.map((f) => (
        <FlightCard
          key={f.id}
          flight={f}
          animScope={animScope}
          overlayOffset={overlayOffset}
          onComplete={onComplete}
        />
      ))}
    </div>
  );
}
