import { Check, X } from "lucide-react";
import { useTranslation } from "react-i18next";
import { useNavigate } from "react-router";

import { Avatar } from "@/shared/components/ui/avatar";
import { Eyebrow } from "@/shared/components/ui/eyebrow";
import {
  useAcceptFriendRequestMutation,
  useDeclineFriendRequestMutation,
} from "@/shared/hooks/mutations/useFriendMutations";
import { useFriendRequests } from "@/shared/hooks/queries/useFriendRequests";

/**
 * The pending friend-requests section inside the Friends card, with accept and decline actions.
 * It renders nothing while loading or when there are no pending requests.
 */
export function FriendRequests() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const { data: requests, isPending } = useFriendRequests();
  const acceptMutation = useAcceptFriendRequestMutation();
  const declineMutation = useDeclineFriendRequestMutation();

  const busy = acceptMutation.isPending || declineMutation.isPending;

  if (isPending || !requests || requests.length === 0) return null;

  return (
    <section
      className="border-border mb-2.5 border-b pb-2.5"
      data-testid="friend-requests"
      data-count={requests.length}
    >
      <div className="mb-1.5 flex items-center gap-1.5">
        <Eyebrow>{t("friends.requests.heading")}</Eyebrow>
        <span className="text-brass-deep font-mono text-[10.5px] font-semibold tabular-nums">
          {requests.length}
        </span>
      </div>

      <ul className="grid grid-cols-[repeat(auto-fill,minmax(15rem,1fr))] list-none gap-1.5 p-0">
        {requests.map((request) => (
          <li
            key={request.id}
            className="bg-surface-elevated border-border flex items-center gap-2 rounded-md border px-2.5 py-1.5"
            data-testid="friend-request-row"
            data-request-id={request.id}
          >
            <button
              type="button"
              onClick={() => navigate(`/players/${request.fromUserId}`)}
              aria-label={t("friends.viewProfileAria", { username: request.fromUsername })}
              data-testid="friend-request-profile"
              className="hover:bg-surface-sunken focus-visible:ring-ring/70 flex min-w-0 flex-1 cursor-pointer items-center gap-2 rounded-md text-left transition-colors focus-visible:ring-3 focus-visible:outline-none"
            >
              <Avatar
                name={request.fromUsername}
                avatarUrl={request.avatarUrl}
                size={28}
                className="shrink-0"
              />
              <span className="text-ink truncate text-sm font-medium">{request.fromUsername}</span>
            </button>
            <button
              type="button"
              data-testid="friend-request-accept"
              disabled={busy}
              onClick={() =>
                acceptMutation.mutate({ requestId: request.id, userId: request.fromUserId })
              }
              aria-label={t("friends.accept")}
              title={t("friends.accept")}
              className="text-accent hover:text-accent-deep focus-visible:ring-ring/70 flex size-6 shrink-0 cursor-pointer items-center justify-center rounded-md transition-colors focus-visible:ring-3 focus-visible:outline-none disabled:opacity-50"
            >
              <Check className="size-4" aria-hidden="true" />
            </button>
            <button
              type="button"
              data-testid="friend-request-decline"
              disabled={busy}
              onClick={() =>
                declineMutation.mutate({ requestId: request.id, userId: request.fromUserId })
              }
              aria-label={t("friends.decline")}
              title={t("friends.decline")}
              className="text-ink-mute hover:text-danger focus-visible:ring-ring/70 flex size-6 shrink-0 cursor-pointer items-center justify-center rounded-md transition-colors focus-visible:ring-3 focus-visible:outline-none disabled:opacity-50"
            >
              <X className="size-4" aria-hidden="true" />
            </button>
          </li>
        ))}
      </ul>
    </section>
  );
}
