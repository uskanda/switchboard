import { cva, type VariantProps } from "class-variance-authority";
import type { ComponentProps } from "react";
import { cn } from "@/lib/utils";

// shadcn/ui's Badge, as DESIGN.md's badge-attention, badge-error, badge-github, badge-gitlab and badge-neutral.
const badgeVariants = cva("inline-flex items-center rounded-sm px-xs text-label-md font-medium", {
  variants: {
    tone: {
      attention: "bg-attention-container text-attention",
      error: "bg-error-container text-on-error-container",
      github: "bg-forge-github-container text-forge-github",
      gitlab: "bg-forge-gitlab-container text-forge-gitlab",
      neutral: "bg-outline text-on-surface",
    },
  },
  defaultVariants: { tone: "attention" },
});

export function Badge({ className, tone, ...props }: ComponentProps<"span"> & VariantProps<typeof badgeVariants>) {
  return <span className={cn(badgeVariants({ tone }), className)} {...props} />;
}
