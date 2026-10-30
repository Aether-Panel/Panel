import { cn } from "@/lib/utils";
import { useConfig } from "@/contexts/config-context";

export function Logo({ className }: Readonly<{ className?: string }>) {
  const { config } = useConfig();
  const panelName = config?.branding?.name || "Aether Panel";

  return (
    <div className={cn("flex items-center gap-2", className)}>
      {/* The panel name sits right beside the mark, so the image itself is decorative. */}
      <img src="/img/appicons/128.png" alt="" className="h-6 w-6 shrink-0 object-contain" />

      {/* in the collapsed sidebar rail only the mark shows; the name fades in while it opens */}
      <span className="truncate text-lg font-semibold tracking-[-0.02em] text-foreground transition-opacity duration-200 ease-panel motion-reduce:transition-none group-data-[collapsible=icon]:opacity-0 group-data-[collapsible=icon]:group-hover/rail:opacity-100 group-data-[collapsible=icon]:group-focus-within/rail:opacity-100">
        {panelName}
      </span>
    </div>
  );
}