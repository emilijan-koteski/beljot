import { ExternalLink, Mail } from "lucide-react";
import { useState } from "react";
import { useTranslation } from "react-i18next";

import { AppDialog } from "@/shared/components/ui/app-dialog";

const EMAIL = "emilijankoteski@pm.me";
const LINKEDIN_URL = "https://www.linkedin.com/in/emilijan-koteski/";

/** Footer "Contact" link that opens a dialog with the maker's email, LinkedIn and a beta note. */
export function ContactDialog() {
  const { t } = useTranslation();
  const [open, setOpen] = useState(false);
  const close = () => setOpen(false);

  return (
    <>
      <button
        type="button"
        onClick={() => setOpen(true)}
        data-testid="landing-contact"
        className="hover:text-ink cursor-pointer transition-colors"
      >
        {t("landing.foot.contact")}
      </button>
      <AppDialog
        open={open}
        onClose={close}
        icon={<Mail />}
        title={t("landing.contact.title")}
        description={t("landing.contact.lead")}
        primary={{ label: t("common.close"), onClick: close, testId: "contact-close" }}
        testId="contact-dialog"
      >
        <div className="flex flex-col gap-2.5">
          <div className="text-ink text-sm font-semibold">Emilijan Koteski</div>
          <a
            href={`mailto:${EMAIL}`}
            className="text-accent hover:text-accent-deep inline-flex items-center gap-2 text-sm font-medium"
            data-testid="contact-email"
          >
            <Mail className="size-4" />
            {EMAIL}
          </a>
          <a
            href={LINKEDIN_URL}
            target="_blank"
            rel="noopener noreferrer"
            className="text-accent hover:text-accent-deep inline-flex items-center gap-2 text-sm font-medium"
            data-testid="contact-linkedin"
          >
            <ExternalLink className="size-4" />
            {t("landing.contact.linkedin")}
          </a>
        </div>
        <p className="text-ink-mute mt-4 mb-0 text-xs">{t("landing.contact.disclaimer")}</p>
      </AppDialog>
    </>
  );
}
