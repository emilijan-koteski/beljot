import {
  ArrowRight,
  Clock,
  Coins,
  Eye,
  EyeOff,
  KeyRound,
  Lock,
  LockOpen,
  Users,
  Zap,
} from "lucide-react";
import { useState } from "react";
import { Trans, useTranslation } from "react-i18next";
import { useNavigate } from "react-router";

import { SeatChip } from "@/features/lobby/components/SeatChip";
import { FetchError } from "@/shared/api/axiosClient";
import { Button } from "@/shared/components/ui/button";
import { Dialog, DialogContent } from "@/shared/components/ui/dialog";
import { DurationSlider } from "@/shared/components/ui/duration-slider";
import { Eyebrow } from "@/shared/components/ui/eyebrow";
import { Field } from "@/shared/components/ui/field";
import { Input } from "@/shared/components/ui/input";
import { Segmented } from "@/shared/components/ui/segmented";
import { useCreateRoomMutation } from "@/shared/hooks/mutations/useRooms";
import { COIN_GOLD } from "@/shared/lib/coinGold";
import { formatCoins } from "@/shared/lib/formatCoins";
import { cn } from "@/shared/lib/utils";
import { useAuthStore } from "@/shared/stores/authStore";

interface CreateRoomModalProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
}

const MIN_NAME = 3;
const MAX_NAME = 32;
const DEFAULT_BUY_IN = 500; // mirrors the server default (Story 9.2)
// Private-room password bounds (Story 9.6). Cosmetic client guards; the server
// is authoritative (apperr ROOM_PASSWORD_TOO_SHORT / ROOM_PASSWORD_TOO_LONG).
const MIN_ROOM_PASSWORD = 4;
const MAX_ROOM_PASSWORD = 72;

/**
 * Split-panel create-room modal. Left = form (name, variant, match mode,
 * timer, conditional move-duration slider). Right = darker live-preview pane
 * that mirrors the lobby card the room will become so the host sees exactly
 * what the lobby grid will display.
 *
 * Backend contract unchanged — only `name | variant | matchMode | timerStyle
 * | timerDurationSeconds` are submitted. The disabled segmented option
 * (Croatian variant) sits in the UI to telegraph the planned scope;
 * its "Only X for now" Field hint communicates that locked state.
 */
