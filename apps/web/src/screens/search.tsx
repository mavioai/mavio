// Search asks every server at once and lists the results by server
// (docs/clients.md §5.4), and the server that discovers for titles to
// request (§6.3.28). A search runs when it is confirmed; on the
// television it runs as the on-screen keyboard types, beside the results
// (docs/tv.md §6.4).
import { type SearchResults, titlesOf, useDiscovery, useHome, useSearch, useSearchTitles, useSignedIn } from "@mavio/client-ts";
import { getRouteApi, useNavigate } from "@tanstack/react-router";
import { type CSSProperties, type KeyboardEvent, useEffect, useRef, useState } from "react";

import { discoverLanguage, t } from "../i18n.ts";
import { Icon } from "../icon.tsx";
import { usePrefs, useWidthClass } from "../prefs.ts";
import { CardPlaceholder, Grid, ItemCard, Shelf, StatusView } from "../ui.tsx";
import { tmdbOf, TitleCard } from "./discover.tsx";
import { SectionView } from "./home.tsx";

const route = getRouteApi("/search");

export function SearchScreen() {
  const { tv } = usePrefs();
  return tv ? <TvSearch /> : <StandardSearch />;
}

/** The search of the phone and desktop layouts: a field, confirmed with Return. */
function StandardSearch() {
  const { q = "" } = route.useSearch();
  const navigate = useNavigate();
  const [text, setText] = useState(q);
  useEffect(() => setText(q), [q]);
  const { sidebar } = usePrefs();
  // The expanded sidebar has the field already.
  const field = !(useWidthClass() === "expanded" && sidebar);
  const results = useSearch(q);
  return (
    <div className="pt-4">
      {field && (
        <form
          role="search"
          className="px-margin pb-2"
          onSubmit={(e) => {
            e.preventDefault();
            void navigate({ to: "/search", search: { q: text.trim() || undefined } });
          }}
        >
          <label className="flex min-h-12 items-center gap-3 rounded-lg bg-surface-raised px-4">
            <Icon name="search" className="text-text-secondary" />
            <input
              type="search"
              value={text}
              onChange={(e) => setText(e.target.value)}
              placeholder={t("search.placeholder")}
              aria-label={t("nav.search")}
              autoFocus={q === ""}
              enterKeyHint="search"
              className="w-full bg-transparent type-body outline-none placeholder:text-text-tertiary"
            />
          </label>
        </form>
      )}
      <Results q={q} results={results} grid={false} />
    </div>
  );
}

const keys = [..."abcdefghijklmnopqrstuvwxyz1234567890"];

/**
 * The television's search: on the left the query, space, delete and
 * clear, and a keyboard of letters and digits; on the right, Home's rows
 * while the query is empty, and the results as it is typed.
 */
function TvSearch() {
  const { q = "" } = route.useSearch();
  const navigate = useNavigate();
  const [text, setText] = useState(q);
  // The URL follows the query after a pause, so that results are linkable,
  // and the query follows the URL when it changes otherwise, going back.
  const pushed = useRef(q);
  useEffect(() => {
    if (q !== pushed.current) setText(q);
    pushed.current = q;
  }, [q]);
  useEffect(() => {
    const h = setTimeout(() => {
      if (text.trim() === q) return;
      pushed.current = text.trim();
      void navigate({ to: "/search", search: { q: text.trim() || undefined }, replace: true });
    }, 300);
    return () => clearTimeout(h);
  }, [text, q, navigate]);
  const results = useSearch(q);
  const type = (c: string) => setText((s) => s + c);
  const erase = () => setText((s) => [...s].slice(0, -1).join(""));
  // Within the keyboard back deletes a character, and goes back only once
  // there is none; a keyboard types as well.
  const onKeyDown = (e: KeyboardEvent) => {
    if (e.metaKey || e.ctrlKey || e.altKey) return;
    if (e.key === "Backspace" || e.key === "Escape") {
      if (text === "") return;
      erase();
    } else if (e.key.length === 1) type(e.key);
    else return;
    e.preventDefault();
    e.stopPropagation();
  };
  const key = "tv-pill flex items-center justify-center rounded-lg bg-fill type-headline";
  return (
    <div className="flex items-start gap-gutter pl-margin">
      <div role="search" data-focus-region onKeyDown={onKeyDown} className="sticky top-12 flex w-[clamp(300px,30vw,480px)] shrink-0 flex-col gap-4">
        <div aria-live="polite" className="flex min-h-16 items-center gap-3 rounded-lg bg-surface-raised px-5">
          <Icon name="search" className="shrink-0 text-text-secondary" />
          {/* The caret shows where the keyboard types, wherever focus is. */}
          <span className="flex min-w-0 flex-1 items-center type-title3">
            <span className="truncate whitespace-pre">{text}</span>
            <span aria-hidden className="caret mx-px h-[1.1em] w-0.5 shrink-0 rounded-full bg-accent" />
            {!text && <span className="truncate text-text-tertiary">{t("search.placeholder")}</span>}
          </span>
        </div>
        <div className="grid grid-cols-3 gap-2">
          <button type="button" onClick={() => type(" ")} className={`${key} h-14`}>
            {t("search.space")}
          </button>
          <button type="button" onClick={erase} className={`${key} h-14`}>
            {t("search.backspace")}
          </button>
          <button type="button" onClick={() => setText("")} className={`${key} h-14`}>
            {t("search.clear")}
          </button>
        </div>
        <div className="grid grid-cols-6 gap-2">
          {/* The page has no tabs, so focus starts on the keyboard. */}
          {keys.map((c, i) => (
            <button key={c} type="button" autoFocus={i === 0} onClick={() => type(c)} className={`${key} aspect-square uppercase`}>
              {c}
            </button>
          ))}
        </div>
      </div>
      {/* Its rows and grids keep only a little room for a focused card to grow. */}
      <div data-focus-region className="min-w-0 flex-1" style={{ "--mavio-layout-margin": "24px" } as CSSProperties}>
        {q === "" ? <Suggestions /> : <Results q={q} results={results} grid />}
      </div>
    </div>
  );
}

