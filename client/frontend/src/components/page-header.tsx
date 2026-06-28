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
      <div className="space-y-1">
        <h1 className={cn("text-2xl font-bold tracking-tight md:text-3xl", titleClassName)}>{title}</h1>
        {description && (
          <p className="text-muted-foreground">{description}</p>
        )}
        <div className="h-0.5 w-14 bg-gradient-to-r from-primary to-accent rounded-full" />
      </div>
      {children && <div className="flex shrink-0 items-center gap-2">{children}</div>}
    </div>
  );
}