export function CreateRoomModal({ open, onOpenChange }: CreateRoomModalProps) {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const createRoomMutation = useCreateRoomMutation();
  const meUsername = useAuthStore((s) => s.user?.username ?? "");
  const meBalance = useAuthStore((s) => s.user?.walletBalance ?? 0);

  const [name, setName] = useState("");
  const [variant, setVariant] = useState<"bitola" | "croatia">("bitola");
  const [matchMode, setMatchMode] = useState<"1001" | "501">("1001");
  const [timerStyle, setTimerStyle] = useState<"relaxed" | "per-move">("relaxed");
  const [timerDuration, setTimerDuration] = useState(30);
  const [coinBuyIn, setCoinBuyIn] = useState(DEFAULT_BUY_IN);
  const [isPrivate, setIsPrivate] = useState(false);
  const [roomPassword, setRoomPassword] = useState("");
  const [showPassword, setShowPassword] = useState(false);
  // Field-level error shown under the room-name input (name validation only).
  const [error, setError] = useState<string | null>(null);
  // General submit error (already-in-room, insolvency race, unexpected) shown
  // in a pinned banner above the footer — NOT under the name field.
  const [formError, setFormError] = useState<string | null>(null);

  const trimmed = name.trim();
  const nameValid = trimmed.length >= MIN_NAME && trimmed.length <= MAX_NAME;
  // A private room needs a password within bounds; a public room ignores it.
  // MIN is per-character (matches the server's rune count); MAX is the bcrypt
  // byte limit, so measure UTF-8 bytes — not UTF-16 .length — to agree with the
  // server and never accept a multibyte password it would reject as too long.
  const passwordValid =
    !isPrivate ||
    (roomPassword.length >= MIN_ROOM_PASSWORD &&
      new TextEncoder().encode(roomPassword).length <= MAX_ROOM_PASSWORD);
  const submitting = createRoomMutation.isPending;

  // The creator is auto-seated and charged the buy-in at match start, so a room
  // they can't afford is un-startable — block it here too (server re-validates).
  const effectiveBuyIn = Math.max(0, Math.floor(coinBuyIn || 0));
  const buyInExceedsBalance = effectiveBuyIn > meBalance;

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
        // Only send the password for a private room (Story 9.6).
        password: isPrivate ? roomPassword : undefined,
      });
      onOpenChange(false);
      navigate(`/rooms/${room.id}`);
    } catch (err) {
      if (err instanceof FetchError) {
        // Name-specific failures belong under the name field; everything else
        // is a general failure shown in the form banner (never the raw server
        // message — e.g. "player is already in a room" under the name input).
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
      setVariant("bitola");
      setMatchMode("1001");
      setTimerStyle("relaxed");
      setTimerDuration(30);
      setCoinBuyIn(DEFAULT_BUY_IN);
      setIsPrivate(false);
      setRoomPassword("");
      setShowPassword(false);
      setError(null);
      setFormError(null);
      createRoomMutation.reset();
    }
    onOpenChange(nextOpen);
  }

  const variantOptions = [
    { value: "bitola" as const, label: t("lobby.createRoomModal.variantBitola") },
    {
      value: "croatia" as const,
      label: t("lobby.createRoomModal.variantCroatia"),
      disabled: true,
    },
  ];

  const matchModeOptions = [
    { value: "501" as const, label: t("lobby.createRoomModal.matchMode501") },
    { value: "1001" as const, label: t("lobby.createRoomModal.matchMode1001") },
  ];

  const timerOptions = [
    {
      value: "relaxed" as const,
      label: t("lobby.createRoomModal.timerRelaxed"),
      icon: <Clock className="size-3.5" />,
    },
    {
      value: "per-move" as const,
      label: t("lobby.createRoomModal.timerPerMove"),
      icon: <Zap className="size-3.5" />,
    },
  ];

  return (
    <Dialog open={open} onOpenChange={handleOpenChange}>
      <DialogContent
        className="bg-surface border-border max-h-[calc(100dvh-2rem)] w-[calc(100%-2rem)] gap-0 overflow-y-auto rounded-[22px] border p-0 ring-0 shadow-[0_30px_80px_-40px_rgba(14,58,36,0.30),0_0_0_1px_rgba(201,168,118,0.20)] sm:max-w-[min(1100px,calc(100vw-4rem))] md:flex md:max-h-[min(720px,calc(100vh-4rem))] md:flex-col md:overflow-hidden"
        showCloseButton={false}
        data-testid="create-room-modal"
      >
        {/* Top brass hairline accent — same gradient as AuthCard */}
        <div
          aria-hidden="true"
          className="pointer-events-none absolute -top-px right-7 left-7 h-0.5 bg-[linear-gradient(90deg,transparent,var(--brass)_50%,transparent)] opacity-70"
        />

        <form
          onSubmit={handleSubmit}
          className="grid grid-cols-1 md:min-h-0 md:flex-1 md:grid-cols-[1.05fr_0.95fr] md:grid-rows-[minmax(0,1fr)]"
          noValidate
        >
          {/* ── Left: form ────────────────────────────────────────────── */}
          <div className="bg-surface flex flex-col md:min-h-0">
            <header className="flex shrink-0 flex-col gap-2 px-8 pt-7 pb-1.5">
              <Eyebrow tone="accent">
                <span className="bg-accent inline-block size-1.5 rounded-full" />
                {t("lobby.createRoomModal.eyebrow")}
              </Eyebrow>
              <h2 className="font-display text-ink m-0 text-[26px] leading-tight font-bold tracking-[-0.6px]">
                {t("lobby.createRoomModal.headline")}
              </h2>
              <p className="text-ink-dim m-0 text-[13.5px] leading-[1.55]">
                {t("lobby.createRoomModal.subhead")}
              </p>
            </header>

            <div className="flex flex-col gap-4 px-8 py-4 md:min-h-0 md:flex-1 md:overflow-y-auto">
              <Field
                label={t("lobby.createRoomModal.roomName")}
                htmlFor="room-name"
                hint={t("lobby.createRoomModal.roomNameHint", { min: MIN_NAME, max: MAX_NAME })}
                error={error ?? undefined}
                errorTestId="room-name-error"
                required
              >
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
                  className="h-11"
                />
              </Field>

              <Field
                label={t("lobby.createRoomModal.variant")}
                hint={t("lobby.createRoomModal.variantHint")}
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
                hint={t("lobby.createRoomModal.matchModeHint")}
              >
                <Segmented
                  value={matchMode}
                  onValueChange={setMatchMode}
                  options={matchModeOptions}
                  testId="match-mode-segmented"
                  ariaLabel={t("lobby.createRoomModal.matchMode")}
                />
              </Field>

              <Field
                label={t("lobby.createRoomModal.timerStyle")}
                hint={
                  timerStyle === "relaxed" ? t("lobby.createRoomModal.timerHintRelaxed") : undefined
                }
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
                  hint={t("lobby.createRoomModal.timerDurationHint")}
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
                htmlFor="coin-buy-in"
                hint={t("lobby.createRoomModal.coinBuyInHint")}
                error={
                  buyInExceedsBalance
                    ? t("lobby.createRoomModal.errors.buyInTooHigh", {
                        balance: formatCoins(meBalance),
                      })
                    : undefined
                }
                errorTestId="buy-in-error"
              >
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
                  className="h-11"
                />
              </Field>

              <Field
                label={t("lobby.createRoomModal.privateRoom")}
                hint={t("lobby.createRoomModal.privateRoomHint")}
              >
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
                      icon: <LockOpen className="size-3.5" />,
                    },
                    {
                      value: "private",
                      label: t("lobby.createRoomModal.privacyPrivate"),
                      icon: <Lock className="size-3.5" />,
                    },
                  ]}
                  testId="private-room-toggle"
                  ariaLabel={t("lobby.createRoomModal.privateRoom")}
                />
              </Field>

              {isPrivate && (
                <Field
                  label={t("lobby.createRoomModal.roomPassword")}
                  htmlFor="room-password"
                  hint={t("lobby.createRoomModal.roomPasswordHint", { min: MIN_ROOM_PASSWORD })}
                  required
                >
                  <div className="relative">
                    <Input
                      id="room-password"
                      type={showPassword ? "text" : "password"}
                      autoComplete="new-password"
                      placeholder={t("lobby.createRoomModal.roomPasswordPlaceholder")}
                      value={roomPassword}
                      onChange={(e) => setRoomPassword(e.target.value.slice(0, MAX_ROOM_PASSWORD))}
                      maxLength={MAX_ROOM_PASSWORD}
                      data-testid="room-password-input"
                      className="h-11 pr-10"
                    />
                    <button
                      type="button"
                      tabIndex={-1}
                      className="text-ink-mute hover:text-ink absolute top-1/2 right-2.5 -translate-y-1/2 p-1.5"
                      onClick={() => setShowPassword(!showPassword)}
                      data-testid="room-password-toggle"
                      aria-label={showPassword ? "Hide password" : "Show password"}
                    >
                      {showPassword ? <EyeOff className="size-4" /> : <Eye className="size-4" />}
                    </button>
                  </div>
                </Field>
              )}
            </div>

            {/* General submit error — pinned above the footer so it's always
                visible (the form body scrolls independently on desktop). */}
            {formError && (
              <div
                role="alert"
                data-testid="create-room-form-error"
                className="text-destructive border-border shrink-0 border-t px-8 py-2.5 text-sm font-medium"
              >
                {formError}
              </div>
            )}

            <footer className="border-border bg-surface flex shrink-0 items-center justify-between gap-2 border-t px-8 py-3.5">
              <Button
                type="button"
                variant="ghost"
                onClick={() => handleOpenChange(false)}
                data-testid="cancel-button"
              >
                {t("lobby.createRoomModal.cancel")}
              </Button>
              <Button
                type="submit"
                size="cta"
                disabled={!nameValid || submitting || buyInExceedsBalance || !passwordValid}
                data-testid="create-room-button"
              >
                {submitting
                  ? t("lobby.createRoomModal.submitting")
                  : t("lobby.createRoomModal.submit")}
              </Button>
            </footer>
          </div>

          {/* ── Right: live preview pane ──────────────────────────────── */}
          <aside
            className="border-border hidden flex-col gap-4 border-t px-9 py-7 md:flex md:min-h-0 md:overflow-y-auto md:border-t-0 md:border-l"
            style={{
              background:
                "radial-gradient(ellipse 90% 60% at 50% -10%, rgba(201,168,118,0.18), transparent 70%), var(--surface-3)",
            }}
          >
            <Eyebrow>{t("lobby.createRoomModal.preview.title")}</Eyebrow>

            <PreviewCard
              name={trimmed || t("lobby.createRoomModal.roomNamePlaceholder")}
              variant={variant}
              matchMode={matchMode}
              timerStyle={timerStyle}
              timerDuration={timerDuration}
              coinBuyIn={coinBuyIn}
              isPrivate={isPrivate}
              hostUsername={meUsername || "host"}
            />

            <div className="bg-surface border-border rounded-[12px] border border-dashed p-4">
              <Eyebrow>{t("lobby.createRoomModal.nextSteps.title")}</Eyebrow>
              <ol className="text-ink-dim mt-2 list-decimal pl-4.5 text-[12.5px] leading-[1.65]">
                <li>
                  <Trans
                    i18nKey="lobby.createRoomModal.nextSteps.step1"
                    components={{ strong: <strong className="text-ink font-semibold" /> }}
                  />
                </li>
                <li>
                  <Trans
                    i18nKey="lobby.createRoomModal.nextSteps.step2"
                    components={{
                      code: (
                        <code className="bg-brass-soft text-brass-deep rounded px-1.5 py-px font-mono text-[11.5px] font-semibold tracking-[1.2px]" />
                      ),
                    }}
                  />
                </li>
                <li>{t("lobby.createRoomModal.nextSteps.step3")}</li>
              </ol>
            </div>
          </aside>
        </form>
      </DialogContent>
    </Dialog>
  );
}

