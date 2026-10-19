// A message of a conversation with the assistant or a model (docs/clients.md
// §6.3): its bubble, and the bar under it with its time, a button copying it
// and, when it can be spoken, one playing it.
import { useState } from "react";

import { Tip } from "../controls/tooltip.tsx";
import { locale, t } from "../i18n.ts";
import { Icon, type IconName } from "../icon.tsx";

const when = new Intl.DateTimeFormat(locale, { dateStyle: "medium", timeStyle: "short" });
const clock = new Intl.DateTimeFormat(locale, { timeStyle: "short" });

/** The classes of a message's bubble on a background of bg: the user's and the answer's alike, the answer's in a dashed border. */
export function bubble(user: boolean, bg = "bg-surface"): string {
  return `max-w-[min(100%,42rem)] rounded-lg px-3 py-2 type-callout break-words whitespace-pre-wrap ${bg} ${user ? "" : "border border-dashed border-accent/50"}`;
}

/** Buttons copying a message and, given onPlay, playing it, then its time. */
export function MessageBar({
  text,
  at,
  playing,
  onPlay,
}: {
  text: string;
  at?: Date;
  playing?: boolean;
  onPlay?: () => void;
}) {
  const [copied, setCopied] = useState(false);
  return (
    <div className="flex items-center gap-0.5">
      <BarButton
        icon={copied ? "check" : "copy"}
        label={copied ? t("assistant.copiedMessage") : t("assistant.copyMessage")}
        onClick={() =>
          void navigator.clipboard?.writeText(text).then(
            () => (setCopied(true), setTimeout(() => setCopied(false), 2000)),
            () => {},
          )
        }
      />
      {onPlay && <BarButton icon={playing ? "pause" : "play"} label={playing ? t("assistant.stopAnswer") : t("assistant.playAnswer")} onClick={onPlay} />}
      {at && (
        <time dateTime={at.toISOString()} title={when.format(at)} className="px-1 type-caption text-text-tertiary">
          {at.toDateString() === new Date().toDateString() ? clock.format(at) : when.format(at)}
        </time>
      )}
    </div>
  );
}

function BarButton({ icon, label, onClick }: { icon: IconName; label: string; onClick: () => void }) {
  return (
    <Tip label={label}>
      <button
        type="button"
        aria-label={label}
        className="flex size-7 items-center justify-center rounded-md text-text-tertiary hover:bg-fill hover:text-on-background"
        onClick={onClick}
      >
        <Icon name={icon} size={15} />
      </button>
    </Tip>
  );
}
