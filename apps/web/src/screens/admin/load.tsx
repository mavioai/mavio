// The load of the server's computer, on the settings' Overview
// (docs/clients.md §6.3.6): a card each of its CPU, memory, disks and
// network, with what it is now, three figures more and a graph of the last
// two minutes.
import { errorMessage, type ServerEntry, useSystemLoad } from "@mavio/client-ts";
import type { LoadSample } from "@mavio/proto/mavio/system/v1/system_pb";
import type { ReactNode } from "react";

import { locale, t } from "../../i18n.ts";
import type { IconName } from "../../icon.tsx";
import { Group } from "../../ui.tsx";
import { capacityOf, ColorTile } from "./common.tsx";

/** The samples a graph has room for: two minutes', as the server keeps. */
const columns = 60;

/** The cards of the load of the server's computer. */
export function Load({ server }: { server: ServerEntry }) {
  const load = useSystemLoad(server);
  const samples = load.data ?? [];
  const now = samples.at(-1);
  return (
    <Group title={t("overview.load")} footer={t("overview.loadFooter")}>
      {load.isError ? (
        <p className="px-4 py-3 type-callout text-danger">{errorMessage(load.error)}</p>
      ) : (
        <div className="grid grid-cols-1 gap-px bg-border sm:grid-cols-2">
          <CpuCard now={now} samples={samples} />
          <MemoryCard now={now} samples={samples} />
          <DiskCard now={now} samples={samples} />
          <NetworkCard now={now} samples={samples} />
        </div>
      )}
    </Group>
  );
}

interface CardProps {
  now?: LoadSample;
  samples: LoadSample[];
}

function CpuCard({ now, samples }: CardProps) {
  const c = now?.cpu;
  return (
    <Card
      icon="cpu"
      color="#007AFF"
      title={t("overview.cpu")}
      told={!now || !!c}
      value={c && <Big text={percent.format(c.user + c.system)} />}
      stats={[
        [t("overview.cpuUser"), c && percent.format(c.user)],
        [t("overview.cpuSystem"), c && percent.format(c.system)],
        [t("overview.byServer"), c && percent.format(c.server)],
      ]}
      graph={samples.map((s) => s.cpu && s.cpu.user + s.cpu.system)}
    />
  );
}

function MemoryCard({ now, samples }: CardProps) {
  const m = now?.memory;
  const share = m && m.totalBytes > 0n ? Number(m.usedBytes) / Number(m.totalBytes) : undefined;
  const high = share !== undefined && share >= 0.9;
  return (
    <Card
      icon="memory"
      color="#34C759"
      title={t("overview.memory")}
      badge={
        share !== undefined && (
          <span className={`rounded-full px-2 py-0.5 type-caption ${high ? "bg-[#FF9500]/15 text-[#C93400]" : "bg-[#34C759]/15 text-[#248A3D]"}`}>
            {high ? t("overview.memoryHigh") : t("overview.memoryNormal")}
          </span>
        )
      }
      told={!now || !!m}
      value={m && <Big text={capacityOf(m.usedBytes)} after={t("overview.memoryOf", { total: capacityOf(m.totalBytes) })} />}
      stats={[
        [t("overview.memoryAvailable"), m && capacityOf(m.totalBytes - m.usedBytes)],
        [t("overview.swap"), m && (m.swapTotalBytes > 0n ? capacityOf(m.swapUsedBytes) : "–")],
        [t("overview.byServer"), m && capacityOf(m.serverBytes)],
      ]}
      graph={samples.map((s) => (s.memory && s.memory.totalBytes > 0n ? Number(s.memory.usedBytes) / Number(s.memory.totalBytes) : undefined))}
    />
  );
}

function DiskCard({ now, samples }: CardProps) {
  const d = now?.disk;
  return (
    <Card
      icon="disk"
      color="#FF9500"
      title={t("overview.disk")}
      told={!now || !!d}
      value={d && <Big text={rateOf(d.readRate)} />}
      caption={t("overview.read")}
      stats={[
        [t("overview.write"), d && rateOf(d.writeRate)],
        [t("overview.readTotal"), d && capacityOf(d.readBytes)],
        [t("overview.written"), d && capacityOf(d.writtenBytes)],
      ]}
      graph={scaled(
        samples.map((s) => s.disk && s.disk.readRate + s.disk.writeRate),
        1e6,
      )}
    />
  );
}

function NetworkCard({ now, samples }: CardProps) {
  const n = now?.network;
  return (
    <Card
      icon="network"
      color="#30B0C7"
      title={t("overview.network")}
      told={!now || !!n}
      value={n && <Big text={rateOf(n.receiveRate)} />}
      caption={t("overview.download")}
      stats={[
        [t("overview.upload"), n && rateOf(n.sendRate)],
        [t("overview.received"), n && capacityOf(n.receivedBytes)],
        [t("overview.sent"), n && capacityOf(n.sentBytes)],
      ]}
      graph={scaled(
        samples.map((s) => s.network && s.network.receiveRate + s.network.sendRate),
        1e5,
      )}
    />
  );
}

