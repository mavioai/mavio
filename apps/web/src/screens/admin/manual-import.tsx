// Choosing what a file of a manual import is for (docs/clients.md §6.3.17),
// as Radarr's and Sonarr's manual import: a movie of the server's libraries
// found by name, or, for a folder's file, a series found by name and the
// episodes of one of its seasons, those its names hold chosen at first.
import { errorMessage, seasonsOf, type ServerEntry, useDownloadActions, useImportTargets, useManualImport, useSeriesEpisodes } from "@mavio/client-ts";
import type { ImportMode, ManualImportFile } from "@mavio/proto/mavio/acquisition/v1/download_pb";
import { type Item, ItemKind } from "@mavio/proto/mavio/library/v1/item_pb";
import { useEffect, useState } from "react";

import { t } from "../../i18n.ts";
import { Button, Group, Spinner } from "../../ui.tsx";
import { Modal } from "../../controls/dialog.tsx";
import { Checkbox, Segmented } from "../../controls/inputs.tsx";
import { Select } from "../../controls/select.tsx";
import { input } from "../../controls/styles.ts";

type Kind = "movie" | "episodes";

/** The movie or episodes to import a file for, imported once chosen. */
export function ImportTargetDialog({
  server,
  target,
  file,
  mode,
  onImported,
  onClose,
}: {
  server: ServerEntry;
  target: { id?: string; folder?: string };
  file: ManualImportFile;
  mode: ImportMode;
  onImported: () => void;
  onClose: () => void;
}) {
  const { manualImport } = useDownloadActions(server);
  // Episodes are imported from folders; a tracked download is a movie's.
  const folder = !target.id;
  const [kind, setKind] = useState<Kind>("movie");
  const [movieId, setMovieId] = useState(file.movieId);
  const [seriesId, setSeriesId] = useState("");
  const [episodeIds, setEpisodeIds] = useState<string[]>([]);
  const ready = kind === "movie" ? !!movieId : !!seriesId && episodeIds.length > 0;
  const submit = () =>
    manualImport.mutate(
      kind === "movie" ? { ...target, path: file.path, mode, movieId } : { folder: target.folder, path: file.path, mode, seriesId, episodeIds },
      { onSuccess: () => (onImported(), onClose()) },
    );
  return (
    <Modal title={t("downloads.chooseTitle")} onClose={onClose}>
      <p className="type-footnote break-all text-text-secondary">{file.relativePath || file.path}</p>
      {folder && (
        <Segmented
          value={kind}
          label={t("downloads.importFor")}
          options={[
            { value: "movie", label: t("downloads.forMovie") },
            { value: "episodes", label: t("downloads.forEpisodes") },
          ]}
          onChange={setKind}
        />
      )}
      <div className="-mx-6 flex min-h-0 flex-col gap-4 overflow-y-auto px-6 pb-1">
        {kind === "movie" ? (
          <TargetSearch server={server} kind={ItemKind.MOVIE} chosen={movieId} onChoose={setMovieId} />
        ) : (
          <>
            <TargetSearch server={server} kind={ItemKind.SERIES} chosen={seriesId} onChoose={(id) => (setSeriesId(id), setEpisodeIds([]))} />
            {seriesId && <EpisodePicker key={seriesId} server={server} folder={target.folder ?? ""} file={file} seriesId={seriesId} chosen={episodeIds} onChange={setEpisodeIds} />}
          </>
        )}
      </div>
      {manualImport.isError && <p className="type-callout break-words text-danger">{errorMessage(manualImport.error)}</p>}
      <div className="flex justify-end gap-2">
        <Button variant="secondary" onClick={onClose}>
          {t("action.cancel")}
        </Button>
        <Button disabled={!ready || manualImport.isPending} onClick={submit}>
          {manualImport.isPending ? "…" : t("downloads.importFile")}
        </Button>
      </div>
    </Modal>
  );
}

