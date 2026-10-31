import type { ComponentProps } from "react";
import { cn } from "@/lib/utils";

// shadcn/ui's Table, with rows divided by the outline.
export const Table = ({ className, ...p }: ComponentProps<"table">) => <table className={cn("w-full border-collapse text-left", className)} {...p} />;
export const TableHead = ({ className, ...p }: ComponentProps<"th">) => (
  <th className={cn("border-b border-outline px-sm py-xs text-label-md font-medium text-muted", className)} {...p} />
);
export const TableCell = ({ className, ...p }: ComponentProps<"td">) => <td className={cn("border-b border-outline px-sm py-sm align-top", className)} {...p} />;
