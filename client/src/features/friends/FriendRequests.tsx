import { Check, X } from "lucide-react";
import { useTranslation } from "react-i18next";

import {
  useAcceptFriendRequestMutation,
  useDeclineFriendRequestMutation,
} from "@/shared/hooks/mutations/useFriendMutations";
import { useFriendRequests } from "@/shared/hooks/queries/useFriendRequests";

/**
 * The viewer's incoming friend-requests surface in the lobby (Story 11.2, AC4 +
 * AC8). Each row shows the sender's username with Accept / Decline actions wired
 * to the friend mutations. This list is the DURABLE delivery path — the WS
 * system:friend_request push only invalidates it (best-effort, online-only).
 */
export function FriendRequests() {
  const { t } = useTranslation();
  const { data: requests, isPending } = useFriendRequests();
  const acceptMutation = useAcceptFriendRequestMutation();
  const declineMutation = useDeclineFriendRequestMutation();

  const busy = acceptMutation.isPending || declineMutation.isPending;

  return (
    <div
      className="bg-surface border-border mb-3.5 rounded-lg border p-3.5"
      data-testid="friend-requests"
    >
      <h2 className="text-ink-dim mb-2 text-xs font-semibold">{t("friends.requests.heading")}</h2>

      {isPending ? (
        <p className="text-ink-mute px-2.5 py-2 text-xs">{t("friends.requests.loading")}</p>
      ) : !requests || requests.length === 0 ? (
        <p className="text-ink-dim px-2.5 py-2 text-xs" data-testid="friend-requests-empty">
          {t("friends.requests.empty")}
        </p>
      ) : (
        <ul className="flex flex-col gap-1">
          {requests.map((request) => (
            <li
              key={request.id}
              className="flex items-center gap-2 rounded-lg px-2.5 py-2"
              data-testid="friend-request-row"
              data-request-id={request.id}
            >
              <span className="bg-accent-soft text-accent flex size-7 shrink-0 items-center justify-center rounded-full text-xs font-semibold">
                {request.fromUsername.charAt(0).toUpperCase()}
              </span>
              <span className="text-ink min-w-0 flex-1 truncate text-sm font-medium">
                {request.fromUsername}
              </span>
              <button
                type="button"
                data-testid="friend-request-accept"
                disabled={busy}
                onClick={() =>
                  acceptMutation.mutate({ requestId: request.id, userId: request.fromUserId })
                }
                aria-label={t("friends.accept")}
                className="text-accent hover:bg-surface-sunken flex items-center gap-1 rounded-md px-2 py-1 text-xs font-medium disabled:opacity-50"
              >
                <Check className="size-3.5" />
                {t("friends.accept")}
              </button>
              <button
                type="button"
                data-testid="friend-request-decline"
                disabled={busy}
                onClick={() =>
                  declineMutation.mutate({ requestId: request.id, userId: request.fromUserId })
                }
                aria-label={t("friends.decline")}
                className="text-ink-mute hover:bg-surface-sunken flex items-center gap-1 rounded-md px-2 py-1 text-xs font-medium disabled:opacity-50"
              >
                <X className="size-3.5" />
                {t("friends.decline")}
              </button>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}
