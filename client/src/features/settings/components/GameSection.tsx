import { Gamepad2 } from "lucide-react";
import { useTranslation } from "react-i18next";

import { Segmented } from "@/shared/components/ui/segmented";
import {
  LANGUAGE_OPTIONS,
  type LanguageCode,
  useChangeLanguage,
} from "@/shared/hooks/useChangeLanguage";

import { CardDeckRow } from "./CardDeckRow";
import { SettingsGroup, SettingsRow } from "./SettingsGroup";

/** The language setting: a switch of the four language codes, the current language named under the label. */
function LanguageRow({ wide }: { wide: boolean }) {
  const { t, i18n } = useTranslation();
  const changeLanguage = useChangeLanguage(true);
  const current = (LANGUAGE_OPTIONS.find((l) => i18n.language.startsWith(l.code)) ??
    LANGUAGE_OPTIONS[0]) as (typeof LANGUAGE_OPTIONS)[number];

  const picker = (
    <Segmented<LanguageCode>
      value={current.code}
      onValueChange={(code) => void changeLanguage(code)}
      ariaLabel={t("settings.language.label")}
      testId="settings-language"
      dense
      className={wide ? "w-64 shrink-0" : undefined}
      options={LANGUAGE_OPTIONS.map((l) => ({ value: l.code, label: l.code.toUpperCase() }))}
    />
  );

  return (
    <SettingsRow
      label={t("settings.language.label")}
      hint={t(current.labelKey)}
      stack={!wide}
      control={wide ? picker : undefined}
    >
      {!wide && picker}
    </SettingsRow>
  );
}

/** The Game group of Settings: language and card deck; `wide` lines their controls up on the right. */
export function GameSection({ wide = false }: { wide?: boolean }) {
  const { t } = useTranslation();
  return (
    <SettingsGroup
      id="game"
      title={t("settings.sections.game")}
      icon={<Gamepad2 />}
      testId="settings-game"
    >
      <LanguageRow wide={wide} />
      <CardDeckRow wide={wide} />
    </SettingsGroup>
  );
}
