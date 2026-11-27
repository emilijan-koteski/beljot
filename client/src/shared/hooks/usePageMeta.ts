import { useEffect } from "react";

const SITE_NAME = "Beljot.online";

/** Sets the tab title, and optionally the meta description, while the page is mounted. */
export function usePageMeta(title: string, description?: string) {
  useEffect(() => {
    const previousTitle = document.title;
    document.title = `${title} · ${SITE_NAME}`;
    const meta = document.querySelector<HTMLMetaElement>('meta[name="description"]');
    const previousDescription = meta?.content;
    if (meta && description !== undefined) meta.content = description;
    return () => {
      document.title = previousTitle;
      if (meta && previousDescription !== undefined) meta.content = previousDescription;
    };
  }, [title, description]);
}
