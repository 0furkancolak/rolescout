import { ArrowLeft } from "lucide-react";
import { Link } from "react-router-dom";

export function BackLink({ to, children }: { to: string; children: string }) {
  return (
    <Link to={to} className="inline-flex items-center gap-1 text-sm text-ink-muted hover:text-ink">
      <ArrowLeft aria-hidden className="size-4" />
      {children}
    </Link>
  );
}
