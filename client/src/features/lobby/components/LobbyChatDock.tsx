import { ChatDock } from "@/features/chat/ChatDock";

/**
 * The lobby's global chat dock. On a phone its launcher sits above the pinned New room bar
 * so the two never overlap.
 */
export function LobbyChatDock({ aboveBottomBar = false }: { aboveBottomBar?: boolean }) {
  return <ChatDock variant="lobby" launcherClassName={aboveBottomBar ? "bottom-19" : undefined} />;
}
