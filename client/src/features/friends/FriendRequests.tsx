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
 * The viewer's incoming friend-requests surface in the lobby (Story 11.2, AC4 +
 * AC8). Each row shows the sender's username with Accept / Decline actions wired
 * to the friend mutations. This list is the DURABLE delivery path — the WS
 * system:friend_request push only invalidates it (best-effort, online-only).
 *
 * A SECTION inside the Friends card, not a card of its own: requests and friends
 * are one relationship in two states, and giving each a card meant two headings,
 * two borders and two paddings to show what is usually two or three names. It
 * renders NOTHING unless there is at least one pending request — an empty inbox
 * is the normal state and is not worth the row it takes to announce itself. No
 * loading placeholder either, for the same reason: it would resolve to nothing.
 *
 * Requests tile into an auto-filling grid rather than stacking full width, so a
 * handful of names occupies one line on a wide lobby instead of three.
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
        {/* The count earns its place once the tiles wrap onto a second line and
            the eye can no longer take them in at a glance. */}
        <span className="text-brass-deep font-mono text-[10.5px] font-semibold tabular-nums">
          {requests.length}
        </span>
      </div>

      <ul className="grid grid-cols-[repeat(auto-fill,minmax(15rem,1fr))] list-none gap-1.5 p-0">
        {requests.map((request) => (
          <li
            key={request.id}
            className="bg-surface-elevated border-border flex items-center gap-2 rounded-[10px] border px-2.5 py-1.5"
            data-testid="friend-request-row"
            data-request-id={request.id}
          >
            {/* Avatar + name open the sender's public profile, exactly as a friend
                row does — deciding on a request usually means looking the person
                up first, and the two surfaces sit in the same card. */}
            <button
              type="button"
              onClick={() => navigate(`/players/${request.fromUserId}`)}
              aria-label={t("friends.viewProfileAria", { username: request.fromUsername })}
              data-testid="friend-request-profile"
              className="hover:bg-surface-sunken focus-visible:ring-ring/50 flex min-w-0 flex-1 cursor-pointer items-center gap-2 rounded-md text-left transition-colors focus-visible:ring-3 focus-visible:outline-none"
            >
              <Avatar name={request.fromUsername} size={28} className="shrink-0" />
              <span className="text-ink truncate text-sm font-medium">{request.fromUsername}</span>
            </button>
            {/* Bare glyphs, no chip: a tick and a cross beside a pending name are
                as legible as the verbs at a third of the width, and boxing each
                one put two more borders inside a tile that already has one. Solid
                ink, not a tinted fill — green reads accept, grey reads dismiss,
                and hover confirms which is which. The verbs stay as the accessible
                name and the hover title. The size-6 box is the tap target only. */}
            <button
              type="button"
              data-testid="friend-request-accept"
              disabled={busy}
              onClick={() =>
                acceptMutation.mutate({ requestId: request.id, userId: request.fromUserId })
              }
              aria-label={t("friends.accept")}
              title={t("friends.accept")}
              className="text-accent hover:text-accent-deep focus-visible:ring-ring/50 flex size-6 shrink-0 cursor-pointer items-center justify-center rounded-md transition-colors focus-visible:ring-3 focus-visible:outline-none disabled:opacity-50"
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
              className="text-ink-mute hover:text-danger focus-visible:ring-ring/50 flex size-6 shrink-0 cursor-pointer items-center justify-center rounded-md transition-colors focus-visible:ring-3 focus-visible:outline-none disabled:opacity-50"
            >
              <X className="size-4" aria-hidden="true" />
            </button>
          </li>
        ))}
      </ul>
    </section>
  );
}
