import { ChevronDown, Coins, Plus, Settings, Shuffle } from "lucide-react";
import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { useNavigate } from "react-router";

import { CreateRoomPreview } from "@/features/room/components/CreateRoomPreview";
import { FetchError } from "@/shared/api/axiosClient";
import { HonorShield } from "@/shared/components/HonorShield";
import { PasswordToggle } from "@/shared/components/PasswordToggle";
import { RULE_ICONS } from "@/shared/components/roomRules/roomRuleLines";
import { AppDialogTile } from "@/shared/components/ui/app-dialog";
import { Badge } from "@/shared/components/ui/badge";
import { Button } from "@/shared/components/ui/button";
import { Chips } from "@/shared/components/ui/chips";
import { Dialog, DialogContent, DialogTitle } from "@/shared/components/ui/dialog";
import { DurationSlider } from "@/shared/components/ui/duration-slider";
import { Eyebrow } from "@/shared/components/ui/eyebrow";
import { Field } from "@/shared/components/ui/field";
import { FormError } from "@/shared/components/ui/form-error";
import { Input } from "@/shared/components/ui/input";
import { Segmented } from "@/shared/components/ui/segmented";
import { useCreateRoomMutation } from "@/shared/hooks/mutations/useRooms";
import { COIN_GOLD } from "@/shared/lib/coinGold";
import { formatCoins } from "@/shared/lib/formatCoins";
import {
  HONOR_TIER_BANDS,
  HONOR_TIER_COLOR,
  honorFloorLabel,
  honorIsNewPlayer,
  honorScoreOrPrior,
  honorTierForScore,
} from "@/shared/lib/honor";
import { randomRoomName } from "@/shared/lib/randomRoomName";
import { modeOptionLabel, variantLabel } from "@/shared/lib/roomLabels";
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
const MIN_ROOM_PASSWORD = 4;
const MAX_ROOM_PASSWORD = 72;
const MIN_HONOR_FLOOR = 0;
const MIN_HONOR_CEILING = 100;
const DEFAULT_MIN_HONOR = 0;

const MIN_HONOR_PRESETS = HONOR_TIER_BANDS.map((b) => b.from);

/**
 * Split-panel create-room modal: the form on the left, with rarely changed settings in an
 * "Advanced settings" accordion, and a live preview of the resulting lobby card on the right.
 */
