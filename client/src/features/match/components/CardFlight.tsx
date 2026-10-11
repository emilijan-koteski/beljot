import { useEffect, useId, useLayoutEffect, useRef, useState } from "react";

import { Z } from "@/shared/lib/zLayers";
import type { Card } from "@/shared/types/matchTypes";

import { CARD_SIZES } from "../lib/cardFace";
import { PlayingCard } from "./PlayingCard";

/** Viewport-relative rect captured via `getBoundingClientRect()`. */
export interface FlightRect {
  left: number;
  top: number;
  width: number;
  height: number;
}

export interface CardFlightDescriptor {
  id: string;
  card: Card | null;
  fromRect: FlightRect;
  toRect: FlightRect;
  waypointRect?: FlightRect;
  durationMs: number;
  easing?: string;
  delayMs?: number;
  endOpacity?: number;
}

interface CardFlightProps {
  flights: CardFlightDescriptor[];
  onComplete: (id: string) => void;
}

const { width: BASE_W, height: BASE_H } = CARD_SIZES.md;

interface FlightCardProps {
  flight: CardFlightDescriptor;
  animScope: string;
  overlayOffset: { x: number; y: number };
  onComplete: (id: string) => void;
}

/**
 * Single flying card with its own scoped keyframes. The animationend listener is attached natively
 * because React 19's synthetic onAnimationEnd does not fire under jsdom.
 */
function FlightCard({ flight: f, animScope, overlayOffset, onComplete }: FlightCardProps) {
  const animatedElRef = useRef<HTMLDivElement | null>(null);
  const onCompleteRef = useRef(onComplete);
  onCompleteRef.current = onComplete;

  useEffect(() => {
    const el = animatedElRef.current;
    if (!el) return;
    const handler = (e: AnimationEvent) => {
      if (e.target !== el) return;
      onCompleteRef.current(f.id);
    };
    el.addEventListener("animationend", handler);
    return () => el.removeEventListener("animationend", handler);
  }, [f.id]);

  const fromScale = f.fromRect.width / BASE_W;
  const toScale = f.toRect.width / BASE_W;
  const fromX = f.fromRect.left - overlayOffset.x;
  const fromY = f.fromRect.top - overlayOffset.y;
  const toX = f.toRect.left - overlayOffset.x;
  const toY = f.toRect.top - overlayOffset.y;
  const safeId = f.id.replace(/[^a-zA-Z0-9]/g, "_");
  const keyframeName = `cardFlight_${animScope}_${safeId}`;
  const endOpacity = f.endOpacity ?? 1;

  let keyframes: string;
  if (f.waypointRect) {
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
        {f.card === null ? (
          <PlayingCard card={null} state="face-down" size="md" withTransition={false} />
        ) : (
          <PlayingCard card={f.card} state="default" size="md" withTransition={false} />
        )}
      </div>
    </div>
  );
}

/**
 * Viewport-fixed overlay that animates cards between any two screen rects with size morphing.
 * It measures its own offset on mount so flights stay correct inside a transformed ancestor.
 */
export function CardFlight({ flights, onComplete }: CardFlightProps) {
  const animScope = useId().replace(/:/g, "");
  const overlayRef = useRef<HTMLDivElement | null>(null);
  const [overlayOffset, setOverlayOffset] = useState<{ x: number; y: number }>({ x: 0, y: 0 });

  useLayoutEffect(() => {
    const el = overlayRef.current;
    if (!el) return;
    const r = el.getBoundingClientRect();
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
