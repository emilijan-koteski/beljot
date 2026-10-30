import type { TFunction } from "i18next";
import { Check, Send, UserPlus } from "lucide-react";
import { useState } from "react";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";

import { FetchError } from "@/shared/api/axiosClient";
import { EmptyState } from "@/shared/components/EmptyState";
import { LoadingLine } from "@/shared/components/LoadingLine";
import { AppDialog } from "@/shared/components/ui/app-dialog";
import { Avatar } from "@/shared/components/ui/avatar";
import { Button } from "@/shared/components/ui/button";
import { SearchInput } from "@/shared/components/ui/search-input";
import { Tabs, TabsList, TabsTrigger } from "@/shared/components/ui/tabs";
import { useInviteToRoomMutation } from "@/shared/hooks/mutations/useRooms";
import { useInvitableFriends } from "@/shared/hooks/queries/useInvitableFriends";
import { useInvitablePlayers } from "@/shared/hooks/queries/useInvitablePlayers";
import { useDebounce } from "@/shared/hooks/useDebounce";
import { inviteFailure } from "@/shared/lib/inviteFailure";
import { cn } from "@/shared/lib/utils";
import type { InvitableFriend } from "@/shared/types/apiTypes";

type InviteTab = "lobby" | "friends";

const TABS = [
  { value: "lobby", labelKey: "roomInvite.panel.tabLobby" },
  { value: "friends", labelKey: "roomInvite.panel.tabFriends" },
] as const satisfies ReadonlyArray<{ value: InviteTab; labelKey: string }>;

const SCROLL_LIST = "overflow-y-auto overscroll-contain pr-1 [scrollbar-width:thin]";

function isInviteTab(value: unknown): value is InviteTab {
  return value === "friends" || value === "lobby";
}

function reasonLabel(t: TFunction, row: InvitableFriend): string {
  switch (row.reason) {
    case "offline":
      return t("roomInvite.reasons.offline");
    case "in_match":
      return t("roomInvite.reasons.inMatch");
    case "in_room":
      return t("roomInvite.reasons.inRoom");
    case "in_this_room":
      return t("roomInvite.reasons.alreadyHere");
    case "room_full":
      return t("roomInvite.reasons.roomFull");
    default:
      return t("roomInvite.reasons.unavailable");
  }
}

interface InviteFriendsDialogProps {
  open: boolean;
  roomId: number;
  onOpenChange: (open: boolean) => void;
}

/** The invite list in the app's dialog, opened from an open seat or the room roster. */
export function InviteFriendsDialog({ open, roomId, onOpenChange }: InviteFriendsDialogProps) {
  const { t } = useTranslation();
  return (
    <AppDialog
      open={open}
      onClose={() => onOpenChange(false)}
      icon={<UserPlus />}
      title={t("roomInvite.panel.title")}
      primary={{
        label: t("common.done"),
        onClick: () => onOpenChange(false),
        testId: "invite-friends-done",
      }}
      testId="invite-friends-dialog"
    >
      <InvitePlayers
        roomId={roomId}
        active={open}
        listClassName="max-h-80"
        onRoomGone={() => onOpenChange(false)}
      />
    </AppDialog>
  );
}

interface InvitePlayersProps {
  roomId: number;
  active: boolean;
  listClassName?: string;
  onRoomGone?: () => void;
}

/**
 * Who to invite: lobby players first, then friends, a search over the open tab and one row per player.
 * Unavailable players stay listed with the reason; the rows scroll inside `listClassName`'s height.
 */