/**
 * A card: its tile and title, what it is now, large, three figures more
 * and the graph of the last samples. told is false where the server's
 * system does not tell it; while no sample came, the figures wait.
 */
function Card({
  icon,
  color,
  title,
  badge,
  told,
  value,
  caption,
  stats,
  graph,
}: {
  icon: IconName;
  color: string;
  title: string;
  badge?: ReactNode;
  told: boolean;
  value?: ReactNode;
  caption?: string;
  stats: [string, string | undefined][];
  graph: (number | undefined)[];
}) {
  return (
    <section aria-label={title} className="flex min-w-0 flex-col gap-3 bg-surface p-4">
      <header className="flex items-center gap-2">
        <ColorTile icon={icon} color={color} size={24} />
        <h3 className="type-headline" style={{ color }}>
          {title}
        </h3>
        <span className="ml-auto">{badge}</span>
      </header>
      {told ? (
        <>
          <div className="flex min-h-9 items-baseline gap-2">
            {value ?? <span className="type-title2 text-text-tertiary">…</span>}
            {caption && value && <span className="type-footnote text-text-secondary">{caption}</span>}
          </div>
          <dl className="grid grid-cols-3 gap-2">
            {stats.map(([label, v]) => (
              <div key={label} className="min-w-0">
                <dt className="truncate type-caption text-text-secondary">{label}</dt>
                <dd className="truncate type-footnote tabular-nums">{v ?? "…"}</dd>
              </div>
            ))}
          </dl>
          <Dots values={graph} color={color} />
        </>
      ) : (
        <p className="type-callout text-text-secondary">{t("overview.loadUnknown")}</p>
      )}
    </section>
  );
}

/** A figure large, its unit small, and what follows it. */
function Big({ text, after }: { text: string; after?: string }) {
  const m = /^([^\d]*)([\d.,\s]+)(.*)$/.exec(text);
  const [, before = "", digits = text, unit = ""] = m ?? [];
  return (
    <span className="flex items-baseline gap-1">
      <span className="type-title2 font-semibold tabular-nums">
        {before}
        {digits.trim()}
      </span>
      {unit && <span className="type-callout text-text-secondary">{unit.trim()}</span>}
      {after && <span className="ml-1 type-footnote text-text-secondary">{after}</span>}
    </span>
  );
}

/** Rows of dots in a column of the graph. */
const rows = 6;

/**
 * A graph of dots: a column per sample, the latest at the right, lit from
 * the bottom as high as its value, from 0 to 1; columns without a sample
 * stay unlit.
 */
function Dots({ values, color }: { values: (number | undefined)[]; color: string }) {
  const cell = 6;
  const shown = values.slice(-columns);
  const offset = columns - shown.length;
  const dots: ReactNode[] = [];
  for (let x = 0; x < columns; x++) {
    const v = x >= offset ? shown[x - offset] : undefined;
    // A value above nothing lights a dot, so that activity shows.
    const lit = v === undefined ? 0 : v > 0 ? Math.max(1, Math.round(Math.min(1, v) * rows)) : 0;
    for (let y = 0; y < rows; y++) {
      const on = rows - y <= lit;
      dots.push(<circle key={`${x}.${y}`} cx={x * cell + cell / 2} cy={y * cell + cell / 2} r={1.7} fill={on ? color : "currentColor"} opacity={on ? 1 : 0.12} />);
    }
  }
  return (
    <svg viewBox={`0 0 ${columns * cell} ${rows * cell}`} className="w-full text-text-secondary" aria-hidden="true">
      {dots}
    </svg>
  );
}

/** Rates as shares of the highest of them, or of floor when all are lower. */
function scaled(values: (number | undefined)[], floor: number): (number | undefined)[] {
  const top = Math.max(floor, ...values.filter((v) => v !== undefined));
  return values.map((v) => v && v / top);
}

const percent = new Intl.NumberFormat([locale], { style: "percent", maximumFractionDigits: 0 });
const rates = (["kilobyte-per-second", "megabyte-per-second", "gigabyte-per-second"] as const).map((unit) =>
  [1, 0].map((digits) => new Intl.NumberFormat([locale], { style: "unit", unit, maximumFractionDigits: digits })),
);

/** Bytes per second in kilobytes, megabytes or gigabytes as it is large, a decimal only below ten. */
function rateOf(bytes: number): string {
  const [n, formats] = bytes >= 1e9 ? [bytes / 1e9, rates[2]] : bytes >= 1e6 ? [bytes / 1e6, rates[1]] : [bytes / 1e3, rates[0]];
  return formats[n < 10 ? 0 : 1].format(n);
}
