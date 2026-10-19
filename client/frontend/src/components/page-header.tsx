import { cn } from "@/lib/utils";

interface PageHeaderProps {
  title: string;
  description?: string;
  /** Semantic status badges rendered inline after the title (e.g. a node or server state). */
  badges?: React.ReactNode;
  /** Right-aligned actions. Primary/secondary `<Button>`s read as one group. */
  children?: React.ReactNode;
  className?: string;
  titleClassName?: string;
}

export function PageHeader({ title, description, badges, children, className, titleClassName }: PageHeaderProps) {
  return (
    <div className={cn("flex flex-col gap-4 md:flex-row md:items-start md:justify-between", className)}>
      <div className="min-w-0 space-y-1.5">
        <div className="flex flex-wrap items-center gap-x-2.5 gap-y-1.5">
          <h1 className={cn("font-headline text-2xl font-semibold tracking-tight text-balance md:text-3xl md:leading-tight", titleClassName)}>{title}</h1>
          {badges && <div className="flex shrink-0 flex-wrap items-center gap-2">{badges}</div>}
        </div>
        {description && (
          <p className="max-w-[70ch] text-pretty text-sm text-muted-foreground">{description}</p>
        )}
      </div>
      {children && (
        <div className="flex shrink-0 flex-wrap items-center gap-2 md:justify-end">{children}</div>
      )}
    </div>
  );
}
