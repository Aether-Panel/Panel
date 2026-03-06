import { cn } from "@/lib/utils";

import { useConfig } from "@/contexts/config-context";

export function Logo({ className }: { className?: string }) {
  const { config } = useConfig();
  const panelName = config?.branding?.name || "Aether Panel";

  return (
    <div className={cn("flex items-center gap-2", className)}>
      <svg
        xmlns="http://www.w3.org/2000/svg"
        viewBox="0 0 24 24"
        fill="none"
        stroke="currentColor"
        strokeWidth="2"
        strokeLinecap="round"
        strokeLinejoin="round"
        className="h-6 w-6 text-primary"
      >
        <path d="M12 2L2 7l10 5 10-5-10-5z" />
        <path d="M2 17l10 5 10-5" />
        <path d="M2 12l10 5 10-5" />
      </svg>
      <span className="text-lg font-semibold text-foreground">{panelName}</span>
    </div>
  );
}
