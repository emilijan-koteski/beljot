import { Flag } from "lucide-react";
import type { ReactNode } from "react";

import { useFocusTrap } from "@/shared/hooks/useFocusTrap";
import { Z } from "@/shared/lib/zLayers";

import { ClassicActions } from "./overlay/ClassicActions";
import { ClassicPanel } from "./overlay/ClassicPanel";
import { ExplainerStrip } from "./overlay/ExplainerStrip";
import { OverlayBackdrop } from "./overlay/OverlayBackdrop";

const DANGER = "var(--danger-light)";

interface SurrenderDialogTestIds {
  overlay: string;
  confirm: string;
  cancel: string;
}

interface SurrenderDialogProps {
  titleId: string;
  title: ReactNode;
  body: ReactNode;
  cost: ReactNode;
  confirmLabel: ReactNode;
  cancelLabel: ReactNode;
  onConfirm: () => void;
  onCancel: () => void;
  testIds: SurrenderDialogTestIds;
}

/**
 * Shell of both surrender dialogs: a danger-glow panel with a flag title, what happens next,
 * what it costs, and a danger confirm beside a ghost cancel. Escape cancels.
 */
export function SurrenderDialog({
  titleId,
  title,
  body,
  cost,
  confirmLabel,
  cancelLabel,
  onConfirm,
  onCancel,
  testIds,
}: SurrenderDialogProps) {
  const dialogRef = useFocusTrap<HTMLDivElement>({ onEscape: onCancel });

  return (
    <div className="fixed inset-0" style={{ zIndex: Z.SURRENDER }} data-testid={testIds.overlay}>
      <OverlayBackdrop dim="prompt">
        <div
          ref={dialogRef}
          role="dialog"
          aria-modal="true"
          aria-labelledby={titleId}
          className="motion-safe:animate-in motion-safe:zoom-in-95 motion-safe:duration-150"
        >
          <ClassicPanel
            size="dialog"
            glowColor={DANGER}
            title={
              <span id={titleId} className="inline-flex items-center gap-2.5">
                <Flag size={20} style={{ color: DANGER }} aria-hidden="true" />
                {title}
              </span>
            }
          >
            <p
              className="mb-4 text-sm"
              style={{ color: "var(--ink-light)", opacity: 0.8, lineHeight: 1.6 }}
            >
              {body}
            </p>
            <ExplainerStrip tone="danger" className="mb-4">
              {cost}
            </ExplainerStrip>
            <ClassicActions
              tone="danger"
              primary={{ label: confirmLabel, onClick: onConfirm, testId: testIds.confirm }}
              secondary={{ label: cancelLabel, onClick: onCancel, testId: testIds.cancel }}
            />
          </ClassicPanel>
        </div>
      </OverlayBackdrop>
    </div>
  );
}