export function CreateRoomModal({ open, onOpenChange }: CreateRoomModalProps) {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const createRoomMutation = useCreateRoomMutation();
  const meUsername = useAuthStore((s) => s.user?.username ?? "");
  const meAvatarUrl = useAuthStore((s) => s.user?.avatarUrl ?? null);
  const meBalance = useAuthStore((s) => s.user?.walletBalance ?? 0);
  const meHonor = useAuthStore((s) => honorScoreOrPrior(s.user?.honorScore));
  const meIsNewPlayer = useAuthStore((s) => honorIsNewPlayer(s.user?.isNewPlayer));
  const honorKnown = useAuthStore(
    (s) => typeof s.user?.isNewPlayer === "boolean" && typeof s.user?.honorScore === "number",
  );

  const [name, setName] = useState(() => randomRoomName(t));
  const [nameShuffleSpin, setNameShuffleSpin] = useState(0);
  const [variant, setVariant] = useState<"bitola" | "croatia">("bitola");
  const [matchMode, setMatchMode] = useState<MatchMode>("1001");
  const [declarationsEnabled, setDeclarationsEnabled] = useState(true);
  const [stopAtTarget, setStopAtTarget] = useState(false);
  const [declarationsTransfer, setDeclarationsTransfer] = useState(true);
  const [clockwise, setClockwise] = useState(false);
  const [timerStyle, setTimerStyle] = useState<"relaxed" | "per-move">("relaxed");
  const [timerDuration, setTimerDuration] = useState(30);
  const [coinBuyIn, setCoinBuyIn] = useState(DEFAULT_BUY_IN);
  const [customBuyIn, setCustomBuyIn] = useState(false);
  const [isPrivate, setIsPrivate] = useState(false);
  const [roomPassword, setRoomPassword] = useState("");
  const [showPassword, setShowPassword] = useState(false);
  const [minHonor, setMinHonor] = useState(DEFAULT_MIN_HONOR);
  const [allowNewPlayers, setAllowNewPlayers] = useState(true);
  const [advOpen, setAdvOpen] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [formError, setFormError] = useState<string | null>(null);

  const trimmed = name.trim();
  const nameValid = trimmed.length >= MIN_NAME && trimmed.length <= MAX_NAME;
  const passwordValid =
    !isPrivate ||
    (roomPassword.length >= MIN_ROOM_PASSWORD &&
      new TextEncoder().encode(roomPassword).length <= MAX_ROOM_PASSWORD);
  const submitting = createRoomMutation.isPending;

  const effectiveBuyIn = Math.max(0, Math.floor(coinBuyIn || 0));
  const buyInExceedsBalance = effectiveBuyIn > meBalance;

  const effectiveMinHonor = Math.min(
    MIN_HONOR_CEILING,
    Math.max(MIN_HONOR_FLOOR, Math.floor(minHonor || 0)),
  );
  const failsOwnNewPlayerGate = honorKnown && meIsNewPlayer && !allowNewPlayers;
  const failsOwnHonorGate = honorKnown && meHonor < effectiveMinHonor;
  const failsOwnGate = failsOwnNewPlayerGate || failsOwnHonorGate;
  const effectiveDeclarationsTransfer = !declarationsEnabled || declarationsTransfer;

  useEffect(() => {
    if (!advOpen && (failsOwnGate || (isPrivate && !passwordValid))) setAdvOpen(true);
  }, [advOpen, failsOwnGate, isPrivate, passwordValid]);

  const advancedChangedCount = [
    declarationsEnabled !== true,
    stopAtTarget !== false,
    effectiveDeclarationsTransfer !== true,
    clockwise !== false,
    isPrivate !== false,
    effectiveMinHonor !== DEFAULT_MIN_HONOR,
    allowNewPlayers !== true,
  ].filter(Boolean).length;

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();

    if (!trimmed) {
      setError(t("lobby.createRoomModal.errors.nameRequired"));
      return;
    }
    if (trimmed.length < MIN_NAME) {
      setError(t("lobby.createRoomModal.errors.nameTooShort", { min: MIN_NAME }));
      return;
    }

    setError(null);
    setFormError(null);

    try {
      const room = await createRoomMutation.mutateAsync({
        name: trimmed,
        variant,
        matchMode,
        timerStyle,
        timerDurationSeconds: timerStyle === "per-move" ? timerDuration : null,
        coinBuyIn: Math.max(0, Math.floor(coinBuyIn || 0)),
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
      setName(randomRoomName(t));
      setVariant("bitola");
      setMatchMode("1001");
      setDeclarationsEnabled(true);
      setStopAtTarget(false);
      setDeclarationsTransfer(true);
      setClockwise(false);
      setTimerStyle("relaxed");
      setTimerDuration(30);
      setCoinBuyIn(DEFAULT_BUY_IN);
      setCustomBuyIn(false);
      setIsPrivate(false);
      setRoomPassword("");
      setShowPassword(false);
      setMinHonor(DEFAULT_MIN_HONOR);
      setAllowNewPlayers(true);
      setAdvOpen(false);
      setError(null);
      setFormError(null);
      createRoomMutation.reset();
    }
    onOpenChange(nextOpen);
  }

  const variantOptions = (["bitola", "croatia"] as const).map((value) => ({
    value,
    label: variantLabel(t, value),
  }));

  const matchModeOptions = (["501", "701", "1001"] as const).map((value) => ({
    value,
    label: modeOptionLabel(t, value),
  }));

  const declarationsOptions = [
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
  ];

  const matchEndOptions = [
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
  ];

  const declarationsTransferOptions = [
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
  ];

  const directionOptions = [
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
  ];

  const timerOptions = [
    {
      value: "relaxed" as const,
      label: t("lobby.createRoomModal.timerRelaxed"),
      icon: <RULE_ICONS.relaxedTimer className="size-3.5" />,
    },
    {
      value: "per-move" as const,
      label: t("lobby.createRoomModal.timerPerMove"),
      icon: <RULE_ICONS.moveTimer className="size-3.5" />,
    },
  ];

  const buyInOptions = [
    ...BUY_IN_PRESETS.map((value) => ({
      value: String(value),
      label: value > 0 ? formatCoins(value) : t("lobby.card.buyInFree"),
    })),
    { value: "custom", label: t("lobby.createRoomModal.coinBuyInCustom") },
  ];
  const buyInChipValue = customBuyIn ? "custom" : String(coinBuyIn);

  const minHonorOptions = MIN_HONOR_PRESETS.map((value) => ({
    value,
    tone: value === 0 ? undefined : HONOR_TIER_COLOR[honorTierForScore(value)],
    label:
      value === 0 ? (
        t("lobby.createRoomModal.minHonorAnyone")
      ) : (
        <span className="inline-flex items-center gap-1">
          <HonorShield tier={honorTierForScore(value)} size={12} />
          {t(`profile.honor.tierPlural.${honorTierForScore(value)}`)}
          <span className="font-mono text-2xs tracking-wider opacity-70">
            {honorFloorLabel(value)}
          </span>
        </span>
      ),
  }));
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

  return (
    <Dialog open={open} onOpenChange={handleOpenChange}>
      <DialogContent
        className="bg-surface ring-brass-soft flex max-h-[calc(100dvh-2rem)] w-[calc(100%-2rem)] flex-col gap-0 overflow-hidden rounded-xl p-0 shadow-xl sm:max-w-[min(1100px,calc(100vw-4rem))] md:max-h-[min(720px,calc(100vh-4rem))]"
        showCloseButton={false}
        data-testid="create-room-modal"
      >
        <form
          onSubmit={handleSubmit}
          className="grid min-h-0 flex-1 grid-cols-1 grid-rows-[minmax(0,1fr)] md:grid-cols-[1.05fr_0.95fr]"
          noValidate
        >
          <div className="bg-surface flex min-h-0 flex-col">
            <header className="flex shrink-0 items-center gap-4 px-8 pt-6 pb-1.5">
              <AppDialogTile>
                <Plus />
              </AppDialogTile>
              <DialogTitle className="text-ink text-xl leading-tight font-semibold tracking-tight">
                {t("lobby.createRoomModal.headline")}
              </DialogTitle>
            </header>

            <div className="flex min-h-0 flex-1 flex-col gap-4 overflow-y-auto px-8 py-4">
              <Field
                label={t("lobby.createRoomModal.roomName")}
                htmlFor="room-name"
                info={t("lobby.createRoomModal.roomNameInfo", { min: MIN_NAME, max: MAX_NAME })}
                infoTestId="room-name-info"
                error={error ?? undefined}
                errorTestId="room-name-error"
              >
                <div className="relative">
                  <Input
                    id="room-name"
                    placeholder={t("lobby.createRoomModal.roomNamePlaceholder")}
                    value={name}
                    onChange={(e) => {
                      setName(e.target.value.slice(0, MAX_NAME));
                      if (error) setError(null);
                    }}
                    aria-invalid={!!error}
                    data-testid="room-name-input"
                    maxLength={MAX_NAME}
                    className="h-11 pr-11 text-base font-semibold"
                  />
                  <button
                    type="button"
                    onClick={() => {
                      setName(randomRoomName(t, name));
                      setNameShuffleSpin((n) => n + 1);
                      if (error) setError(null);
                    }}
                    aria-label={t("lobby.createRoomModal.roomNameShuffleAria")}
                    data-testid="room-name-shuffle"
                    className="text-brass-deep bg-brass-soft border-brass/55 absolute top-1/2 right-1.5 -translate-y-1/2 inline-flex size-8 items-center justify-center rounded-sm border"
                  >
                    <span
                      key={nameShuffleSpin}
                      style={{ display: "flex", animation: "spin .45s ease" }}
                    >
                      <Shuffle className="size-4" />
                    </span>
                  </button>
                </div>
              </Field>

              <Field
                label={t("lobby.createRoomModal.variant")}
                info={t("lobby.createRoomModal.variantHint")}
                infoTestId="variant-info"
              >
                <Segmented
                  value={variant}
                  onValueChange={setVariant}
                  options={variantOptions}
                  testId="variant-segmented"
                  ariaLabel={t("lobby.createRoomModal.variant")}
                />
              </Field>

              <Field
                label={t("lobby.createRoomModal.matchMode")}
                info={t("lobby.createRoomModal.matchModeHint")}
                infoTestId="match-mode-info"
              >
                <Segmented
                  value={matchMode}
                  onValueChange={setMatchMode}
                  options={matchModeOptions}
                  dense
                  className="[&>button]:px-1.5 [&>button]:py-2"
                  testId="match-mode-segmented"
                  ariaLabel={t("lobby.createRoomModal.matchMode")}
                />
              </Field>

              <Field
                label={t("lobby.createRoomModal.timerStyle")}
                info={
                  timerStyle === "relaxed" ? t("lobby.createRoomModal.timerHintRelaxed") : undefined
                }
                infoTestId="timer-style-info"
              >
                <Segmented
                  value={timerStyle}
                  onValueChange={setTimerStyle}
                  options={timerOptions}
                  testId="timer-style-segmented"
                  ariaLabel={t("lobby.createRoomModal.timerStyle")}
                />
              </Field>

              {timerStyle === "per-move" && (
                <Field
                  label={t("lobby.createRoomModal.timerDuration")}
                  info={t("lobby.createRoomModal.timerDurationHint")}
                  infoTestId="timer-duration-info"
                >
                  <DurationSlider
                    value={timerDuration}
                    onChange={setTimerDuration}
                    min={10}
                    max={120}
                    step={5}
                    unitLabel={t("lobby.createRoomModal.timerDurationSuffix")}
                    testId="timer-duration-slider"
                  />
                </Field>
              )}

              <Field
                label={t("lobby.createRoomModal.coinBuyIn")}
                info={t("lobby.createRoomModal.coinBuyInHint")}
                infoTestId="coin-buy-in-info"
                hint={t("lobby.createRoomModal.coinBuyInBalance", {
                  balance: formatCoins(meBalance),
                })}
                error={
                  buyInExceedsBalance
                    ? t("lobby.createRoomModal.errors.buyInTooHigh", {
                        balance: formatCoins(meBalance),
                      })
                    : undefined
                }
                errorTestId="buy-in-error"
              >
                <div className="flex flex-col gap-2">
                  <Chips
                    value={buyInChipValue}
                    onValueChange={(v) => {
                      if (v === "custom") {
                        setCustomBuyIn(true);
                      } else {
                        setCustomBuyIn(false);
                        setCoinBuyIn(Number(v));
                      }
                    }}
                    options={buyInOptions}
                    ariaLabel={t("lobby.createRoomModal.coinBuyIn")}
                    testId="coin-buy-in-chips"
                  />
                  {customBuyIn && (
                    <div className="relative w-fit">
                      <Coins
                        className="pointer-events-none absolute top-1/2 left-3 size-4 -translate-y-1/2"
                        style={{ color: COIN_GOLD }}
                        aria-hidden="true"
                      />
                      <Input
                        id="coin-buy-in"
                        type="number"
                        min={0}
                        step={50}
                        inputMode="numeric"
                        value={coinBuyIn}
                        onChange={(e) =>
                          setCoinBuyIn(Math.max(0, Math.floor(Number(e.target.value) || 0)))
                        }
                        data-testid="coin-buy-in-input"
                        className="h-11 w-36 pl-9"
                      />
                    </div>
                  )}
                </div>
              </Field>

              <div className="bg-border my-0.5 h-px shrink-0" aria-hidden="true" />

              <AdvancedSettings
                open={advOpen}
                onToggle={() => setAdvOpen((v) => !v)}
                changedCount={advancedChangedCount}
              >
                <Field
                  label={t("lobby.createRoomModal.declarations")}
                  info={t("lobby.createRoomModal.declarationsHint")}
                  infoTestId="declarations-info-toggle"
                >
                  <Segmented
                    value={declarationsEnabled ? "on" : "off"}
                    onValueChange={(v) => setDeclarationsEnabled(v === "on")}
                    options={declarationsOptions}
                    testId="declarations-segmented"
                    ariaLabel={t("lobby.createRoomModal.declarations")}
                  />
                </Field>

                {declarationsEnabled && (
                  <Field
                    label={t("lobby.createRoomModal.declarationsTransfer")}
                    info={t("lobby.createRoomModal.declarationsTransferHint")}
                    infoTestId="declarations-transfer-info-toggle"
                  >
                    <Segmented
                      value={declarationsTransfer ? "transfer" : "void"}
                      onValueChange={(v) => setDeclarationsTransfer(v === "transfer")}
                      options={declarationsTransferOptions}
                      testId="declarations-transfer-segmented"
                      ariaLabel={t("lobby.createRoomModal.declarationsTransfer")}
                    />
                  </Field>
                )}

                <Field
                  label={t("lobby.createRoomModal.matchEnd")}
                  info={t("lobby.createRoomModal.matchEndHint")}
                  infoTestId="match-end-info-toggle"
                >
                  <Segmented
                    value={stopAtTarget ? "stop" : "finish"}
                    onValueChange={(v) => setStopAtTarget(v === "stop")}
                    options={matchEndOptions}
                    testId="match-end-segmented"
                    ariaLabel={t("lobby.createRoomModal.matchEnd")}
                  />
                </Field>

                <Field
                  label={t("lobby.createRoomModal.direction")}
                  info={t("lobby.createRoomModal.directionHint")}
                  infoTestId="direction-info-toggle"
                >
                  <Segmented
                    value={clockwise ? "clockwise" : "counterClockwise"}
                    onValueChange={(v) => setClockwise(v === "clockwise")}
                    options={directionOptions}
                    testId="direction-segmented"
                    ariaLabel={t("lobby.createRoomModal.direction")}
                  />
                </Field>

                <Field
                  label={t("lobby.createRoomModal.privateRoom")}
                  info={t("lobby.createRoomModal.privateRoomHint")}
                  infoTestId="private-room-info"
                >
                  <div className="flex flex-col gap-2">
                    <Segmented
                      value={isPrivate ? "private" : "public"}
                      onValueChange={(v) => {
                        setIsPrivate(v === "private");
                        if (v === "public") setRoomPassword("");
                      }}
                      options={[
                        {
                          value: "public",
                          label: t("lobby.createRoomModal.privacyPublic"),
                          icon: <RULE_ICONS.public className="size-3.5" />,
                        },
                        {
                          value: "private",
                          label: t("lobby.createRoomModal.privacyPrivate"),
                          icon: <RULE_ICONS.private className="size-3.5" />,
                        },
                      ]}
                      testId="private-room-toggle"
                      ariaLabel={t("lobby.createRoomModal.privateRoom")}
                    />

                    {isPrivate && (
                      <Field
                        label={t("lobby.createRoomModal.roomPassword")}
                        htmlFor="room-password"
                        hint={t("lobby.createRoomModal.roomPasswordHint", {
                          min: MIN_ROOM_PASSWORD,
                        })}
                        required
                      >
                        <div className="relative">
                          <Input
                            id="room-password"
                            type={showPassword ? "text" : "password"}
                            autoComplete="new-password"
                            value={roomPassword}
                            onChange={(e) =>
                              setRoomPassword(e.target.value.slice(0, MAX_ROOM_PASSWORD))
                            }
                            maxLength={MAX_ROOM_PASSWORD}
                            data-testid="room-password-input"
                            className="h-11 pr-10"
                          />
                          <PasswordToggle
                            shown={showPassword}
                            onToggle={() => setShowPassword(!showPassword)}
                            testId="room-password-toggle"
                          />
                        </div>
                      </Field>
                    )}
                  </div>
                </Field>

                <Field
                  label={t("lobby.createRoomModal.allowNewPlayers")}
                  info={t("lobby.createRoomModal.allowNewPlayersHint")}
                  infoTestId="allow-new-players-info"
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
                        value: "allow",
                        label: t("lobby.createRoomModal.allowNewPlayersYes"),
                        icon: <RULE_ICONS.newPlayers className="size-3.5" />,
                      },
                      {
                        value: "veterans",
                        label: t("lobby.createRoomModal.allowNewPlayersNo"),
                        icon: <RULE_ICONS.veteransOnly className="size-3.5" />,
                      },
                    ]}
                    testId="allow-new-players-toggle"
                    ariaLabel={t("lobby.createRoomModal.allowNewPlayers")}
                  />
                </Field>

                <Field
                  label={t("lobby.createRoomModal.minHonor")}
                  info={minHonorInfo}
                  infoTestId="min-honor-info"
                  error={
                    failsOwnHonorGate
                      ? t("lobby.createRoomModal.errors.ownHonorTooLow", {
                          minHonor: effectiveMinHonor,
                          honor: meHonor,
                        })
                      : undefined
                  }
                  errorTestId="min-honor-error"
                >
                  <Chips
                    value={effectiveMinHonor}
                    onValueChange={setMinHonor}
                    options={minHonorOptions}
                    ariaLabel={t("lobby.createRoomModal.minHonor")}
                    testId="min-honor-chips"
                  />
                </Field>
              </AdvancedSettings>
            </div>

            {formError && (
              <FormError testId="create-room-form-error" className="shrink-0 px-8 pt-3">
                {formError}
              </FormError>
            )}

            <footer className="border-border bg-surface-sunken/50 flex shrink-0 flex-col gap-2 border-t px-8 py-4 sm:flex-row sm:justify-end">
              <Button
                type="submit"
                size="cta"
                disabled={
                  !nameValid || submitting || buyInExceedsBalance || !passwordValid || failsOwnGate
                }
                data-testid="create-room-button"
              >
                {submitting
                  ? t("lobby.createRoomModal.submitting")
                  : t("lobby.createRoomModal.submit")}
              </Button>
              <Button
                type="button"
                variant="ghost"
                size="cta"
                onClick={() => handleOpenChange(false)}
                data-testid="cancel-button"
              >
                {t("lobby.createRoomModal.cancel")}
              </Button>
            </footer>
          </div>

          <aside
            className="border-border hidden flex-col gap-4 border-t px-9 py-7 md:flex md:min-h-0 md:overflow-y-auto md:border-t-0 md:border-l"
            style={{
              background:
                "radial-gradient(ellipse 90% 60% at 50% -10%, var(--brass-soft), transparent 70%), var(--surface-3)",
            }}
          >
            <Eyebrow>{t("lobby.createRoomModal.preview.title")}</Eyebrow>

            <CreateRoomPreview
              name={trimmed || t("lobby.createRoomModal.roomNamePlaceholder")}
              variant={variant}
              matchMode={matchMode}
              timerStyle={timerStyle}
              timerDuration={timerDuration}
              coinBuyIn={coinBuyIn}
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
        </form>
      </DialogContent>
    </Dialog>
  );
}

