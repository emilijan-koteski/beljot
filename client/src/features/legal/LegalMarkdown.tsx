import type { ComponentPropsWithoutRef } from "react";
import Markdown, { type Components, type ExtraProps } from "react-markdown";
import { Link } from "react-router";
import remarkGfm from "remark-gfm";

const REMARK_PLUGINS = [remarkGfm];

const linkClass =
  "text-accent font-medium underline underline-offset-2 decoration-accent/40 hover:decoration-accent [overflow-wrap:anywhere]";

type AnchorProps = ComponentPropsWithoutRef<"a"> & ExtraProps;

/** Routes app paths through the router, opens other sites in a new tab and leaves mailto links alone. */
function MarkdownLink({ href = "", children }: AnchorProps) {
  if (href.startsWith("/") && !href.startsWith("//")) {
    return (
      <Link to={href} className={linkClass}>
        {children}
      </Link>
    );
  }
  if (href.startsWith("mailto:") || href.startsWith("#")) {
    return (
      <a href={href} className={linkClass}>
        {children}
      </a>
    );
  }
  return (
    <a href={href} target="_blank" rel="noopener noreferrer" className={linkClass}>
      {children}
    </a>
  );
}

/** Wraps a table in its own horizontal scroller so wide tables never widen the page on a phone. */
function MarkdownTable({ children }: ComponentPropsWithoutRef<"table"> & ExtraProps) {
  return (
    <div
      className="border-border my-5 overflow-x-auto rounded-lg border"
      data-testid="legal-table"
      tabIndex={0}
    >
      <table className="w-full min-w-xl border-collapse text-left text-sm">{children}</table>
    </div>
  );
}

function headingFactory(testId: string | undefined): Components {
  return {
    h1: ({ children }) => (
      <h1
        className="text-ink m-0 mt-2 mb-2 text-3xl font-semibold tracking-tight md:text-4xl"
        data-testid={testId}
      >
        {children}
      </h1>
    ),
    h2: ({ children }) => (
      <h2 className="text-ink border-border mt-10 mb-3 border-t pt-6 text-lg font-semibold tracking-tight md:text-xl">
        {children}
      </h2>
    ),
    h3: ({ children }) => (
      <h3 className="text-ink mt-6 mb-2 text-base font-semibold">{children}</h3>
    ),
    h4: ({ children }) => <h4 className="text-ink mt-5 mb-2 text-sm font-semibold">{children}</h4>,
  };
}

const BODY_COMPONENTS: Components = {
  p: ({ children }) => <p className="text-ink-dim my-3 leading-relaxed">{children}</p>,
  ul: ({ children }) => (
    <ul className="text-ink-dim marker:text-brass my-3 list-disc space-y-1.5 pl-5 leading-relaxed">
      {children}
    </ul>
  ),
  ol: ({ children }) => (
    <ol className="text-ink-dim marker:text-ink-mute my-3 list-decimal space-y-1.5 pl-5 leading-relaxed">
      {children}
    </ol>
  ),
  li: ({ children }) => <li className="pl-1">{children}</li>,
  strong: ({ children }) => <strong className="text-ink font-semibold">{children}</strong>,
  em: ({ children }) => <em className="italic">{children}</em>,
  a: MarkdownLink,
  code: ({ children }) => (
    <code className="bg-surface-sunken text-ink rounded px-1 py-0.5 font-mono text-[0.85em] [overflow-wrap:anywhere]">
      {children}
    </code>
  ),
  blockquote: ({ children }) => (
    <blockquote className="border-brass text-ink-dim my-4 border-l-2 pl-4">{children}</blockquote>
  ),
  hr: () => <hr className="border-border my-8" />,
  table: MarkdownTable,
  thead: ({ children }) => <thead className="bg-surface-sunken">{children}</thead>,
  th: ({ children }) => (
    <th className="text-ink px-3 py-2 align-top text-xs font-semibold">{children}</th>
  ),
  td: ({ children }) => (
    <td className="text-ink-dim border-border border-t px-3 py-2 align-top leading-snug">
      {children}
    </td>
  ),
};

type Props = {
  source: string;
  titleTestId?: string;
};

/** Renders a legal document written in Markdown with the app's long-form typography. */
export function LegalMarkdown({ source, titleTestId }: Props) {
  return (
    <Markdown
      remarkPlugins={REMARK_PLUGINS}
      components={{ ...BODY_COMPONENTS, ...headingFactory(titleTestId) }}
    >
      {source}
    </Markdown>
  );
}
