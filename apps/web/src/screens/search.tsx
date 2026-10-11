// Search asks every server at once and lists the results by server
// (docs/clients.md §5.4).
import { useSearch, useSignedIn } from "@mavio/client-ts";
import { getRouteApi, useNavigate } from "@tanstack/react-router";
import { useEffect, useState } from "react";

import { t } from "../i18n.ts";
import { Icon } from "../icon.tsx";
import { CardPlaceholder, ItemCard, Shelf, StatusView } from "../ui.tsx";

const route = getRouteApi("/search");

export function SearchScreen() {
  const { q = "" } = route.useSearch();
  const navigate = useNavigate();
  const [text, setText] = useState(q);
  // The URL follows the field after a pause, so that results are linkable.
  useEffect(() => {
    const h = setTimeout(() => {
      if (text.trim() !== q) void navigate({ to: "/search", search: { q: text.trim() || undefined }, replace: true });
    }, 300);
    return () => clearTimeout(h);
  }, [text, q, navigate]);
  const results = useSearch(q);
  const several = useSignedIn().length > 1;
  const settled = results.every((r) => !r.pending);
  const none = q !== "" && settled && results.every((r) => r.items.length === 0);
  return (
    <div className="pt-4">
      <div className="px-margin pb-2">
        <label className="flex min-h-12 items-center gap-3 rounded-lg bg-surface-raised px-4">
          <Icon name="search" className="text-text-secondary" />
          <input
            type="search"
            value={text}
            onChange={(e) => setText(e.target.value)}
            placeholder={t("search.placeholder")}
            aria-label={t("nav.search")}
            autoFocus
            className="w-full bg-transparent type-body outline-none placeholder:text-text-tertiary"
          />
        </label>
      </div>
      {results.map(({ server, items, pending, error }) => {
        if (q === "") return null;
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
        return (
          <Shelf key={server.id} title={title}>
            {items.map((item) => (
              <ItemCard key={item.id} server={server} serverId={server.id} item={item} />
            ))}
          </Shelf>
        );
      })}
      {none && <StatusView icon="search" title={t("search.noResults")} />}
    </div>
  );
}
