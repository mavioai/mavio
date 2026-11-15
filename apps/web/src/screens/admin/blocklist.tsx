// The Acquisition settings' blocklisted titles (docs/clients.md §6.3.11), as
// Seerr's Blocklist: the movies and series nobody can request, blocklisted
// from their pages, with their collections, or for blocklisted tags,
// newest first, found by name and by how they were blocklisted, each
// taken off the blocklist in place.
import { timestampDate } from "@bufbuild/protobuf/wkt";
import { errorMessage, type ServerEntry, useBlockedTitles, useRequestActions, useUsers } from "@mavio/client-ts";
import { MediaKind } from "@mavio/proto/mavio/acquisition/v1/profile_pb";
import { type BlockedTitle, BlockedTitleSource } from "@mavio/proto/mavio/acquisition/v1/request_pb";
import { Link } from "@tanstack/react-router";
import { useEffect, useState } from "react";

import { t } from "../../i18n.ts";
import { Button, Group, LoadMore } from "../../ui.tsx";
import { Select } from "../../controls/select.tsx";
import { input } from "../../controls/styles.ts";
import { kindParam } from "../discover.tsx";
import { BackLink, fromNow, Page, useChoice } from "./common.tsx";

/** The page of the blocklisted titles. */
export function BlockedTitlesPage({ server }: { server: ServerEntry }) {
  const [source, setSource] = useChoice<BlockedTitleSource>("filter", [BlockedTitleSource.UNSPECIFIED, BlockedTitleSource.MANUAL, BlockedTitleSource.TAGS]);
  const [text, setText] = useState("");
  const [search, setSearch] = useState("");
  useEffect(() => {
    const timer = setTimeout(() => setSearch(text.trim()), 300);
    return () => clearTimeout(timer);
  }, [text]);
  const list = useBlockedTitles(server, search, source);
  const users = useUsers(server);
  const titles = list.data?.pages.flatMap((p) => p.titles) ?? [];
  const total = list.data?.pages[0]?.total ?? 0;
  return (
    <Page
      title={t("blockedtitles.title")}
      message={t("blockedtitles.message")}
      back={<BackLink section="acquisition" label={t("settings.acquisition")} />}
    >
      <div className="flex flex-wrap items-center gap-2">
        <input
          type="search"
          className={`${input} min-w-0 flex-1 basis-60`}
          aria-label={t("blockedtitles.search")}
          placeholder={t("blockedtitles.search")}
          value={text}
          onChange={(e) => setText(e.target.value)}
        />
        <Select
          value={String(source)}
          label={t("blockedtitles.source")}
          options={[
            { value: String(BlockedTitleSource.UNSPECIFIED), label: t("blockedtitles.sourceAll") },
            { value: String(BlockedTitleSource.MANUAL), label: t("blockedtitles.sourceManual") },
            { value: String(BlockedTitleSource.TAGS), label: t("blockedtitles.sourceTags") },
          ]}
          onChange={(v) => setSource(Number(v) as BlockedTitleSource)}
        />
      </div>
      <Group footer={list.isSuccess && total > 0 ? t("blockedtitles.count", { count: total }) : undefined}>
        {titles.map((b) => (
          <BlockedTitleRow key={`${b.mediaKind}/${b.tmdbId}`} server={server} title={b} userName={users.data?.find((u) => u.id === b.userId)?.name ?? ""} />
        ))}
        {list.isSuccess && titles.length === 0 && <p className="px-4 py-3 type-callout text-text-secondary">{t("blockedtitles.empty")}</p>}
        {list.isError && <p className="px-4 py-3 type-callout break-words text-danger">{errorMessage(list.error)}</p>}
      </Group>
      <LoadMore active={list.hasNextPage && !list.isFetchingNextPage} onVisible={() => void list.fetchNextPage()} />
    </Page>
  );
}

/** A blocklisted title: its name opening its discovered page, its kind, who blocklisted it or its tags, and when. */
function BlockedTitleRow({ server, title: b, userName }: { server: ServerEntry; title: BlockedTitle; userName: string }) {
  const { unblockTitle } = useRequestActions(server);
  const made = b.createTime && timestampDate(b.createTime);
  const detail = [
    b.mediaKind === MediaKind.SERIES ? t("discover.series") : t("discover.movie"),
    b.tags.length > 0 ? t("blockedtitles.byTags") : userName && t("requests.by", { name: userName }),
    made && fromNow(made),
  ].filter(Boolean);
  return (
    <div className="flex min-h-13 flex-wrap items-center gap-x-3 gap-y-1 px-4 py-2.5">
      <div className="min-w-0 flex-1">
        <Link
          to="/s/$serverId/title/$kind/$tmdbId"
          params={{ serverId: server.id, kind: kindParam(b.mediaKind), tmdbId: String(b.tmdbId) }}
          className="type-body hover:underline"
        >
          {b.title || t("blockedtitles.unknown", { id: b.tmdbId })}
        </Link>
        <div className="type-footnote break-words text-text-secondary">{detail.join(" · ")}</div>
        {unblockTitle.isError && <div className="type-footnote break-words text-danger">{errorMessage(unblockTitle.error)}</div>}
      </div>
      <Button variant="secondary" disabled={unblockTitle.isPending} onClick={() => unblockTitle.mutate({ mediaKind: b.mediaKind, tmdbId: b.tmdbId })}>
        {t("collection.unblock")}
      </Button>
    </div>
  );
}
