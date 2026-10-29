import { ArrowLeft, Plus } from "lucide-react";
import { useState } from "react";
import { useTranslation } from "react-i18next";
import { useNavigate } from "react-router";

import { CreateRoomPreview } from "@/features/room/components/CreateRoomPreview";
import { FetchError } from "@/shared/api/axiosClient";
import { HonorShield } from "@/shared/components/HonorShield";
import { PasswordToggle } from "@/shared/components/PasswordToggle";
import { RULE_ICONS } from "@/shared/components/roomRules/roomRuleLines";
import { AppDialogTile } from "@/shared/components/ui/app-dialog";
import { Button } from "@/shared/components/ui/button";
import { CompactField } from "@/shared/components/ui/compact-field";
import { Dialog, DialogContent, DialogTitle } from "@/shared/components/ui/dialog";
import { Eyebrow } from "@/shared/components/ui/eyebrow";
import { FormError } from "@/shared/components/ui/form-error";
import { Input } from "@/shared/components/ui/input";
import { Segmented } from "@/shared/components/ui/segmented";
import { useCreateRoomMutation } from "@/shared/hooks/mutations/useRooms";
import { useLayoutBand } from "@/shared/hooks/useLayoutBand";
import { formatCoins } from "@/shared/lib/formatCoins";
import {
  HONOR_TIER_BANDS,
  honorFloorLabel,
  honorIsNewPlayer,
  honorScoreOrPrior,
  honorTierForScore,
} from "@/shared/lib/honor";
import { variantLabel } from "@/shared/lib/roomLabels";
import { cn } from "@/shared/lib/utils";
import { useAuthStore } from "@/shared/stores/authStore";
import type { MatchMode } from "@/shared/types/matchTypes";

interface CreateRoomModalProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
}

const MIN_NAME = 3;
const MAX_NAME = 32;
const DEFAULT_BUY_IN = 500;
const BUY_IN_PRESETS = [0, 100, 500, 1000];
const MAX_BUY_IN_DIGITS = 7;
const MOVE_SECONDS = [10, 20, 30, 60, 90];
const DEFAULT_MOVE_SECONDS = 20;
const MIN_ROOM_PASSWORD = 4;
const MAX_ROOM_PASSWORD = 72;
const MIN_HONOR_FLOOR = 0;
const MIN_HONOR_CEILING = 100;
const MIN_HONOR_PRESETS = HONOR_TIER_BANDS.map((b) => b.from);

const SEGMENTED = "[&>button]:gap-1 [&>button]:px-2 lg:[&>button]:py-1.5";
const BUY_IN_CELL =
  "min-w-0 flex-1 rounded-sm bg-transparent px-2 text-center text-sm font-medium tabular-nums outline-none transition-[background-color,box-shadow,color] placeholder:text-ink-dim focus-visible:ring-3 focus-visible:ring-ring/70";
const BUY_IN_CELL_ACTIVE = "bg-surface-elevated text-ink shadow-xs ring-1 ring-border";

/**
 * The create-room form: a full-screen page on phones, a dialog with the form in two columns on tablets, and on
 * desktops the same dialog beside a live preview of the lobby card. A name left as suggested goes up blank, so the
 * server names the room after its host.
 */
