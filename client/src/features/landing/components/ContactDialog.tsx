import { ExternalLink, Mail } from "lucide-react";
import { useTranslation } from "react-i18next";

import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/shared/components/ui/dialog";

const EMAIL = "emilijankoteski@pm.me";
const LINKEDIN_URL = "https://www.linkedin.com/in/emilijan-koteski/";

// Footer "Contact" link that opens a dialog with the maker's email, LinkedIn and a beta disclaimer.
export function ContactDialog() {
  const { t } = useTranslation();
  return (
    <Dialog>
      <DialogTrigger
        data-testid="landing-contact"
        className="hover:text-ink cursor-pointer transition-colors"
      >
        {t("landing.foot.contact")}
      </DialogTrigger>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>{t("landing.contact.title")}</DialogTitle>
          <DialogDescription>{t("landing.contact.lead")}</DialogDescription>
        </DialogHeader>

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

        <div className="border-border bg-surface-sunken text-ink-dim rounded-lg border px-3.5 py-3 text-xs leading-relaxed">
          {t("landing.contact.disclaimer")}
        </div>
      </DialogContent>
    </Dialog>
  );
}
