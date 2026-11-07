import { useActiveTheme } from "@/shared/hooks/useActiveTheme";

type ScreenShotProps = {
  name: string;
  alt: string;
  themed?: boolean;
  eager?: boolean;
};

/** A captured app screen from `public/landing`, in the visitor's theme when `themed`. */
export function ScreenShot({ name, alt, themed = false, eager = false }: ScreenShotProps) {
  const theme = useActiveTheme();
  const file = themed ? `${name}-${theme}` : name;
  return (
    <img
      src={`${import.meta.env.BASE_URL}landing/${file}.webp`}
      alt={alt}
      loading={eager ? "eager" : "lazy"}
      decoding="async"
      className="block w-full"
    />
  );
}
