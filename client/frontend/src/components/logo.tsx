import { cn } from "@/lib/utils";
import { useConfig } from "@/contexts/config-context";

export function Logo({ className }: { className?: string }) {
  const { config } = useConfig();
  const panelName = config?.branding?.name || "Aether Panel";

  return (
    <div className={cn("flex items-center gap-2 group-data-[state=collapsed]:justify-center", className)}>
      <img
        src="/img/appicons/144.png"
        alt="Logo"
        className="h-12 w-12 group-data-[state=collapsed]:h-8 group-data-[state=collapsed]:w-8"
      />

      <span className="text-lg font-semibold group-data-[state=collapsed]:hidden">
        <span className="text-gradient">{panelName}</span>
      </span>
    </div>
  );
}