import { cn } from "@/lib/utils";

interface PageHeaderProps {
  title: string;
  description?: string;
  children?: React.ReactNode;
  className?: string;
  titleClassName?: string;
}

export function PageHeader({ title, description, children, className, titleClassName }: PageHeaderProps) {
  return (
    <div className={cn("flex flex-col gap-4 md:flex-row md:items-center md:justify-between", className)}>
      <div className="space-y-1.5">
        <h1 className={cn("font-headline text-2xl font-semibold tracking-tight md:text-[1.75rem] md:leading-tight", titleClassName)}>{title}</h1>
        {description && (
          <p className="text-sm text-muted-foreground">{description}</p>
        )}
        <div className="h-px w-14 bg-border" />
      </div>
      {children && <div className="flex shrink-0 items-center gap-2">{children}</div>}
    </div>
  );
}
