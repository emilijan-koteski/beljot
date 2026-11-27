import type { ReactNode } from "react";
import { useTranslation } from "react-i18next";

type SocialNetwork = "instagram" | "facebook" | "youtube";

const SOCIAL_PROFILES: ReadonlyArray<{ network: SocialNetwork; url: string }> = [
  { network: "instagram", url: "https://www.instagram.com/beljot.online" },
  { network: "facebook", url: "https://www.facebook.com/beljot.online" },
  { network: "youtube", url: "https://www.youtube.com/@beljot.online" },
];

const ICONS: Record<SocialNetwork, ReactNode> = {
  instagram: (
    <>
      <rect x="3" y="3" width="18" height="18" rx="5" />
      <circle cx="12" cy="12" r="4" />
      <circle cx="17.5" cy="6.5" r="0.6" fill="currentColor" />
    </>
  ),
  facebook: <path d="M14 8h3V4h-3a4 4 0 0 0-4 4v3H7v4h3v6h4v-6h3l1-4h-4V8z" />,
  youtube: (
    <>
      <rect x="2.5" y="5.5" width="19" height="13" rx="4" />
      <path d="M10 9.5v5l4.5-2.5z" fill="currentColor" />
    </>
  ),
};

/** Row of icon links to the Beljot.online social profiles, opened in a new tab. */
export function SocialLinks() {
  const { t } = useTranslation();

  return (
    <ul className="flex items-center gap-1" aria-label={t("landing.foot.social.label")}>
      {SOCIAL_PROFILES.map(({ network, url }) => (
        <li key={network}>
          <a
            href={url}
            target="_blank"
            rel="noopener noreferrer"
            aria-label={t(`landing.foot.social.${network}`)}
            data-testid={`social-link-${network}`}
            className="text-ink-mute hover:text-ink inline-flex size-8 items-center justify-center rounded-full transition-colors"
          >
            <svg
              viewBox="0 0 24 24"
              className="size-4.5"
              fill="none"
              stroke="currentColor"
              strokeWidth="1.8"
              strokeLinejoin="round"
              aria-hidden="true"
            >
              {ICONS[network]}
            </svg>
          </a>
        </li>
      ))}
    </ul>
  );
}
