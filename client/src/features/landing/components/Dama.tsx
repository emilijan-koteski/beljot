import { cn } from "@/shared/lib/utils";

type DamaProps = {
  pose: "signature" | "win-half" | "pointing";
  className?: string;
  eager?: boolean;
};

/** Dama, the house hostess, as a decorative cut-out from `public/mascot`. */
export function Dama({ pose, className, eager = false }: DamaProps) {
  return (
    <img
      src={`${import.meta.env.BASE_URL}mascot/dama-${pose}.webp`}
      alt=""
      aria-hidden="true"
      loading={eager ? "eager" : "lazy"}
      decoding="async"
      className={cn("pointer-events-none max-w-none select-none", className)}
    />
  );
}
