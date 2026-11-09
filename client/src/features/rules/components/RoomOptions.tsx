import { type LucideIcon, Trophy } from "lucide-react";
import { useTranslation } from "react-i18next";

import { useRules } from "@/features/rules/RulesContext";
import { RULE_ICONS } from "@/shared/components/roomRules/roomRuleLines";
import {
  DECLARATION_LAST_TRICK_OPTIONS,
  DEFAULT_DECLARATION_LAST_TRICK,
} from "@/shared/types/matchTypes";

const FORM = "lobby.createRoomModal";

type Option = {
  key: string;
  icon: LucideIcon;
  label: string;
  standard: string;
  other: string;
  hint?: string;
};

/**
 * The game rules a host can change when creating a room, worded exactly as the create-room form words them,
 * the standard value first.
 */
export function RoomOptions() {
  const { t } = useTranslation();
  const { ui } = useRules();
  const options: Option[] = [
    {
      key: "target",
      icon: Trophy,
      label: t(`${FORM}.matchMode`),
      standard: "1001",
      other: "501 · 701",
    },
    {
      key: "direction",
      icon: RULE_ICONS.counterClockwise,
      label: t(`${FORM}.direction`),
      standard: t(`${FORM}.directionCounterClockwise`),
      other: t(`${FORM}.directionClockwise`),
      hint: t(`${FORM}.directionHint`),
    },
    {
      key: "declarations",
      icon: RULE_ICONS.declarationsOn,
      label: t(`${FORM}.declarations`),
      standard: t(`${FORM}.declarationsOn`),
      other: t(`${FORM}.declarationsOff`),
      hint: t(`${FORM}.declarationsHint`),
    },
    {
      key: "declarationLastTrick",
      icon: RULE_ICONS.declarationWindow,
      label: t(`${FORM}.declarationLastTrick`),
      standard: String(DEFAULT_DECLARATION_LAST_TRICK),
      other: DECLARATION_LAST_TRICK_OPTIONS.filter(
        (n) => n !== DEFAULT_DECLARATION_LAST_TRICK,
      ).join(" · "),
      hint: t(`${FORM}.declarationLastTrickHint`),
    },
    {
      key: "declarationsTransfer",
      icon: RULE_ICONS.fallTransfer,
      label: t(`${FORM}.declarationsTransfer`),
      standard: t(`${FORM}.declarationsTransferOn`),
      other: t(`${FORM}.declarationsTransferOff`),
    },
    {
      key: "matchEnd",
      icon: RULE_ICONS.finishHand,
      label: t(`${FORM}.matchEnd`),
      standard: t(`${FORM}.matchEndFinish`),
      other: t(`${FORM}.matchEndStop`),
    },
  ];
  return (
    <div className="flex flex-col gap-2">
      <ul
        className="bg-surface border-border divide-border m-0 flex list-none flex-col divide-y rounded-xl border p-0 shadow-xs"
        data-testid="rules-room-options"
      >
        {options.map(({ key, icon: Icon, label, standard, other, hint }) => (
          <li
            key={key}
            className="flex items-start gap-3 px-4 py-2.5"
            data-testid={`rules-option-${key}`}
          >
            <Icon className="text-ink-mute mt-0.5 size-4 shrink-0" aria-hidden="true" />
            <div className="flex min-w-0 flex-1 flex-col gap-0.5">
              <span className="flex flex-wrap items-baseline justify-between gap-x-3">
                <span className="text-ink text-sm font-semibold">{label}</span>
                <span className="text-xs">
                  <span className="text-ink font-semibold">{standard}</span>
                  <span className="text-ink-mute"> · {other}</span>
                </span>
              </span>
              {hint !== undefined && <span className="text-ink-mute text-xs">{hint}</span>}
            </div>
          </li>
        ))}
      </ul>
      <p className="text-ink-mute m-0 px-1 text-xs">{ui.optionsFoot}</p>
    </div>
  );
}
