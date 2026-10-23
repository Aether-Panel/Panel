import * as React from "react"
import { cva, type VariantProps } from "class-variance-authority"

import { cn } from "@/lib/utils"

const badgeVariants = cva(
  "inline-flex items-center whitespace-nowrap rounded-md border px-2 py-0.5 text-xs font-medium ring-offset-background transition-colors duration-200 focus:outline-none focus-visible:ring-2 focus-visible:ring-ring/70 focus-visible:ring-offset-2",
  {
    variants: {
      variant: {
        default:
          "border-primary/25 bg-primary/10 text-primary-bright",
        secondary:
          "border-border/50 bg-secondary text-secondary-foreground",
        destructive:
          "border-destructive/25 bg-destructive/10 text-destructive-bright",
        outline: "text-foreground border-border/60",
      },
    },
    defaultVariants: {
      variant: "default",
    },
  }
)

export interface BadgeProps
  extends React.HTMLAttributes<HTMLDivElement>,
    VariantProps<typeof badgeVariants> {}

function Badge({ className, variant, ...props }: BadgeProps) {
  return (
    <div className={cn(badgeVariants({ variant }), className)} {...props} />
  )
}

export { Badge, badgeVariants }