export function InvitePlayers({ roomId, active, listClassName, onRoomGone }: InvitePlayersProps) {
  const { t } = useTranslation();
  const [tab, setTab] = useState<InviteTab>("lobby");
  const [search, setSearch] = useState("");
  const debouncedSearch = useDebounce(search);
  const query = search.trim() === "" ? "" : debouncedSearch.trim();
  const friendsQuery = useInvitableFriends(roomId, active && tab === "friends");
  const lobbyQuery = useInvitablePlayers(roomId, query, active && tab === "lobby");
  const inviteMutation = useInviteToRoomMutation();

  const [invited, setInvited] = useState<Record<number, boolean>>({});
  const [pendingId, setPendingId] = useState<number | null>(null);
  const [rowErrors, setRowErrors] = useState<Record<number, string>>({});

  async function invite(row: InvitableFriend) {
    setPendingId(row.userId);
    setRowErrors((prev) => {
      const next = { ...prev };
      delete next[row.userId];
      return next;
    });
    try {
      await inviteMutation.mutateAsync({ roomId, friendUserId: row.userId });
      setInvited((prev) => ({ ...prev, [row.userId]: true }));
    } catch (err) {
      const { kind, message } = inviteFailure(err instanceof FetchError ? err.code : null);
      if (kind === "alreadyPending") {
        setInvited((prev) => ({ ...prev, [row.userId]: true }));
      } else if (kind === "roomGone") {
        toast.error(message);
        onRoomGone?.();
      } else {
        setRowErrors((prev) => ({ ...prev, [row.userId]: message }));
      }
    } finally {
      setPendingId(null);
    }
  }

  const friends = friendsQuery.data?.filter((row) =>
    row.username.toLowerCase().includes(query.toLowerCase()),
  );
  const lobby = tab === "lobby";
  const rows = lobby ? lobbyQuery.data : friends;
  const state = lobby ? lobbyQuery : friendsQuery;

  return (
    <div className="flex flex-col gap-2.5">
      <Tabs
        value={tab}
        onValueChange={(value) => {
          if (isInviteTab(value)) setTab(value);
        }}
      >
        <TabsList aria-label={t("roomInvite.panel.tabsLabel")} className="w-full">
          {TABS.map(({ value, labelKey }) => (
            <TabsTrigger key={value} value={value} data-testid={`invite-tab-${value}`}>
              {t(labelKey)}
            </TabsTrigger>
          ))}
        </TabsList>
      </Tabs>
      <SearchInput
        value={search}
        onValueChange={setSearch}
        placeholder={t("roomInvite.panel.searchPlaceholder")}
        aria-label={t("roomInvite.panel.searchPlaceholder")}
        data-testid="invite-search"
      />
      <InviteList
        rows={rows}
        isPending={state.isPending}
        isError={state.isError}
        loadingText={t("roomInvite.panel.loading")}
        errorText={t("roomInvite.panel.loadFailed")}
        emptyText={
          query !== ""
            ? t("roomInvite.panel.noMatch", { query })
            : lobby
              ? t("roomInvite.panel.lobbyEmpty")
              : t("roomInvite.panel.empty")
        }
        stateTestId={lobby ? "invite-lobby" : "invite-friends"}
        rowTestId={lobby ? "invite-player" : "invite-friend"}
        listClassName={listClassName}
        invited={invited}
        rowErrors={rowErrors}
        pendingId={pendingId}
        onInvite={(row) => void invite(row)}
      />
    </div>
  );
}

interface InviteListProps {
  rows: InvitableFriend[] | undefined;
  isPending: boolean;
  isError: boolean;
  loadingText: string;
  errorText: string;
  emptyText: string;
  stateTestId: string;
  rowTestId: string;
  listClassName?: string;
  invited: Record<number, boolean>;
  rowErrors: Record<number, string>;
  pendingId: number | null;
  onInvite: (row: InvitableFriend) => void;
}

/** One tab's list with its loading, error and empty states; unavailable rows stay listed but disabled with a reason. */
function InviteList({
  rows,
  isPending,
  isError,
  loadingText,
  errorText,
  emptyText,
  stateTestId,
  rowTestId,
  listClassName,
  invited,
  rowErrors,
  pendingId,
  onInvite,
}: InviteListProps) {
  const { t } = useTranslation();

  if (isPending) {
    return <LoadingLine testId={`${stateTestId}-loading`}>{loadingText}</LoadingLine>;
  }
  if (isError) {
    return (
      <p className="text-danger px-1 py-2 text-xs" data-testid={`${stateTestId}-error`}>
        {errorText}
      </p>
    );
  }
  if (!rows || rows.length === 0) {
    return (
      <EmptyState compact testId={`${stateTestId}-empty`}>
        {emptyText}
      </EmptyState>
    );
  }

  return (
    <ul
      className={cn(
        "divide-border m-0 flex list-none flex-col divide-y p-0",
        SCROLL_LIST,
        listClassName,
      )}
    >
      {rows.map((row) => {
        const isInvited = invited[row.userId] === true;
        const rowError = rowErrors[row.userId];
        const disabled = row.available !== true || isInvited || pendingId !== null;
        return (
          <li
            key={row.userId}
            className="flex items-center gap-2.5 py-2"
            data-testid={`${rowTestId}-row`}
            data-user-id={row.userId}
            data-available={row.available === true ? "true" : "false"}
          >
            <Avatar name={row.username} avatarUrl={row.avatarUrl} size={32} className="shrink-0" />
            <span className="flex min-w-0 flex-1 flex-col leading-tight">
              <span className="text-ink truncate text-sm font-semibold">{row.username}</span>
              {(rowError !== undefined || row.available !== true) && (
                <span
                  className={
                    rowError !== undefined ? "text-danger text-2xs" : "text-ink-mute text-2xs"
                  }
                  data-testid={`${rowTestId}-reason-${row.userId}`}
                >
                  {rowError ?? reasonLabel(t, row)}
                </span>
              )}
            </span>
            <Button
              type="button"
              size="sm"
              variant={isInvited ? "ghost" : "outline"}
              disabled={disabled}
              onClick={() => onInvite(row)}
              data-testid={`${rowTestId}-${row.userId}`}
            >
              {isInvited ? (
                <>
                  <Check className="size-3.5" />
                  {t("roomInvite.panel.invited")}
                </>
              ) : (
                <>
                  <Send className="size-3.5" />
                  {t("roomInvite.panel.invite")}
                </>
              )}
            </Button>
          </li>
        );
      })}
    </ul>
  );
}
