import type { ComponentProps } from "react";
import { cn } from "@/lib/utils";

// shadcn/ui's Alert, as DESIGN.md's banner-error.
export function Alert({ className, ...props }: ComponentProps<"div">) {
  return <div role="alert" className={cn("rounded-sm bg-error-container p-sm text-on-error-container", className)} {...props} />;
}
