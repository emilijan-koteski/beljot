type StepProps = {
  n: string;
  title: string;
  body: string;
};

// A numbered "how it works" step with a brass-ringed numeral, title and body.
export function Step({ n, title, body }: StepProps) {
  return (
    <div className="flex items-start gap-4.5">
      <div className="font-display text-accent border-border-2 flex size-10 shrink-0 items-center justify-center rounded-full border-[1.5px] text-[17px] font-bold">
        {n}
      </div>
      <div>
        <h3 className="font-display text-ink mb-1.5 text-xl font-semibold">{title}</h3>
        <p className="text-ink-dim text-[15px] leading-[1.6]">{body}</p>
      </div>
    </div>
  );
}
