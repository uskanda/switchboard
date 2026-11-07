import { clsx, type ClassValue } from "clsx";
import { extendTailwindMerge } from "tailwind-merge";

/**
 * twMerge knows the design tokens of tokens.css (DESIGN.md), so p-0 replaces p-lg and text-label-md (a size) is kept
 * beside text-muted (a color). Without them, an unknown token is merged as nothing, or as a color.
 */
const twMerge = extendTailwindMerge({
  extend: {
    theme: {
      spacing: ["xs", "sm", "md", "lg", "xl"],
      text: ["headline-md", "body-md", "label-md", "mono-sm"],
      radius: ["sm", "md", "full"],
    },
  },
});

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs));
}