export function CreateRoomModal({ open, onOpenChange }: CreateRoomModalProps) {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const band = useLayoutBand();
  const phone = band === "phone";
  const desktop = band === "desktop";
  const createRoomMutation = useCreateRoomMutation();
  const meUsername = useAuthStore((s) => s.user?.username ?? "");
  const meAvatarUrl = useAuthStore((s) => s.user?.avatarUrl ?? null);
  const meBalance = useAuthStore((s) => s.user?.walletBalance ?? 0);
  const meHonor = useAuthStore((s) => honorScoreOrPrior(s.user?.honorScore));
  const meIsNewPlayer = useAuthStore((s) => honorIsNewPlayer(s.user?.isNewPlayer));
  const honorKnown = useAuthStore(
    (s) => typeof s.user?.isNewPlayer === "boolean" && typeof s.user?.honorScore === "number",
  );

  const [name, setName] = useState("");
  const [nameEdited, setNameEdited] = useState(false);
  const [variant, setVariant] = useState<"bitola" | "croatia">("croatia");
  const [matchMode, setMatchMode] = useState<MatchMode>("1001");
  const [declarationsEnabled, setDeclarationsEnabled] = useState(true);
  const [stopAtTarget, setStopAtTarget] = useState(false);
  const [declarationsTransfer, setDeclarationsTransfer] = useState(true);
  const [clockwise, setClockwise] = useState(false);
  const [timerStyle, setTimerStyle] = useState<"relaxed" | "per-move">("per-move");
  const [timerDuration, setTimerDuration] = useState(DEFAULT_MOVE_SECONDS);
  const [presetBuyIn, setPresetBuyIn] = useState(DEFAULT_BUY_IN);
  const [customBuyIn, setCustomBuyIn] = useState("");
  const [isPrivate, setIsPrivate] = useState(false);
  const [roomPassword, setRoomPassword] = useState("");
  const [showPassword, setShowPassword] = useState(false);
  const [minHonor, setMinHonor] = useState(0);
  const [allowNewPlayers, setAllowNewPlayers] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [formError, setFormError] = useState<string | null>(null);

  const defaultName = t("lobby.createRoomModal.defaultName", { username: meUsername });
  const shownName = nameEdited ? name : defaultName;
  const trimmed = shownName.trim();
  const usesDefaultName = trimmed === "" || trimmed === defaultName;
  const nameValid = usesDefaultName || (trimmed.length >= MIN_NAME && trimmed.length <= MAX_NAME);
  const passwordValid =
    !isPrivate ||
    (roomPassword.length >= MIN_ROOM_PASSWORD &&
      new TextEncoder().encode(roomPassword).length <= MAX_ROOM_PASSWORD);
  const submitting = createRoomMutation.isPending;

  const usingCustomBuyIn = customBuyIn !== "";
  const effectiveBuyIn = usingCustomBuyIn ? Number(customBuyIn) : presetBuyIn;
  const buyInExceedsBalance = effectiveBuyIn > meBalance;

  const effectiveMinHonor = Math.min(MIN_HONOR_CEILING, Math.max(MIN_HONOR_FLOOR, minHonor));
  const failsOwnNewPlayerGate = honorKnown && meIsNewPlayer && !allowNewPlayers;
  const failsOwnHonorGate = honorKnown && meHonor < effectiveMinHonor;
  const failsOwnGate = failsOwnNewPlayerGate || failsOwnHonorGate;
  const effectiveDeclarationsTransfer = !declarationsEnabled || declarationsTransfer;

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();

    if (!nameValid) {
      setError(t("lobby.createRoomModal.errors.nameTooShort", { min: MIN_NAME }));
      return;
    }

    setError(null);
    setFormError(null);

    try {
      const room = await createRoomMutation.mutateAsync({
        name: usesDefaultName ? "" : trimmed,
        variant,
        matchMode,
        timerStyle,
        timerDurationSeconds: timerStyle === "per-move" ? timerDuration : null,
        coinBuyIn: effectiveBuyIn,
        isPrivate,
        password: isPrivate ? roomPassword : undefined,
        minHonor: effectiveMinHonor,
        allowNewPlayers,
        declarationsEnabled,
        stopAtTarget,
        declarationsTransfer: effectiveDeclarationsTransfer,
        clockwise,
      });
      onOpenChange(false);
      navigate(`/rooms/${room.id}`);
    } catch (err) {
      if (err instanceof FetchError) {
        if (err.code === "ROOM_NAME_TAKEN") {
          setError(t("lobby.createRoomModal.errors.nameTaken"));
        } else if (err.code === "ROOM_NAME_REQUIRED") {
          setError(t("lobby.createRoomModal.errors.nameRequired"));
        } else if (err.code === "ALREADY_IN_ROOM") {
          setFormError(t("lobby.createRoomModal.errors.alreadyInRoom"));
        } else if (err.code === "INSUFFICIENT_COINS") {
          setFormError(
            t("lobby.createRoomModal.errors.buyInTooHigh", { balance: formatCoins(meBalance) }),
          );
        } else if (err.code === "ROOM_PASSWORD_REQUIRED") {
          setFormError(t("lobby.createRoomModal.errors.passwordRequired"));
        } else if (err.code === "ROOM_PASSWORD_TOO_SHORT") {
          setFormError(
            t("lobby.createRoomModal.errors.passwordTooShort", { min: MIN_ROOM_PASSWORD }),
          );
        } else if (err.code === "ROOM_PASSWORD_TOO_LONG") {
          setFormError(
            t("lobby.createRoomModal.errors.passwordTooLong", { max: MAX_ROOM_PASSWORD }),
          );
        } else if (err.code === "INVALID_MIN_HONOR") {
          setFormError(
            t("lobby.createRoomModal.errors.invalidMinHonor", {
              min: MIN_HONOR_FLOOR,
              max: MIN_HONOR_CEILING,
            }),
          );
        } else if (err.code === "HONOR_TOO_LOW") {
          setFormError(
            t("lobby.createRoomModal.errors.ownHonorTooLow", {
              minHonor: effectiveMinHonor,
              honor: meHonor,
            }),
          );
        } else if (err.code === "NEW_PLAYER_NOT_ALLOWED") {
          setFormError(t("lobby.createRoomModal.errors.ownNewPlayerNotAllowed"));
        } else {
          setFormError(t("lobby.createRoomModal.errors.unexpected"));
        }
      } else {
        setFormError(t("lobby.createRoomModal.errors.unexpected"));
      }
    }
  }

  function handleOpenChange(nextOpen: boolean) {
    if (!nextOpen) {
      setName("");
      setNameEdited(false);
      setVariant("croatia");
      setMatchMode("1001");
      setDeclarationsEnabled(true);
      setStopAtTarget(false);
      setDeclarationsTransfer(true);
      setClockwise(false);
      setTimerStyle("per-move");
      setTimerDuration(DEFAULT_MOVE_SECONDS);
      setPresetBuyIn(DEFAULT_BUY_IN);
      setCustomBuyIn("");
      setIsPrivate(false);
      setRoomPassword("");
      setShowPassword(false);
      setMinHonor(0);
      setAllowNewPlayers(true);
      setError(null);
      setFormError(null);
      createRoomMutation.reset();
    }
    onOpenChange(nextOpen);
  }

  const minHonorBands = MIN_HONOR_PRESETS.filter((v) => v > 0)
    .map((v) => `${t(`profile.honor.tierPlural.${honorTierForScore(v)}`)} ${honorFloorLabel(v)}`)
    .join(", ");
  const minHonorInfo = (
    <>
      <p className="m-0">
        {t("lobby.createRoomModal.minHonorInfoBands", { bands: minHonorBands })}
      </p>
      {honorKnown && (
        <p className="m-0 mt-1.5">
          {t("lobby.createRoomModal.minHonorInfoYou", {
            honor: meHonor,
            tier: t(`profile.honor.tier.${honorTierForScore(meHonor)}`),
          })}
        </p>
      )}
    </>
  );

  const pair = phone ? "flex flex-col gap-4" : "grid grid-cols-2 gap-x-4 gap-y-3.5";
  const rulesPair = phone
    ? "flex flex-col gap-4"
    : "grid grid-cols-[minmax(0,1fr)_minmax(0,1.25fr)] gap-x-4 gap-y-3.5";

  const fields = (
    <div className={cn("flex flex-col", phone ? "gap-4" : "gap-3.5")}>
      <div className="flex flex-col gap-4">
        <CompactField
          label={t("lobby.createRoomModal.roomName")}
          htmlFor="room-name"
          info={t("lobby.createRoomModal.roomNameInfo", { min: MIN_NAME, max: MAX_NAME })}
          infoTestId="room-name-info"
          error={error ?? undefined}
          errorTestId="room-name-error"
        >
          <Input
            id="room-name"
            placeholder={defaultName}
            value={shownName}
            onChange={(e) => {
              setName(e.target.value.slice(0, MAX_NAME));
              setNameEdited(true);
              if (error) setError(null);
            }}
            aria-invalid={!!error}
            data-testid="room-name-input"
            maxLength={MAX_NAME}
            className="h-11 text-base font-semibold lg:h-10"
          />
        </CompactField>

        <div className={pair}>
          <CompactField
            label={t("lobby.createRoomModal.variant")}
            info={t("lobby.createRoomModal.variantHint")}
            infoTestId="variant-info"
          >
            <Segmented
              value={variant}
              onValueChange={setVariant}
              options={(["bitola", "croatia"] as const).map((value) => ({
                value,
                label: variantLabel(t, value),
              }))}
              className={SEGMENTED}
              testId="variant-segmented"
              ariaLabel={t("lobby.createRoomModal.variant")}
            />
          </CompactField>
          <CompactField
            label={t("lobby.createRoomModal.matchMode")}
            info={t("lobby.createRoomModal.matchModeHint")}
            infoTestId="match-mode-info"
          >
            <Segmented
              value={matchMode}
              onValueChange={setMatchMode}
              options={(["501", "701", "1001"] as const).map((value) => ({ value, label: value }))}
              className={SEGMENTED}
              testId="match-mode-segmented"
              ariaLabel={t("lobby.createRoomModal.matchMode")}
            />
          </CompactField>

          <CompactField label={t("lobby.createRoomModal.timerDuration")}>
            <Segmented
              value={timerStyle === "relaxed" ? "relaxed" : String(timerDuration)}
              onValueChange={(v) => {
                if (v === "relaxed") {
                  setTimerStyle("relaxed");
                } else {
                  setTimerStyle("per-move");
                  setTimerDuration(Number(v));
                }
              }}
              options={[
                ...MOVE_SECONDS.map((s) => ({ value: String(s), label: String(s) })),
                {
                  value: "relaxed",
                  label: (
                    <span className="sr-only">{t("lobby.createRoomModal.timerHintRelaxed")}</span>
                  ),
                  icon: <RULE_ICONS.relaxedTimer className="size-4" aria-hidden="true" />,
                },
              ]}
              className={SEGMENTED}
              testId="timer-segmented"
              ariaLabel={t("lobby.createRoomModal.timerDuration")}
            />
          </CompactField>
          <CompactField
            label={t("lobby.createRoomModal.coinBuyIn")}
            info={t("lobby.createRoomModal.coinBuyInHint")}
            infoTestId="coin-buy-in-info"
            meta={t("lobby.createRoomModal.coinBuyInBalance", { balance: formatCoins(meBalance) })}
            error={
              buyInExceedsBalance
                ? t("lobby.createRoomModal.errors.buyInTooHigh", {
                    balance: formatCoins(meBalance),
                  })
                : undefined
            }
            errorTestId="buy-in-error"
          >
            <Segmented
              value={usingCustomBuyIn ? "" : String(presetBuyIn)}
              onValueChange={(v) => {
                setCustomBuyIn("");
                setPresetBuyIn(Number(v));
              }}
              options={BUY_IN_PRESETS.map((v) => ({ value: String(v), label: formatCoins(v) }))}
              className={SEGMENTED}
              testId="coin-buy-in-segmented"
              ariaLabel={t("lobby.createRoomModal.coinBuyIn")}
              trailing={
                <input
                  inputMode="numeric"
                  value={customBuyIn}
                  onChange={(e) =>
                    setCustomBuyIn(e.target.value.replace(/\D/g, "").slice(0, MAX_BUY_IN_DIGITS))
                  }
                  placeholder={t("lobby.createRoomModal.coinBuyInCustom")}
                  aria-label={t("lobby.createRoomModal.coinBuyInCustom")}
                  data-testid="coin-buy-in-input"
                  className={cn(BUY_IN_CELL, usingCustomBuyIn && BUY_IN_CELL_ACTIVE)}
                />
              }
            />
          </CompactField>
        </div>
      </div>

      <div className={rulesPair}>
        <CompactField
          testId="field-declarations-segmented"
          label={t("lobby.createRoomModal.declarations")}
          info={t("lobby.createRoomModal.declarationsHint")}
          infoTestId="declarations-info-toggle"
          changed={!declarationsEnabled}
        >
          <Segmented
            value={declarationsEnabled ? "on" : "off"}
            onValueChange={(v) => setDeclarationsEnabled(v === "on")}
            options={[
              {
                value: "on" as const,
                label: t("lobby.createRoomModal.declarationsOn"),
                icon: <RULE_ICONS.declarationsOn className="size-3.5" />,
              },
              {
                value: "off" as const,
                label: t("lobby.createRoomModal.declarationsOff"),
                icon: <RULE_ICONS.declarationsOff className="size-3.5" />,
              },
            ]}
            className={SEGMENTED}
            testId="declarations-segmented"
            ariaLabel={t("lobby.createRoomModal.declarations")}
          />
        </CompactField>
        {declarationsEnabled && (
          <CompactField
            testId="field-declarations-transfer-segmented"
            label={t("lobby.createRoomModal.declarationsTransfer")}
            info={t("lobby.createRoomModal.declarationsTransferHint")}
            infoTestId="declarations-transfer-info-toggle"
            changed={!declarationsTransfer}
          >
            <Segmented
              value={declarationsTransfer ? "transfer" : "void"}
              onValueChange={(v) => setDeclarationsTransfer(v === "transfer")}
              options={[
                {
                  value: "transfer" as const,
                  label: t("lobby.createRoomModal.declarationsTransferOn"),
                  icon: <RULE_ICONS.fallTransfer className="size-3.5" />,
                },
                {
                  value: "void" as const,
                  label: t("lobby.createRoomModal.declarationsTransferOff"),
                  icon: <RULE_ICONS.fallVoided className="size-3.5" />,
                },
              ]}
              className={SEGMENTED}
              testId="declarations-transfer-segmented"
              ariaLabel={t("lobby.createRoomModal.declarationsTransfer")}
            />
          </CompactField>
        )}
        <CompactField
          testId="field-match-end-segmented"
          label={t("lobby.createRoomModal.matchEnd")}
          info={t("lobby.createRoomModal.matchEndHint")}
          infoTestId="match-end-info-toggle"
          changed={stopAtTarget}
        >
          <Segmented
            value={stopAtTarget ? "stop" : "finish"}
            onValueChange={(v) => setStopAtTarget(v === "stop")}
            options={[
              {
                value: "finish" as const,
                label: t("lobby.createRoomModal.matchEndFinish"),
                icon: <RULE_ICONS.finishHand className="size-3.5" />,
              },
              {
                value: "stop" as const,
                label: t("lobby.createRoomModal.matchEndStop"),
                icon: <RULE_ICONS.stopAtTarget className="size-3.5" />,
              },
            ]}
            className={SEGMENTED}
            testId="match-end-segmented"
            ariaLabel={t("lobby.createRoomModal.matchEnd")}
          />
        </CompactField>
        <CompactField
          testId="field-direction-segmented"
          label={t("lobby.createRoomModal.direction")}
          info={t("lobby.createRoomModal.directionHint")}
          infoTestId="direction-info-toggle"
          changed={clockwise}
        >
          <Segmented
            value={clockwise ? "clockwise" : "counterClockwise"}
            onValueChange={(v) => setClockwise(v === "clockwise")}
            options={[
              {
                value: "counterClockwise" as const,
                label: t("lobby.createRoomModal.directionCounterClockwise"),
                icon: <RULE_ICONS.counterClockwise className="size-3.5" />,
              },
              {
                value: "clockwise" as const,
                label: t("lobby.createRoomModal.directionClockwise"),
                icon: <RULE_ICONS.clockwise className="size-3.5" />,
              },
            ]}
            className={SEGMENTED}
            testId="direction-segmented"
            ariaLabel={t("lobby.createRoomModal.direction")}
          />
        </CompactField>
      </div>

      <div className={pair}>
        <CompactField
          testId="field-private-room-toggle"
          label={t("lobby.createRoomModal.privateRoom")}
          info={t("lobby.createRoomModal.privateRoomHint")}
          infoTestId="private-room-info"
          changed={isPrivate}
        >
          <Segmented
            value={isPrivate ? "private" : "public"}
            onValueChange={(v) => {
              setIsPrivate(v === "private");
              if (v === "public") setRoomPassword("");
            }}
            options={[
              {
                value: "public" as const,
                label: t("lobby.createRoomModal.privacyPublic"),
                icon: <RULE_ICONS.public className="size-3.5" />,
              },
              {
                value: "private" as const,
                label: t("lobby.createRoomModal.privacyPrivate"),
                icon: <RULE_ICONS.private className="size-3.5" />,
              },
            ]}
            className={SEGMENTED}
            testId="private-room-toggle"
            ariaLabel={t("lobby.createRoomModal.privateRoom")}
          />
          {isPrivate && (
            <div className="relative">
              <Input
                id="room-password"
                type={showPassword ? "text" : "password"}
                autoComplete="new-password"
                value={roomPassword}
                onChange={(e) => setRoomPassword(e.target.value.slice(0, MAX_ROOM_PASSWORD))}
                maxLength={MAX_ROOM_PASSWORD}
                placeholder={t("lobby.createRoomModal.roomPassword")}
                aria-label={t("lobby.createRoomModal.roomPassword")}
                aria-describedby="room-password-hint"
                data-testid="room-password-input"
                className="h-10 pr-10"
              />
              <PasswordToggle
                shown={showPassword}
                onToggle={() => setShowPassword(!showPassword)}
                testId="room-password-toggle"
              />
            </div>
          )}
          {isPrivate && !passwordValid && (
            <p id="room-password-hint" className="text-ink-mute m-0 text-xs">
              {t("lobby.createRoomModal.roomPasswordHint", { min: MIN_ROOM_PASSWORD })}
            </p>
          )}
        </CompactField>
        <CompactField
          testId="field-allow-new-players-toggle"
          label={t("lobby.createRoomModal.allowNewPlayers")}
          info={t("lobby.createRoomModal.allowNewPlayersHint")}
          infoTestId="allow-new-players-info"
          changed={!allowNewPlayers}
          error={
            failsOwnNewPlayerGate
              ? t("lobby.createRoomModal.errors.ownNewPlayerNotAllowed")
              : undefined
          }
          errorTestId="allow-new-players-error"
        >
          <Segmented
            value={allowNewPlayers ? "allow" : "veterans"}
            onValueChange={(v) => setAllowNewPlayers(v === "allow")}
            options={[
              {
                value: "allow" as const,
                label: t("lobby.createRoomModal.allowNewPlayersYes"),
                icon: <RULE_ICONS.newPlayers className="size-3.5" />,
              },
              {
                value: "veterans" as const,
                label: t("lobby.createRoomModal.allowNewPlayersNo"),
                icon: <RULE_ICONS.veteransOnly className="size-3.5" />,
              },
            ]}
            className={SEGMENTED}
            testId="allow-new-players-toggle"
            ariaLabel={t("lobby.createRoomModal.allowNewPlayers")}
          />
        </CompactField>
        <CompactField
          testId="field-min-honor-segmented"
          label={t("lobby.createRoomModal.minHonor")}
          info={minHonorInfo}
          infoTestId="min-honor-info"
          changed={effectiveMinHonor > 0}
          error={
            failsOwnHonorGate
              ? t("lobby.createRoomModal.errors.ownHonorTooLow", {
                  minHonor: effectiveMinHonor,
                  honor: meHonor,
                })
              : undefined
          }
          errorTestId="min-honor-error"
          className={phone ? undefined : "col-span-2"}
        >
          <Segmented
            value={String(effectiveMinHonor)}
            onValueChange={(v) => setMinHonor(Number(v))}
            options={MIN_HONOR_PRESETS.map((v) =>
              v === 0
                ? { value: "0", label: t("lobby.createRoomModal.minHonorAnyone") }
                : {
                    value: String(v),
                    label: (
                      <>
                        {honorFloorLabel(v)}
                        <span className="sr-only">
                          {" "}
                          {t(`profile.honor.tierPlural.${honorTierForScore(v)}`)}
                        </span>
                      </>
                    ),
                    icon: <HonorShield tier={honorTierForScore(v)} size={12} />,
                  },
            )}
            className={SEGMENTED}
            testId="min-honor-segmented"
            ariaLabel={t("lobby.createRoomModal.minHonor")}
          />
        </CompactField>
      </div>
    </div>
  );

  return (
    <Dialog open={open} onOpenChange={handleOpenChange}>
      <DialogContent
        className={cn(
          "bg-surface ring-brass-soft flex flex-col gap-0 overflow-hidden p-0 shadow-xl",
          phone
            ? "top-0 left-0 h-dvh max-h-none w-full max-w-none translate-x-0 translate-y-0 rounded-none ring-0 sm:max-w-none"
            : desktop
              ? "max-h-[min(760px,calc(100dvh-4rem))] w-[calc(100%-4rem)] max-w-[1120px] rounded-xl sm:max-w-[1120px]"
              : "max-h-[min(880px,calc(100dvh-4rem))] w-[calc(100%-4rem)] max-w-[756px] rounded-xl sm:max-w-[756px]",
        )}
        showCloseButton={false}
        data-testid="create-room-modal"
        data-layout={band}
      >
        <form
          onSubmit={handleSubmit}
          className={cn(
            "min-h-0 flex-1",
            desktop ? "grid grid-cols-[minmax(0,1fr)_340px]" : "flex flex-col",
          )}
          noValidate
        >
          <div className="bg-surface flex min-h-0 flex-1 flex-col">
            {phone ? (
              <header className="border-border flex h-14 shrink-0 items-center border-b px-2">
                <Button
                  type="button"
                  variant="ghost"
                  size="icon"
                  onClick={() => handleOpenChange(false)}
                  aria-label={t("lobby.createRoomModal.cancel")}
                  data-testid="cancel-button"
                >
                  <ArrowLeft className="size-5" />
                </Button>
                <DialogTitle className="text-ink m-0 flex-1 text-center text-base font-semibold">
                  {t("lobby.createRoomModal.headline")}
                </DialogTitle>
                <span className="size-9" aria-hidden="true" />
              </header>
            ) : (
              <header className="flex shrink-0 items-center gap-4 px-7 pt-6 pb-2">
                <AppDialogTile>
                  <Plus />
                </AppDialogTile>
                <DialogTitle className="text-ink text-xl leading-tight font-semibold tracking-tight">
                  {t("lobby.createRoomModal.headline")}
                </DialogTitle>
              </header>
            )}

            <div
              className={cn(
                "min-h-0 flex-1 overflow-y-auto",
                phone ? "px-4 pt-4 pb-6" : "px-7 pt-3 pb-6",
              )}
            >
              {fields}
            </div>

            {formError && (
              <FormError
                testId="create-room-form-error"
                className={cn("shrink-0 pt-3", phone ? "px-4" : "px-7")}
              >
                {formError}
              </FormError>
            )}

            <footer
              className={cn(
                "border-border flex shrink-0 gap-2 border-t",
                phone ? "bg-surface px-4 pt-3 pb-4" : "bg-surface-sunken/50 justify-end px-7 py-4",
              )}
            >
              <Button
                type="submit"
                size="cta"
                className={phone ? "w-full text-base" : undefined}
                disabled={
                  !nameValid || submitting || buyInExceedsBalance || !passwordValid || failsOwnGate
                }
                data-testid="create-room-button"
              >
                {submitting
                  ? t("lobby.createRoomModal.submitting")
                  : t("lobby.createRoomModal.submit")}
              </Button>
              {!phone && (
                <Button
                  type="button"
                  variant="ghost"
                  size="cta"
                  onClick={() => handleOpenChange(false)}
                  data-testid="cancel-button"
                >
                  {t("lobby.createRoomModal.cancel")}
                </Button>
              )}
            </footer>
          </div>

          {desktop && (
            <aside
              className="border-border flex min-h-0 flex-col gap-4 overflow-y-auto border-l px-7 py-6"
              style={{
                background:
                  "radial-gradient(ellipse 90% 60% at 50% -10%, var(--brass-soft), transparent 70%), var(--surface-3)",
              }}
            >
              <Eyebrow>{t("lobby.createRoomModal.preview.title")}</Eyebrow>

              <CreateRoomPreview
                name={trimmed || defaultName}
                variant={variant}
                matchMode={matchMode}
                timerStyle={timerStyle}
                timerDuration={timerDuration}
                coinBuyIn={effectiveBuyIn}
                isPrivate={isPrivate}
                minHonor={effectiveMinHonor}
                allowNewPlayers={allowNewPlayers}
                declarationsEnabled={declarationsEnabled}
                stopAtTarget={stopAtTarget}
                declarationsTransfer={effectiveDeclarationsTransfer}
                clockwise={clockwise}
                hostUsername={meUsername || "host"}
                hostAvatarUrl={meAvatarUrl}
              />
            </aside>
          )}
        </form>
      </DialogContent>
    </Dialog>
  );
}