/** The movies or series found by name, one of them chosen. */
function TargetSearch({ server, kind, chosen, onChoose }: { server: ServerEntry; kind: ItemKind.MOVIE | ItemKind.SERIES; chosen: string; onChoose: (id: string) => void }) {
  const [search, setSearch] = useState("");
  const targets = useImportTargets(server, kind, search);
  const label = kind === ItemKind.MOVIE ? t("downloads.searchMovies") : t("downloads.searchSeries");
  const list = targets.data ?? [];
  return (
    <div className="flex flex-col gap-2">
      <input className={input} type="search" aria-label={label} placeholder={label} value={search} onChange={(e) => setSearch(e.target.value)} />
      {targets.isPending && <Spinner size={20} />}
      {targets.isError && <p className="type-footnote break-words text-danger">{errorMessage(targets.error)}</p>}
      {targets.isSuccess && list.length === 0 && <p className="type-callout text-text-secondary">{t("downloads.nothingFound")}</p>}
      {list.length > 0 && (
        <Group>
          {list.map((it) => (
            <TargetRow key={it.id} item={it} chosen={it.id === chosen} onChoose={() => onChoose(it.id)} />
          ))}
        </Group>
      )}
    </div>
  );
}

function TargetRow({ item, chosen, onChoose }: { item: Item; chosen: boolean; onChoose: () => void }) {
  return (
    <button
      type="button"
      aria-pressed={chosen}
      onClick={onChoose}
      className={`flex min-h-11 w-full items-center gap-3 px-4 py-2 text-start ${chosen ? "bg-fill" : "hover:bg-fill"}`}
    >
      <span className="min-w-0 flex-1 truncate type-body">{item.name}</span>
      {item.productionYear > 0 && <span className="type-callout text-text-secondary">{item.productionYear}</span>}
      {chosen && <span className="type-callout font-semibold text-accent">{t("downloads.chosen")}</span>}
    </button>
  );
}

/** A season's episodes of a series, those the file holds chosen at first. */
function EpisodePicker({
  server,
  folder,
  file,
  seriesId,
  chosen,
  onChange,
}: {
  server: ServerEntry;
  folder: string;
  file: ManualImportFile;
  seriesId: string;
  chosen: string[];
  onChange: (ids: string[]) => void;
}) {
  const episodes = useSeriesEpisodes(server, seriesId);
  // What the file's names hold among the series' episodes.
  const listed = useManualImport(server, { folder, seriesId });
  const held = listed.data?.find((f) => f.path === file.path)?.episodeIds;
  const all = episodes.data ?? [];
  const seasons = seasonsOf(all);
  const [season, setSeason] = useState<number | undefined>();
  const [prefilled, setPrefilled] = useState(false);
  useEffect(() => {
    if (prefilled || !held || all.length === 0) return;
    setPrefilled(true);
    const first = all.find((e) => held.includes(e.id));
    if (first) {
      setSeason(first.parentIndexNumber);
      onChange(held.filter((id) => all.some((e) => e.id === id)));
    }
  }, [prefilled, held, all, onChange]);
  const shown = season ?? seasons[0];
  const toggle = (id: string, on: boolean) => onChange(on ? [...chosen, id] : chosen.filter((x) => x !== id));
  if (episodes.isPending) return <Spinner size={20} />;
  if (episodes.isError) return <p className="type-footnote break-words text-danger">{errorMessage(episodes.error)}</p>;
  if (all.length === 0) return <p className="type-callout text-text-secondary">{t("downloads.noEpisodes")}</p>;
  return (
    <div className="flex flex-col gap-2">
      <div className="flex flex-wrap items-center gap-3">
        <span className="min-w-0 flex-1 type-callout text-text-secondary">{t("downloads.chooseEpisodes")}</span>
        <Select
          value={String(shown)}
          label={t("downloads.season")}
          options={seasons.map((n) => ({ value: String(n), label: n === 0 ? t("discover.specials") : t("discover.season", { number: n }) }))}
          onChange={(v) => setSeason(Number(v))}
        />
      </div>
      <Group>
        {all
          .filter((e) => e.parentIndexNumber === shown)
          .map((e) => (
            <div key={e.id} className="px-4 py-2">
              <Checkbox checked={chosen.includes(e.id)} onChange={(on) => toggle(e.id, on)}>
                <span className="min-w-0 flex-1 truncate">{t("downloads.episode", { episode: e.indexNumber, name: e.name })}</span>
              </Checkbox>
            </div>
          ))}
      </Group>
    </div>
  );
}