function PreviewCard({
  name,
  variant,
  matchMode,
  timerStyle,
  timerDuration,
  coinBuyIn,
  isPrivate,
  hostUsername,
}: {
  name: string;
  variant: "bitola" | "croatia";
  matchMode: "1001" | "501";
  timerStyle: "relaxed" | "per-move";
  timerDuration: number;
  coinBuyIn: number;
  isPrivate: boolean;
  hostUsername: string;
}) {
  const { t } = useTranslation();
  const variantLabel =
    variant === "bitola"
      ? t("lobby.card.variantBitola")
      : t("lobby.createRoomModal.variantCroatia");
  const matchLabel =
    matchMode === "1001" ? t("lobby.card.matchMode1001") : t("lobby.card.matchMode501");
  const timerLabel =
    timerStyle === "relaxed"
      ? t("lobby.card.relaxed")
      : t("lobby.card.timerSeconds", { seconds: timerDuration });
  const hostInitial = (hostUsername || "?").charAt(0).toUpperCase();

  return (
    <article
      data-testid="create-room-preview"
      className="bg-surface text-ink border-border overflow-hidden rounded-[22px] border shadow-[0_18px_40px_-22px_rgba(14,58,36,0.30)]"
    >
      <div className="px-4.5 pt-4.5 pb-3.5">
        <div className="flex items-center gap-2.5">
          <span
            aria-hidden
            className="bg-accent size-2 rounded-full"
            style={{ boxShadow: "0 0 0 3px var(--accent-soft)" }}
          />
          <h3
            className="font-display text-ink m-0 min-w-0 flex-1 truncate text-[17px] font-semibold tracking-[-0.2px]"
            data-testid="preview-name"
          >
            {name}
          </h3>
          <span className="bg-surface-sunken text-ink-dim border-border inline-flex items-center gap-1 rounded-md border px-2 py-0.5 font-mono text-[11px] font-semibold tracking-[1.5px] tabular-nums">
            <KeyRound className="size-2.5" />
            {t("lobby.createRoomModal.preview.codeLabel")}
          </span>
        </div>

        <div className="text-ink-dim mt-1.5 flex flex-wrap items-center gap-2 text-xs">
          <span>
            {variantLabel} · {matchLabel}
          </span>
          <Dot />
          <span className="inline-flex items-center gap-1">
            <Clock className="size-3" />
            {timerLabel}
          </span>
          <Dot />
          <span className="inline-flex items-center gap-1" data-testid="preview-buy-in">
            <Coins className="size-3" style={{ color: COIN_GOLD }} />
            {/* Icon-only unit — the coin glyph conveys "coins", so the preview
                shows just the grouped number (or "Free" at 0). */}
            {coinBuyIn > 0 ? formatCoins(coinBuyIn) : t("lobby.card.buyInFree")}
          </span>
          <Dot />
          {isPrivate ? (
            <span
              className="inline-flex items-center gap-1"
              data-testid="preview-lock"
              aria-label={t("lobby.card.privateLockAriaLabel")}
            >
              <Lock className="size-3" />
              {t("lobby.card.private")}
            </span>
          ) : (
            <span
              className="inline-flex items-center gap-1"
              data-testid="preview-public"
              aria-label={t("lobby.card.publicAriaLabel")}
            >
              <LockOpen className="size-3" />
              {t("lobby.card.public")}
            </span>
          )}
        </div>
      </div>

      {/* Seat preview — host (you) occupies seat 1 of Team A, others empty */}
      <div className="grid grid-cols-[auto_1fr_1fr] items-center gap-1.5 px-4.5 pb-3.5">
        <TeamLabel team="A" />
        <SeatChip username={hostUsername} team="A" testId="preview-seat-0" />
        <SeatChip username={null} team="A" testId="preview-seat-2" />
        <TeamLabel team="B" />
        <SeatChip username={null} team="B" testId="preview-seat-1" />
        <SeatChip username={null} team="B" testId="preview-seat-3" />
      </div>

      <div className="border-border bg-[rgba(14,58,36,0.03)] flex items-center gap-2.5 border-t px-4.5 py-3">
        <span className="text-ink-dim inline-flex items-center gap-1.5 text-xs">
          <span className="bg-surface-sunken text-ink border-border inline-flex size-4.5 items-center justify-center rounded-full border text-[10px] font-bold">
            {hostInitial}
          </span>
          {hostUsername}
        </span>
        <span className="text-ink-mute inline-flex items-center gap-1 text-xs">
          <Users className="size-3" />
          {t("lobby.createRoomModal.preview.seated", { current: 1 })}
        </span>
        <span className="ml-auto">
          <span className="bg-accent text-accent-ink inline-flex items-center gap-1.5 rounded-[10px] border border-accent-deep px-3.5 py-1.5 text-[11px] font-semibold opacity-70">
            {t("lobby.createRoomModal.preview.joinLabel")}
            <ArrowRight className="size-3.5" strokeWidth={2.2} />
          </span>
        </span>
      </div>
    </article>
  );
}

function TeamLabel({ team }: { team: "A" | "B" }) {
  return (
    <span
      className={cn(
        "pr-1 text-[10px] font-bold uppercase tracking-[1.2px]",
        team === "A" ? "text-team-a" : "text-team-b",
      )}
    >
      {team}
    </span>
  );
}

function Dot() {
  return <span className="text-ink-off text-[10px]">·</span>;
}
