import {
  Flag,
  HelpCircle,
  Menu as MenuIcon,
  Pause,
  Settings as SettingsIcon,
  Volume2,
  VolumeX,
  X,
} from "lucide-react";
import { type CSSProperties, useEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";

import { Z } from "@/shared/lib/zLayers";
import type { EmoteID } from "@/shared/types/wsEvents";

import { floatingPanelStyle } from "../lib/floatingPanel";
import { EmotePickerButton } from "./EmotePickerButton";
import { HUDButton } from "./HUDButton";

const ROW_STYLE: CSSProperties = { width: "100%", justifyContent: "flex-start" };

interface MatchHudMenuProps {
  showTurnActions: boolean;
  showPause: boolean;
  pauseUsed: boolean;
  canPause: boolean;
  canSurrender: boolean;
  audioOn: boolean;
  showEmotes: boolean;
  onPause: () => void;
  onSurrender: () => void;
  onRules: () => void;
  onSettings: () => void;
  onToggleMute: () => void;
  onSendEmote: (emote: EmoteID) => void;
}

/**
 * The compact table's HUD: one floating toggle that opens the table actions as HUDButton rows.
 * Any action, a click outside or Escape closes it, and Escape returns focus to the toggle.
 */
export function MatchHudMenu({
  showTurnActions,
  showPause,
  pauseUsed,
  canPause,
  canSurrender,
  audioOn,
  showEmotes,
  onPause,
  onSurrender,
  onRules,
  onSettings,
  onToggleMute,
  onSendEmote,
}: MatchHudMenuProps) {
  const { t } = useTranslation();
  const [open, setOpen] = useState(false);
  const toggleRef = useRef<HTMLButtonElement | null>(null);

  useEffect(() => {
    if (!open) return;
    const handleKey = (event: KeyboardEvent) => {
      if (event.key !== "Escape") return;
      setOpen(false);
      toggleRef.current?.focus();
    };
    document.addEventListener("keydown", handleKey);
    return () => document.removeEventListener("keydown", handleKey);
  }, [open]);

  const thenClose = (action: () => void) => () => {
    action();
    setOpen(false);
  };

  const MuteIcon = audioOn ? Volume2 : VolumeX;

  return (
    <div className="md:hidden">
      <button
        ref={toggleRef}
        type="button"
        onClick={() => setOpen((v) => !v)}
        aria-label={t("nav.menu")}
        aria-expanded={open}
        data-testid="hud-menu-button"
        className="absolute top-3 right-3 inline-flex size-9 items-center justify-center rounded-lg"
        style={{ zIndex: Z.CHROME_TOP, ...floatingPanelStyle(), color: "var(--ink-light)" }}
      >
        {open ? (
          <X className="h-5 w-5" aria-hidden="true" />
        ) : (
          <MenuIcon className="h-5 w-5" aria-hidden="true" />
        )}
      </button>

      {open && (
        <>
          <button
            type="button"
            aria-hidden="true"
            tabIndex={-1}
            onClick={() => setOpen(false)}
            className="fixed inset-0 cursor-default"
            style={{ zIndex: Z.CHROME_TOP }}
          />
          <div
            className="absolute top-14 right-3 flex w-44 flex-col gap-1.5 rounded-md p-2"
            style={{ zIndex: Z.CHROME_TOP, ...floatingPanelStyle({ menu: true }) }}
            data-testid="hud-menu"
          >
            {showTurnActions && (
              <>
                {showPause && (
                  <HUDButton
                    icon={<Pause className="h-4 w-4" aria-hidden="true" />}
                    label={pauseUsed ? t("match.pause.pauseUsed") : t("match.hud.pause")}
                    onClick={thenClose(onPause)}
                    disabled={!canPause}
                    style={ROW_STYLE}
                    data-testid="hud-menu-pause"
                  />
                )}
                <HUDButton
                  variant="danger"
                  icon={<Flag className="h-4 w-4" aria-hidden="true" />}
                  label={t("match.surrender.requestButton")}
                  onClick={thenClose(onSurrender)}
                  disabled={!canSurrender}
                  style={ROW_STYLE}
                  data-testid="hud-menu-surrender"
                />
              </>
            )}
            <HUDButton
              icon={<HelpCircle className="h-4 w-4" aria-hidden="true" />}
              label={t("match.hud.rules")}
              onClick={thenClose(onRules)}
              style={ROW_STYLE}
              data-testid="hud-menu-rules"
            />
            <HUDButton
              icon={<SettingsIcon className="h-4 w-4" aria-hidden="true" />}
              label={t("match.hud.settings")}
              onClick={thenClose(onSettings)}
              style={ROW_STYLE}
              data-testid="hud-menu-settings"
            />
            <HUDButton
              icon={<MuteIcon className="h-4 w-4" aria-hidden="true" />}
              label={audioOn ? t("match.hud.mute") : t("match.hud.unmute")}
              onClick={thenClose(onToggleMute)}
              style={ROW_STYLE}
              data-testid="hud-menu-mute"
            />
            {showEmotes && (
              <EmotePickerButton
                onSend={onSendEmote}
                onSent={() => setOpen(false)}
                label={t("match.emote.button")}
                openDirection="down"
              />
            )}
          </div>
        </>
      )}
    </div>
  );
}
