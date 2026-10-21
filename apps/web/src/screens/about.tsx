// About, the settings' last section, as the apps': the web UI, its name as
// the servers know it, its version and license; and this browser, the name
// and ID the servers know it by, the browser, the system and the
// architecture, as far as the browser tells them.

import { useServersStore } from "@mavio/client-ts";
import { useEffect, useState } from "react";

import { t } from "../i18n.ts";
import { Group, Row } from "../ui.tsx";
import { Page } from "./admin/common.tsx";

/** What a Chromium browser tells of itself (User-Agent Client Hints); others have none. */
interface UAData {
  getHighEntropyValues(hints: string[]): Promise<{
    platform?: string;
    platformVersion?: string;
    architecture?: string;
    bitness?: string;
    model?: string;
    fullVersionList?: { brand: string; version: string }[];
  }>;
}

/** The browser, the system and the architecture, from the client hints or else the user agent. */
interface Facts {
  browser?: string;
  system?: string;
  architecture?: string;
  model?: string;
}

/** The browser and system the user agent names, where the client hints do not. */
function fromUserAgent(ua: string): Facts {
  const browser =
    ua.match(/(Firefox)\/([\d.]+)/) ?? ua.match(/(Edg)\/([\d.]+)/) ?? ua.match(/(Chrome)\/([\d.]+)/) ?? ua.match(/Version\/([\d.]+).*(Safari)/);
  const named = browser && (browser[1] === "Edg" ? `Edge ${browser[2]}` : browser[2] === "Safari" ? `Safari ${browser[1]}` : `${browser[1]} ${browser[2]}`);
  const ios = ua.match(/(iPhone|iPad).*? OS ([\d_]+)/);
  const mac = ua.match(/Mac OS X ([\d_]+)/);
  const system = ios
    ? `${ios[1] === "iPad" ? "iPadOS" : "iOS"} ${ios[2].replaceAll("_", ".")}`
    : (ua.match(/Android [\d.]+/)?.[0] ??
      (mac ? `macOS ${mac[1].replaceAll("_", ".")}` : (ua.match(/Windows NT [\d.]+/)?.[0] ?? (/Linux/.test(ua) ? "Linux" : undefined))));
  return { browser: named ?? undefined, system };
}

async function facts(): Promise<Facts> {
  const guessed = fromUserAgent(navigator.userAgent);
  const ua = (navigator as Navigator & { userAgentData?: UAData }).userAgentData;
  if (!ua) return guessed;
  try {
    const v = await ua.getHighEntropyValues(["platformVersion", "architecture", "bitness", "model", "fullVersionList"]);
    const brand = v.fullVersionList?.find((b) => !/Not.?A.?Brand|Chromium/i.test(b.brand)) ?? v.fullVersionList?.find((b) => b.brand === "Chromium");
    // The user agent's system is frozen (macOS 10.15.7, Windows NT 10.0); the hints tell the real one, Windows 11 from 13.
    const major = Number(v.platformVersion?.split(".")[0]);
    const system = !v.platformVersion
      ? guessed.system
      : v.platform === "Windows"
        ? `Windows ${major >= 13 ? 11 : 10} (${v.platformVersion})`
        : `${v.platform} ${v.platformVersion}`;
    return {
      browser: brand ? `${brand.brand} ${brand.version}` : guessed.browser,
      system,
      architecture: v.architecture ? `${v.architecture}${v.bitness ? ` ${v.bitness}-bit` : ""}` : undefined,
      model: v.model || undefined,
    };
  } catch {
    return guessed;
  }
}

/** A fact: its label and its value, which wraps when it is long, as a device's ID. */
function Fact({ label, value }: { label: string; value?: string }) {
  if (!value) return null;
  return (
    <Row label={label}>
      <span className="min-w-0 type-body break-all text-text-secondary sm:max-w-[60%] sm:text-end">{value}</span>
    </Row>
  );
}

/** The section of the settings. */
export function AboutSection() {
  const device = useServersStore().device();
  const [known, setKnown] = useState<Facts>(() => fromUserAgent(navigator.userAgent));
  useEffect(() => {
    let live = true;
    void facts().then((f) => live && setKnown(f));
    return () => {
      live = false;
    };
  }, []);
  return (
    <Page title={t("settings.about")}>
      <Group title={t("about.app")}>
        <Fact label={t("about.appName")} value={device.client} />
        <Fact label={t("about.clientVersion")} value={import.meta.env.MAVIO_VERSION as string} />
        <Fact label={t("about.license")} value="GPL-3.0" />
      </Group>
      <Group title={t("about.device")} footer={t("about.deviceFooter")}>
        <Fact label={t("about.deviceName")} value={device.name} />
        <Fact label={t("about.deviceId")} value={device.id} />
        <Fact label={t("about.browser")} value={known.browser} />
        <Fact label={t("about.model")} value={known.model} />
        <Fact label={t("about.system")} value={known.system} />
        <Fact label={t("about.architecture")} value={known.architecture} />
      </Group>
    </Page>
  );
}
