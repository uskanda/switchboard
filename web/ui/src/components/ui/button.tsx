import { cva, type VariantProps } from "class-variance-authority";
import type { ComponentProps } from "react";
import { cn } from "@/lib/utils";

// shadcn/ui's Button, with the variants DESIGN.md defines (button-primary, button-secondary).
const buttonVariants = cva(
  "inline-flex min-h-11 items-center justify-center gap-sm rounded-md px-md text-label-md font-medium focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary disabled:cursor-not-allowed disabled:opacity-50",
  {
    variants: {
      variant: {
        primary: "bg-primary text-on-primary",
        secondary: "border border-outline bg-surface-raised text-on-surface",
      },
    },
    defaultVariants: { variant: "primary" },
  },
);

export function Button({ className, variant, ...props }: ComponentProps<"button"> & VariantProps<typeof buttonVariants>) {
  return <button className={cn(buttonVariants({ variant }), className)} {...props} />;
}
