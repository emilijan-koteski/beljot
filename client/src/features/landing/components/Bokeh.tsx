const DOTS = [
  [62, 8, 3, 0.45],
  [71, 22, 2, 0.6],
  [88, 14, 4, 0.2],
  [94, 38, 2.5, 0.5],
  [79, 61, 3, 0.35],
  [55, 74, 2, 0.55],
  [67, 88, 4, 0.18],
  [91, 79, 2, 0.5],
  [47, 30, 2.5, 0.3],
  [83, 47, 1.5, 0.6],
  [58, 52, 3, 0.25],
  [98, 9, 2, 0.4],
  [73, 95, 2.5, 0.3],
  [41, 90, 1.5, 0.45],
] as const;

/** Soft gold specks over the felt, echoing the splash-art lighting. */
export function Bokeh() {
  return (
    <div className="pointer-events-none absolute inset-0" aria-hidden="true">
      {DOTS.map(([left, top, radius, opacity]) => (
        <span
          key={`${left}-${top}`}
          className="absolute rounded-full bg-[#f2d48f] blur-[1.4px]"
          style={{
            left: `${left}%`,
            top: `${top}%`,
            width: radius * 2,
            height: radius * 2,
            opacity,
          }}
        />
      ))}
    </div>
  );
}
