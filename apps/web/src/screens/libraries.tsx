// The libraries of every server, grouped by server in the user's order.
import { libraryImageUrl, useLibraries, useSignedIn } from "@mavio/client-ts";
import { Link } from "@tanstack/react-router";

import { t } from "../i18n.ts";
import { Icon } from "../icon.tsx";
import { Artwork, ScreenTitle } from "../ui.tsx";

export function LibrariesScreen() {
  const all = useLibraries();
  const several = useSignedIn().length > 1;
  return (
    <div>
      <ScreenTitle>{t("nav.libraries")}</ScreenTitle>
      {all.map(({ server, libraries, pending, error }) => (
        <section key={server.id} className="py-3">
          {several && <h2 className="px-margin pb-2 type-title3">{server.name}</h2>}
          {error && (
            <p className="flex items-center gap-2 px-margin type-callout text-text-tertiary">
              <Icon name="offline" size={18} />
              {t("home.serverUnreachable", { server: server.name })}
            </p>
          )}
          <div
            className="grid gap-gutter px-margin"
            style={{ gridTemplateColumns: "repeat(auto-fill, minmax(var(--mavio-layout-thumbnail), 1fr))" }}
          >
            {pending &&
              Array.from({ length: 3 }, (_, i) => <div key={i} className="aspect-video rounded-md bg-surface-raised" />)}
            {libraries.map((l) => (
              <Link
                key={l.id}
                to="/s/$serverId/library/$libraryId"
                params={{ serverId: server.id, libraryId: l.id }}
                className="group block"
              >
                <Artwork
                  src={libraryImageUrl(server, l, 480)}
                  alt={l.name}
                  className="aspect-video rounded-md transition group-hover:brightness-110"
                />
                <div className="mt-2 type-headline">{l.name}</div>
              </Link>
            ))}
          </div>
        </section>
      ))}
    </div>
  );
}
