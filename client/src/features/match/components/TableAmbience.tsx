import { useMemo } from "react";

import { useReducedMotion } from "@/shared/hooks/useReducedMotion";

interface TableAmbienceProps {
  intensity?: number;
  tint?: string;
}

interface Particle {
  x: number;
  y: number;
  s: number;
  o: number;
  d: number;
}

const PARTICLE_COUNT = 30;

/**
 * Decorative felt ambience: a soft top beam plus floating particles, computed once per mount.
 * Under reduced motion the particles render as a static field.
 */
export function TableAmbience({ intensity = 0.9, tint = "var(--lamp-warm)" }: TableAmbienceProps) {
  const reducedMotion = useReducedMotion();
  const particles = useMemo<Particle[]>(
    () =>
      Array.from({ length: PARTICLE_COUNT }, () => ({
        x: Math.random() * 100,
        y: Math.random() * 100,
        s: Math.random() * 2 + 0.5,
        o: Math.random() * 0.4 + 0.1,
        d: Math.random() * 8 + 6,
      })),
    [],
  );

  return (
    <div
      className="pointer-events-none absolute inset-0 overflow-hidden"
      style={{ opacity: intensity }}
      data-testid="table-ambience"
      aria-hidden
    >
      <div
        className="absolute"
        style={{
          top: -200,
          left: "50%",
          width: 1000,
          height: 600,
          transform: "translateX(-50%)",
          background: `radial-gradient(ellipse at center, color-mix(in srgb, ${tint} 13%, transparent) 0%, transparent 65%)`,
        }}
      />
      {particles.map((p, i) => (
        <span
          key={i}
          className="absolute rounded-full"
          style={{
            left: `${p.x}%`,
            top: `${p.y}%`,
            width: p.s * 2,
            height: p.s * 2,
            background: tint,
            opacity: p.o,
            filter: "blur(0.5px)",
            animation: reducedMotion
              ? undefined
              : `tableFloat${i % 3} ${p.d}s ease-in-out infinite`,
          }}
        />
      ))}
      <style>{`
        @keyframes tableFloat0 { 0%,100% { transform: translateY(0); } 50% { transform: translateY(-14px); } }
        @keyframes tableFloat1 { 0%,100% { transform: translateY(0); } 50% { transform: translateY(-22px); } }
        @keyframes tableFloat2 { 0%,100% { transform: translateY(0); } 50% { transform: translateY(-9px); } }
      `}</style>
    </div>
  );
}
