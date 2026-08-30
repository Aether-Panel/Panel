import { cn } from "@/lib/utils";
import { useConfig } from "@/contexts/config-context";

export function Logo({ className }: { className?: string }) {
  const { config } = useConfig();
  const panelName = config?.branding?.name || "Aether Panel";

  return (
    <div className={cn("flex items-center gap-2 group-data-[state=collapsed]:justify-center", className)}>
      <img src="/img/appicons/128.png" alt="Logo" className="h-6 w-6 object-contain" />

      <span className="text-lg font-semibold group-data-[state=collapsed]:hidden text-foreground">
        {panelName}
      </span>
    </div>
  );
}