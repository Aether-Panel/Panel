import { cn } from "@/lib/utils";
import { useConfig } from "@/contexts/config-context";

export function Logo({ className }: Readonly<{ className?: string }>) {
  const { config } = useConfig();
  const panelName = config?.branding?.name || "Aether Panel";

  return (
    <div className={cn("flex items-center gap-2 group-data-[state=collapsed]:justify-center", className)}>
      {/* The panel name sits right beside the mark, so the image itself is decorative. */}
      <img src="/img/appicons/128.png" alt="" className="h-6 w-6 shrink-0 object-contain" />

      <span className="truncate text-lg font-semibold tracking-[-0.02em] text-foreground group-data-[state=collapsed]:hidden">
        {panelName}
      </span>
    </div>
  );
}