/** Home's rows, while nothing is searched on the television. */
function Suggestions() {
  const sections = useHome();
  const several = useSignedIn().length > 1;
  return sections.map((s) => <SectionView key={s.id} section={s} several={several} />);
}

/** The results of each server, in a row each, or a grid on the television. */
function Results({ q, results, grid }: { q: string; results: SearchResults[]; grid: boolean }) {
  const several = useSignedIn().length > 1;
  // The titles the first server that discovers finds, to request.
  const discovery = useDiscovery(discoverLanguage);
  const found = useSearchTitles(discovery.catalogs.length > 0 ? discovery.server : undefined, q, discoverLanguage);
  const titles = titlesOf(found.data?.pages);
  if (q === "") return null;
  const none =
    results.every((r) => !r.pending) && results.every((r) => r.items.length === 0) && !discovery.pending && !found.isLoading && titles.length === 0;
  return (
    <>
      {results.map(({ server, items, pending, error }) => {
        const title = several ? server.name : t("nav.search");
        if (pending) {
          return (
            <Shelf key={server.id} title={title}>
              {Array.from({ length: 6 }, (_, i) => (
                <CardPlaceholder key={i} />
              ))}
            </Shelf>
          );
        }
        if (error) {
          return (
            <p key={server.id} className="flex items-center gap-2 px-margin py-3 type-callout text-text-tertiary">
              <Icon name="offline" size={18} />
              {t("home.serverUnreachable", { server: server.name })}
            </p>
          );
        }
        if (items.length === 0) return null;
        if (grid) {
          return (
            <section key={server.id} className="py-3">
              {several && <h2 className="mb-2 px-margin type-title3">{server.name}</h2>}
              <Grid>
                {items.map((item) => (
                  <ItemCard key={item.id} server={server} serverId={server.id} item={item} fluid />
                ))}
              </Grid>
            </section>
          );
        }
        return (
          <Shelf key={server.id} title={title}>
            {items.map((item) => (
              <ItemCard key={item.id} server={server} serverId={server.id} item={item} />
            ))}
          </Shelf>
        );
      })}
      {discovery.server && titles.length > 0 &&
        (grid ? (
          <section className="py-3">
            <h2 className="mb-2 px-margin type-title3">{t("search.discover")}</h2>
            <Grid>
              {titles.map((title) => (
                <TitleCard key={`${title.mediaKind}/${tmdbOf(title)}`} serverId={discovery.server!.id} title={title} fluid />
              ))}
            </Grid>
          </section>
        ) : (
          <Shelf title={t("search.discover")}>
            {titles.map((title) => (
              <TitleCard key={`${title.mediaKind}/${tmdbOf(title)}`} serverId={discovery.server!.id} title={title} />
            ))}
          </Shelf>
        ))}
      {none && <StatusView icon="search" title={t("search.noResults")} />}
    </>
  );
}
