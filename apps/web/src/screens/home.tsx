// Home: the hubs of every server, merged as Plex does (docs/clients.md
// §5.4, §6.3.1), each server's drawn as it arrives in its fixed place.
import { type Section, useHome, useServers, useSignedIn } from "@mavio/client-ts";
import { HubKind } from "@mavio/proto/mavio/library/v1/library_pb";
import { Link } from "@tanstack/react-router";

import { t } from "../i18n.ts";
import { Icon } from "../icon.tsx";
import { usePrefs } from "../prefs.ts";
import { Artwork, CardPlaceholder, ItemCard, Shelf, StatusView } from "../ui.tsx";
import { Hero, heroEntries } from "./hero.tsx";

/** The title of a section, named by its kind, library and seed. */
export function sectionTitle(s: Section): string {
  const library = s.libraryName ?? "";
  const seed = s.seedName ?? "";
  switch (s.kind) {
    case HubKind.CONTINUE_WATCHING:
      return t("home.continueWatching");
    case HubKind.RECENTLY_ADDED:
      return t("home.latest", { library });
    case HubKind.RECENTLY_RELEASED:
      return t("home.recentlyReleased", { library });
    case HubKind.RECENTLY_AIRED:
      return t("home.recentlyAired", { library });
    case HubKind.TOP_RATED:
      return t("home.topRated", { library });
    case HubKind.TOP_UNPLAYED:
      return t("home.topUnplayed", { library });
    case HubKind.START_WATCHING:
      return t("home.startWatching", { library });
    case HubKind.REDISCOVER:
      return t("home.rediscover", { library });
    case HubKind.BECAUSE_YOU_WATCHED:
      return t("home.becauseYouWatched", { title: seed });
    case HubKind.BECAUSE_YOU_LIKE:
      return t("home.becauseYouLike", { title: seed });
    case HubKind.MORE_IN_GENRE:
      return t("home.moreInGenre", { genre: seed });
    case HubKind.STARRING:
      return t("home.starring", { name: seed });
    case HubKind.DIRECTED_BY:
      return t("home.directedBy", { name: seed });
    case HubKind.AVAILABLE_REQUESTS:
      return t("home.availableRequests");
    case HubKind.TRENDING:
      return t("home.trending");
    default:
      return s.serverName ?? "";
  }
}

export function HomeScreen() {
  const sections = useHome();
  const several = useSignedIn().length > 1;
  const servers = useServers();
  const signedOut = servers.filter((s) => !s.token);
  const hero = heroEntries(sections, servers);
  const { tv } = usePrefs();
  return (
    <div className={hero.length ? "" : tv ? "pt-32" : "pt-4"}>
      <Hero entries={hero} />
      {hero.length > 0 && <div className="h-4" />}
      {sections.map((s) => (
        <SectionView key={s.id} section={s} several={several} />
      ))}
      {sections.length === 0 && <StatusView title={t("state.empty")} />}
      {signedOut.length > 0 && (
        <p className="px-margin pt-6 type-footnote text-text-tertiary">
          {signedOut.map((s) => `${s.name}: ${t("auth.signedOut")}`).join(" · ")}
        </p>
      )}
    </div>
  );
}

/** A section of Home or of a library's Recommended tab. */
export function SectionView({ section: s, several }: { section: Section; several: boolean }) {
  const servers = useServers();
  const server = (id?: string) => servers.find((x) => x.id === id);
  const c = s.content;
  if (c.type === "unavailable") {
    return (
      <p className="flex items-center gap-2 px-margin py-3 type-callout text-text-tertiary">
        <Icon name="offline" size={18} />
        {t("home.serverUnreachable", { server: s.serverName ?? "" })}
      </p>
    );
  }
  const badge = (
    <>
      {several && s.serverName && s.kind !== HubKind.CONTINUE_WATCHING && (
        <span className="shrink-0 rounded-sm bg-surface-raised px-1.5 py-0.5 type-caption text-text-secondary">
          {s.serverName}
        </span>
      )}
      {s.offline && (
        <span className="flex shrink-0 items-center gap-1 type-caption text-text-tertiary">
          <Icon name="offline" size={14} />
          {t("home.offline")}
        </span>
      )}
    </>
  );
  if (c.type === "loading") {
    return (
      <Shelf title={<span className="text-text-tertiary">{s.serverName}</span>}>
        {Array.from({ length: 8 }, (_, i) => (
          <CardPlaceholder key={i} />
        ))}
      </Shelf>
    );
  }
  const more =
    s.more && s.serverId && s.hubId ? (
      <Link to="/s/$serverId/hub/$hubId" params={{ serverId: s.serverId, hubId: s.hubId }}>
        {t("home.more")}
      </Link>
    ) : undefined;
  if (c.type === "titles") {
    return (
      <Shelf
        title={sectionTitle(s)}
        badge={<span className="shrink-0 type-footnote text-text-tertiary">{t("home.notOnServers")}</span>}
        more={more}
      >
        {c.titles.map(({ title, available }) => {
          const card = (
            <>
              <Artwork src={title.posterUrl || undefined} alt={title.name} className="aspect-[2/3] rounded-md" />
              <div className="mt-2 truncate type-headline">{title.name}</div>
              <div className="truncate type-footnote text-text-secondary">{title.year || ""}</div>
            </>
          );
          const at = available[0];
          return at ? (
            <Link
              key={`${title.name}-${title.year}`}
              to="/s/$serverId/item/$itemId"
              params={{ serverId: at.serverId, itemId: at.itemId }}
              className="block w-poster shrink-0 snap-start"
            >
              {card}
            </Link>
          ) : (
            <div key={`${title.name}-${title.year}`} className="w-poster shrink-0 snap-start opacity-80">
              {card}
            </div>
          );
        })}
      </Shelf>
    );
  }
  const continueWatching = s.kind === HubKind.CONTINUE_WATCHING;
  return (
    <Shelf title={sectionTitle(s)} badge={badge} more={more}>
      {c.entries.map((e) => {
        const srv = server(e.serverId);
        if (!srv) return null;
        return (
          <ItemCard
            key={`${e.serverId}/${e.item.id}`}
            server={srv}
            serverId={e.serverId}
            item={e.item}
            shape={continueWatching ? "thumb" : undefined}
          />
        );
      })}
    </Shelf>
  );
}
