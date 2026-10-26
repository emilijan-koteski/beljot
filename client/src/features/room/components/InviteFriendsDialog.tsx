import type { TFunction } from "i18next";
import { Check, Search, Send } from "lucide-react";
import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";

import { FetchError } from "@/shared/api/axiosClient";
import { Avatar } from "@/shared/components/ui/avatar";
import { Button } from "@/shared/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/shared/components/ui/dialog";
import { Tabs, TabsList, TabsTrigger } from "@/shared/components/ui/tabs";
import { useInviteToRoomMutation } from "@/shared/hooks/mutations/useRooms";
import { useInvitableFriends } from "@/shared/hooks/queries/useInvitableFriends";
import { useInvitablePlayers } from "@/shared/hooks/queries/useInvitablePlayers";
import { useDebounce } from "@/shared/hooks/useDebounce";
import { inviteFailure } from "@/shared/lib/inviteFailure";
import type { InvitableFriend } from "@/shared/types/apiTypes";

type InviteTab = "friends" | "lobby";

const TABS = [
  { value: "friends", labelKey: "roomInvite.panel.tabFriends" },
  { value: "lobby", labelKey: "roomInvite.panel.tabLobby" },
] as const satisfies ReadonlyArray<{ value: InviteTab; labelKey: string }>;

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

/**
 * Inviter-side panel with a Friends tab and a searchable Lobby tab of idle non-friends, both annotated server-side.
 * Per-row state and the search reset on each open because RoomPage keeps this dialog mounted.
 */
export function InviteFriendsDialog({ open, roomId, onOpenChange }: InviteFriendsDialogProps) {
  const { t } = useTranslation();
  const [tab, setTab] = useState<InviteTab>("friends");
  const [search, setSearch] = useState("");
  const debouncedSearch = useDebounce(search);
  const lobbySearch = search.trim() === "" ? "" : debouncedSearch.trim();
  const friendsQuery = useInvitableFriends(roomId, open && tab === "friends");
  const lobbyQuery = useInvitablePlayers(roomId, lobbySearch, open && tab === "lobby");
  const inviteMutation = useInviteToRoomMutation();

  const [invited, setInvited] = useState<Record<number, boolean>>({});
  const [pendingId, setPendingId] = useState<number | null>(null);
  const [rowErrors, setRowErrors] = useState<Record<number, string>>({});

  useEffect(() => {
    if (open) {
      setInvited({});
      setRowErrors({});
    }
    setSearch("");
  }, [open]);

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
        onOpenChange(false);
      } else {
        setRowErrors((prev) => ({ ...prev, [row.userId]: message }));
      }
    } finally {
      setPendingId(null);
    }
  }

  const rowState = {
    invited,
    rowErrors,
    pendingId,
    onInvite: (row: InvitableFriend) => void invite(row),
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent data-testid="invite-friends-dialog">
        <DialogHeader>
          <DialogTitle>{t("roomInvite.panel.title")}</DialogTitle>
          <DialogDescription>{t("roomInvite.panel.description")}</DialogDescription>
        </DialogHeader>

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

        {tab === "friends" ? (
          <InviteList
            rows={friendsQuery.data}
            isPending={friendsQuery.isPending}
            isError={friendsQuery.isError}
            loadingText={t("roomInvite.panel.loading")}
            errorText={t("roomInvite.panel.loadFailed")}
            emptyText={t("roomInvite.panel.empty")}
            stateTestId="invite-friends"
            rowTestId="invite-friend"
            {...rowState}
          />
        ) : (
          <div className="flex flex-col gap-2">
            <div className="bg-surface-elevated border-border flex items-center gap-2 rounded-[10px] border px-2.5 py-1.5 transition-[border-color,box-shadow] has-[input:focus-visible]:border-ring has-[input:focus-visible]:ring-3 has-[input:focus-visible]:ring-ring/70">
              <Search className="text-ink-mute size-3.5 shrink-0" aria-hidden="true" />
              <input
                value={search}
                onChange={(e) => setSearch(e.target.value)}
                placeholder={t("roomInvite.panel.searchPlaceholder")}
                aria-label={t("roomInvite.panel.searchLabel")}
                data-testid="invite-lobby-search"
                className="text-ink placeholder:text-ink-off min-w-0 flex-1 truncate bg-transparent text-sm outline-none"
              />
            </div>
            <InviteList
              rows={lobbyQuery.data}
              isPending={lobbyQuery.isPending}
              isError={lobbyQuery.isError}
              loadingText={t("roomInvite.panel.lobbyLoading")}
              errorText={t("roomInvite.panel.lobbyLoadFailed")}
              emptyText={
                lobbySearch === ""
                  ? t("roomInvite.panel.lobbyEmpty")
                  : t("roomInvite.panel.lobbyNoMatch", { query: lobbySearch })
              }
              stateTestId="invite-lobby"
              rowTestId="invite-player"
              {...rowState}
            />
          </div>
        )}
      </DialogContent>
    </Dialog>
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
  invited,
  rowErrors,
  pendingId,
  onInvite,
}: InviteListProps) {
  const { t } = useTranslation();

  if (isPending) {
    return (
      <p className="text-ink-mute px-1 py-2 text-xs" data-testid={`${stateTestId}-loading`}>
        {loadingText}
      </p>
    );
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
      <p className="text-ink-dim px-1 py-2 text-xs" data-testid={`${stateTestId}-empty`}>
        {emptyText}
      </p>
    );
  }

  return (
    <ul className="flex max-h-80 flex-col gap-1 overflow-y-auto">
      {rows.map((row) => {
        const isInvited = invited[row.userId] === true;
        const rowError = rowErrors[row.userId];
        const disabled = row.available !== true || isInvited || pendingId !== null;
        return (
          <li
            key={row.userId}
            className="flex items-center gap-2 rounded-lg px-1.5 py-2"
            data-testid={`${rowTestId}-row`}
            data-user-id={row.userId}
            data-available={row.available === true ? "true" : "false"}
          >
            <Avatar name={row.username} avatarUrl={row.avatarUrl} size={28} className="shrink-0" />
            <span className="flex min-w-0 flex-1 flex-col">
              <span className="text-ink truncate font-medium">{row.username}</span>
              {(rowError !== undefined || row.available !== true) && (
                <span
                  className={
                    rowError !== undefined ? "text-danger text-[11px]" : "text-ink-mute text-[11px]"
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
                  <Check className="size-3" />
                  {t("roomInvite.panel.invited")}
                </>
              ) : (
                <>
                  <Send className="size-3" />
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
