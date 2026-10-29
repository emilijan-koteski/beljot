import { Check, X } from "lucide-react";
import { useTranslation } from "react-i18next";
import { useNavigate } from "react-router";

import { Avatar } from "@/shared/components/ui/avatar";
import { Button } from "@/shared/components/ui/button";
import { Eyebrow } from "@/shared/components/ui/eyebrow";
import {
  useAcceptFriendRequestMutation,
  useDeclineFriendRequestMutation,
} from "@/shared/hooks/mutations/useFriendMutations";
import { useFriendRequests } from "@/shared/hooks/queries/useFriendRequests";

/**
 * Pending friend requests, each on a brass card with accept and decline.
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
    <section data-testid="friend-requests" data-count={requests.length}>
      <Eyebrow tone="mute" className="mb-1.5">
        {t("friends.requests.groupLabel", { count: requests.length })}
      </Eyebrow>
      <ul className="m-0 flex list-none flex-col gap-1.5 p-0">
        {requests.map((request) => (
          <li
            key={request.id}
            className="bg-brass-soft border-brass/30 flex items-center gap-2.5 rounded-lg border px-2.5 py-2"
            data-testid="friend-request-row"
            data-request-id={request.id}
          >
            <button
              type="button"
              onClick={() => navigate(`/players/${request.fromUserId}`)}
              aria-label={t("friends.viewProfileAria", { username: request.fromUsername })}
              data-testid="friend-request-profile"
              className="focus-visible:ring-ring/70 flex min-w-0 flex-1 cursor-pointer items-center gap-2.5 rounded-md text-left outline-none focus-visible:ring-3"
            >
              <Avatar name={request.fromUsername} avatarUrl={request.avatarUrl} size={32} />
              <span className="flex min-w-0 flex-col leading-tight">
                <span className="text-ink truncate text-sm font-semibold">
                  {request.fromUsername}
                </span>
                <span className="text-ink-mute text-2xs">{t("friends.requests.subtitle")}</span>
              </span>
            </button>
            <Button
              variant="ghost"
              size="icon-sm"
              data-testid="friend-request-accept"
              disabled={busy}
              onClick={() =>
                acceptMutation.mutate({ requestId: request.id, userId: request.fromUserId })
              }
              aria-label={t("friends.accept")}
              title={t("friends.accept")}
              className="text-accent hover:text-accent-deep"
            >
              <Check aria-hidden="true" />
            </Button>
            <Button
              variant="ghost"
              size="icon-sm"
              data-testid="friend-request-decline"
              disabled={busy}
              onClick={() =>
                declineMutation.mutate({ requestId: request.id, userId: request.fromUserId })
              }
              aria-label={t("friends.decline")}
              title={t("friends.decline")}
              className="text-ink-mute hover:text-danger"
            >
              <X aria-hidden="true" />
            </Button>
          </li>
        ))}
      </ul>
    </section>
  );
}
