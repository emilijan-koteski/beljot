type StepProps = {
  n: string;
  title: string;
  body: string;
};

// A numbered "how it works" step with a brass-ringed numeral, title and body.
export function Step({ n, title, body }: StepProps) {
  return (
    <div className="flex items-start gap-4.5">
      <div className="text-accent border-brass flex size-11 shrink-0 items-center justify-center rounded-full border-2 text-lg font-bold">
        {n}
      </div>
      <div>
        <h3 className="text-ink mb-1.5 text-xl font-semibold">{title}</h3>
        <p className="text-ink-dim text-base leading-relaxed">{body}</p>
      </div>
    </div>
  );
}
