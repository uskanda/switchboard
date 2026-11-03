import { useState } from "react";
import { initial, projectOf, useProjectEmoji } from "@/lib/projectIcons";
import { cn } from "@/lib/utils";

/**
 * ProjectIcon is the 16px icon of the project of a reference (DESIGN.md's 参照): the emoji of the configuration, else
 * the image switchboard-web passes on from the inbox service. Under the image lies the project's initial on muted
 * (project-initial), seen while the image loads and left alone when it fails. The icon is aria-hidden and the image's
 * alt empty: the reference beside it is what is read out. A string that is not a reference gets no icon.
 */
export function ProjectIcon({ value }: { value: string }) {
  const project = projectOf(value);
  const emoji = useProjectEmoji(project);
  const [state, setState] = useState<{ src: string; loaded: boolean; failed: boolean } | null>(null);
  if (!project) return null;
  const src = `/api/project-icon?ref=${encodeURIComponent(project)}`;
  const s = state?.src === src ? state : { src, loaded: false, failed: false };
  const showImage = !emoji && !s.failed;
  const hideInitial = showImage && s.loaded;
  return (
    <span data-project-icon aria-hidden="true" className="relative inline-flex size-4 shrink-0 items-center justify-center">
      {emoji ? (
        <span className="text-[13px] leading-none">{emoji}</span>
      ) : (
        !hideInitial && (
          <span className="inline-flex size-4 items-center justify-center rounded-full bg-muted text-[10px] font-semibold leading-none text-on-primary">
            {initial(project)}
          </span>
        )
      )}
      {showImage && (
        <img
          src={src}
          alt=""
          width={16}
          height={16}
          className={cn("absolute inset-0 size-4 rounded-sm object-cover", !s.loaded && "opacity-0")}
          onLoad={() => setState({ ...s, loaded: true })}
          onError={() => setState({ ...s, failed: true })}
        />
      )}
    </span>
  );
}
