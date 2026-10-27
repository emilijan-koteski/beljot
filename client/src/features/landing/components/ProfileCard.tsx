type ProfileLabels = {
  member: string;
  games: string;
  winrate: string;
  wins: string;
  recent: string;
  won: string;
  lost: string;
  with: string;
};

type Match = { won: boolean; a: string; b: string; withWho: string; when: string };

const MATCHES: Match[] = [
  { won: true, a: "1012", b: "743", withWho: "emilijan", when: "2d" },
  { won: true, a: "1024", b: "911", withWho: "cvetanka", when: "3d" },
  { won: false, a: "812", b: "1018", withWho: "ana", when: "4d" },
];

// Static marketing profile card showing the shape of the real profile page.
// Labels are translated; numbers are illustrative.
export function ProfileCard({ labels }: { labels: ProfileLabels }) {
  const stats: [string, string][] = [
    ["142", labels.games],
    ["63%", labels.winrate],
    ["89", labels.wins],
  ];

  return (
    <div className="bg-surface-elevated border-border w-full max-w-95 overflow-hidden rounded-xl border shadow-lg">
      <div className="flex items-center gap-3 px-4.5 pt-4.5 pb-4">
        <span
          className="flex size-11 items-center justify-center rounded-full text-lg font-bold"
          style={{
            background: "var(--accent-deep)",
            border: "2px solid var(--brass)",
            color: "var(--bg)",
          }}
        >
          K
        </span>
        <div className="leading-tight">
          <div className="text-ink text-lg font-semibold">kiro</div>
          <div className="text-ink-mute text-xs">{labels.member}</div>
        </div>
      </div>

      <div className="border-border grid grid-cols-3 border-t border-b">
        {stats.map(([value, label], i) => (
          <div
            key={label}
            className="px-3 py-4 text-center"
            style={i ? { borderLeft: "1px solid var(--border)" } : undefined}
          >
            <div className="text-ink tabular-nums text-3xl font-bold tracking-tight">{value}</div>
            <div className="text-ink-mute mt-0.5 text-xs">{label}</div>
          </div>
        ))}
      </div>

      <div className="text-brass-deep font-mono px-4.5 pt-3 pb-1.5 text-2xs font-semibold tracking-caps uppercase">
        {labels.recent}
      </div>
      <div className="px-4.5 pb-3.5">
        {MATCHES.map((m, i) => (
          <div
            key={i}
            className="grid grid-cols-[auto_1fr_auto] items-center gap-3 py-2.5"
            style={i < MATCHES.length - 1 ? { borderBottom: "1px solid var(--border)" } : undefined}
          >
            <span
              className="min-w-14 rounded-md px-2 py-1 text-center text-xs font-bold"
              style={
                m.won
                  ? { background: "var(--accent-soft)", color: "var(--accent)" }
                  : {
                      background: "color-mix(in srgb, var(--danger) 10%, transparent)",
                      color: "var(--danger)",
                    }
              }
            >
              {m.won ? labels.won : labels.lost}
            </span>
            <span className="font-mono text-ink tabular-nums text-sm font-semibold">
              {m.a} <span className="text-ink-off">–</span> {m.b}
              <span className="font-sans text-ink-mute ml-2 text-xs font-normal">
                {labels.with} {m.withWho}
              </span>
            </span>
            <span className="text-ink-mute text-xs">{m.when}</span>
          </div>
        ))}
      </div>
    </div>
  );
}
