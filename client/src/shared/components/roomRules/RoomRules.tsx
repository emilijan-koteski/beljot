import { Popover } from "@base-ui/react/popover";
import { Bot } from "lucide-react";
import type { MouseEvent, ReactNode } from "react";
import { useTranslation } from "react-i18next";

import { Eyebrow } from "@/shared/components/ui/eyebrow";
import { POPUP_SURFACE } from "@/shared/lib/popupSurface";
import { roomBotCount } from "@/shared/lib/roomOccupancy";
import { cn } from "@/shared/lib/utils";

import {
  matchRuleLines,
  type MatchRuleSource,
  roomRuleLines,
  type RoomRuleSource,
  type RuleLine,
} from "./roomRuleLines";

export const RULES_PANEL = "flex w-70 flex-col gap-2.5 p-3 text-xs";

function iconTone(line: RuleLine): string {
  if (line.unused) return "text-ink-mute opacity-35";
  return line.changed ? "text-brass-deep" : "text-ink-mute";
}

/** A titled list of rule lines: icon, label and value, the value bold where it differs from the standard. */
function RuleSection({
  title,
  lines,
  testId,
}: {
  title: string;
  lines: RuleLine[];
  testId: string;
}) {
  return (
    <section className="flex flex-col gap-1.5" data-testid={testId}>
      <Eyebrow>{title}</Eyebrow>
      <ul className="m-0 flex list-none flex-col gap-1.5 p-0">
        {lines.map((line) => (
          <li
            key={line.key}
            data-testid={`rule-${line.key}`}
            data-changed={line.changed ? "true" : "false"}
            className="grid grid-cols-[14px_minmax(0,1fr)_auto] items-center gap-2"
          >
            <line.icon className={cn("size-3.5", iconTone(line))} aria-hidden="true" />
            <span className="text-ink-dim truncate">{line.label}</span>
            <span
              className={cn(
                "text-right",
                line.changed
                  ? "text-ink font-semibold"
                  : line.unused
                    ? "text-ink-mute"
                    : "text-ink",
              )}
            >
              {line.value}
            </span>
          </li>
        ))}
      </ul>
    </section>
  );
}

/** Every rule of a room in words: match rules, room rules, what its bots mean, and the brass legend. */
export function RoomRulesContent({ room, live = false }: { room: RoomRuleSource; live?: boolean }) {
  const { t } = useTranslation();
  const bots = roomBotCount(room);
  return (
    <>
      <div className="text-ink text-sm font-semibold">{t("roomRules.title")}</div>
      <RuleSection
        title={t("roomRules.matchSection")}
        lines={matchRuleLines(t, room)}
        testId="rules-match"
      />
      <div className="border-border border-t" />
      <RuleSection
        title={t("roomRules.roomSection")}
        lines={roomRuleLines(t, room)}
        testId="rules-room"
      />
      {bots > 0 && (
        <p
          className="bg-bot-soft text-bot-deep m-0 flex gap-2 rounded-md px-2 py-1.5 text-2xs leading-snug"
          data-testid="rules-bots-note"
        >
          <Bot className="mt-px size-3.5 shrink-0" aria-hidden="true" />
          <span>
            {t(live ? "roomRules.botsLive" : "roomRules.botsWaiting")}
            {room.coinBuyIn > 0 && ` ${t("roomRules.botsStake")}`}
          </span>
        </p>
      )}
      <p className="text-ink-mute m-0 text-2xs">{t("roomRules.legend")}</p>
    </>
  );
}

/** The match rules alone in words, for a finished match that no longer has a room behind it. */
export function MatchRulesContent({ rules }: { rules: MatchRuleSource }) {
  const { t } = useTranslation();
  return (
    <>
      <div className="text-ink text-sm font-semibold">{t("roomRules.title")}</div>
      <RuleSection
        title={t("roomRules.matchSection")}
        lines={matchRuleLines(t, rules)}
        testId="rules-match"
      />
      <p className="text-ink-mute m-0 text-2xs">{t("roomRules.legend")}</p>
    </>
  );
}

function stopPropagation(event: MouseEvent) {
  event.stopPropagation();
}

interface RulesPopoverProps {
  rules: MatchRuleSource;
  labelled?: boolean;
  className?: string;
  zIndex?: number;
  children: ReactNode;
}

/**
 * The four match rules as fixed icon slots (brass where changed), opening the rules in words on hover or tap.
 * Clicks stay inside, so the trigger can sit in a clickable row.
 */
function RulesPopover({ rules, labelled = false, className, zIndex, children }: RulesPopoverProps) {
  const { t } = useTranslation();
  const lines = matchRuleLines(t, rules);
  return (
    <Popover.Root>
      <Popover.Trigger
        openOnHover
        delay={0}
        closeDelay={0}
        onClick={stopPropagation}
        aria-label={t("roomRules.triggerAria", {
          summary: lines.map((l) => `${l.label}: ${l.value}`).join(", "),
        })}
        data-testid="room-rules-trigger"
        className={cn(
          "data-popup-open:bg-brass-soft focus-visible:ring-ring/70 -mr-1 inline-flex cursor-pointer items-center gap-1 rounded-md px-1 py-0.5 outline-none focus-visible:ring-3",
          className,
        )}
      >
        {labelled && <span className="mr-0.5">{t("roomRules.title")}</span>}
        {lines.map((line) => (
          <line.icon key={line.key} className={cn("size-3.5", iconTone(line))} aria-hidden="true" />
        ))}
      </Popover.Trigger>
      <Popover.Portal>
        <Popover.Positioner
          side="bottom"
          align="end"
          sideOffset={6}
          collisionPadding={12}
          className="z-50"
          style={zIndex === undefined ? undefined : { zIndex }}
        >
          <Popover.Popup
            className={cn(POPUP_SURFACE, RULES_PANEL)}
            data-testid="room-rules-popup"
            onClick={stopPropagation}
          >
            {children}
          </Popover.Popup>
        </Popover.Positioner>
      </Popover.Portal>
    </Popover.Root>
  );
}

interface RoomRulesPopoverProps {
  room: RoomRuleSource;
  live?: boolean;
  labelled?: boolean;
  className?: string;
}

/** A room's rule icons opening its match and room rules. The slots never change in number, so cards keep one height. */
export function RoomRulesPopover({
  room,
  live = false,
  labelled,
  className,
}: RoomRulesPopoverProps) {
  return (
    <RulesPopover rules={room} labelled={labelled} className={className}>
      <RoomRulesContent room={room} live={live} />
    </RulesPopover>
  );
}

interface MatchRulesPopoverProps {
  rules: MatchRuleSource;
  className?: string;
  zIndex?: number;
}

/** A finished match's rule icons opening its match rules in words; `zIndex` lifts the popup over an overlay. */
export function MatchRulesPopover({ rules, className, zIndex }: MatchRulesPopoverProps) {
  return (
    <RulesPopover rules={rules} className={className} zIndex={zIndex}>
      <MatchRulesContent rules={rules} />
    </RulesPopover>
  );
}