function AdvancedSettings({
  open,
  onToggle,
  changedCount,
  children,
}: {
  open: boolean;
  onToggle: () => void;
  changedCount: number;
  children: React.ReactNode;
}) {
  const { t } = useTranslation();
  return (
    <section
      className={cn(
        "shrink-0 overflow-hidden rounded-lg border transition-[border-color,background-color]",
        open ? "border-brass bg-surface-sunken" : "border-border bg-surface-elevated",
      )}
    >
      <button
        type="button"
        onClick={onToggle}
        aria-expanded={open}
        data-testid="advanced-settings-toggle"
        className="flex w-full items-center gap-2.5 px-3.5 py-3 text-left"
      >
        <span className="text-brass-deep flex">
          <Settings className="size-4" />
        </span>
        <span className="text-ink text-sm font-semibold">
          {t("lobby.createRoomModal.advanced.title")}
        </span>
        {changedCount > 0 && (
          <Badge tone="brass" size="sm" data-testid="advanced-settings-badge">
            {t("lobby.createRoomModal.advanced.changedCount", { count: changedCount })}
          </Badge>
        )}
        <span
          className={cn("text-ink-mute ml-auto flex transition-transform", open && "rotate-180")}
        >
          <ChevronDown className="size-4" />
        </span>
      </button>
      {!open && (
        <p className="text-ink-mute px-3.5 pb-3.5 pl-9.5 text-xs leading-snug">
          {t("lobby.createRoomModal.advanced.summary")}
        </p>
      )}
      {open && (
        <div
          data-testid="advanced-settings-panel"
          className="flex flex-col gap-3.5 px-3.5 pt-1 pb-3.5"
        >
          {children}
        </div>
      )}
    </section>
  );
}
