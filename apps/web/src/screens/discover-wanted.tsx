// Adding a discovered title to the wanted (docs/clients.md §6.3.2), for
// administrators, as Radarr's and Sonarr's Add New: a movie by its TMDB ID
// or a series by its TVDB ID, to a library of its kind with a quality
// profile, a movie's minimum availability or a series' monitoring, type,
// new seasons and season folders, and whether to search for it at once.
import { errorMessage, type ServerEntry, useAddWanted, useQualityProfiles, useRequestSettings, useServerLibraries, wantedTarget } from "@mavio/client-ts";
import type { Title } from "@mavio/proto/mavio/acquisition/v1/discover_pb";
import { MediaKind } from "@mavio/proto/mavio/acquisition/v1/profile_pb";
import { MinimumAvailability, NewItemMonitor, SeriesMonitor } from "@mavio/proto/mavio/acquisition/v1/wanted_pb";
import { SeriesType } from "@mavio/proto/mavio/library/v1/item_pb";
import { LibraryKind } from "@mavio/proto/mavio/library/v1/library_pb";
import { useState } from "react";

import { t } from "../i18n.ts";
import { Button, Group, Loading, Row } from "../ui.tsx";
import { Modal } from "../controls/dialog.tsx";
import { Switch } from "../controls/inputs.tsx";
import { Select } from "../controls/select.tsx";

/** The choices of the dialog, once the server's settings, libraries and profiles have come. */
export function AddWantedDialog({ server, title, onClose }: { server: ServerEntry; title: Title; onClose: () => void }) {
  const settings = useRequestSettings(server).get;
  const libraries = useServerLibraries(server);
  const profiles = useQualityProfiles(server).list;
  const ready = (settings.isSuccess || settings.isError) && libraries.isSuccess && profiles.isSuccess;
  return (
    <Modal title={t("discover.addWantedTitle", { name: title.name })} onClose={onClose}>
      {ready ? (
        <AddWantedForm
          server={server}
          title={title}
          settings={settings.data}
          libraries={libraries.data}
          profiles={profiles.data}
          onClose={onClose}
        />
      ) : libraries.isError || profiles.isError ? (
        <p className="type-callout break-words text-danger">{errorMessage(libraries.error ?? profiles.error)}</p>
      ) : (
        <Loading />
      )}
    </Modal>
  );
}

type Settings = ReturnType<typeof useRequestSettings>["get"]["data"];
type Libraries = NonNullable<ReturnType<typeof useServerLibraries>["data"]>;
type Profiles = NonNullable<ReturnType<typeof useQualityProfiles>["list"]["data"]>;

function AddWantedForm({
  server,
  title,
  settings,
  libraries,
  profiles,
  onClose,
}: {
  server: ServerEntry;
  title: Title;
  settings: Settings;
  libraries: Libraries;
  profiles: Profiles;
  onClose: () => void;
}) {
  const add = useAddWanted(server);
  const series = title.mediaKind === MediaKind.SERIES;
  const [target, setTarget] = useState(() => wantedTarget(title.mediaKind, title.anime, settings, libraries, profiles));
  const [availability, setAvailability] = useState(settings?.minimumAvailability || MinimumAvailability.RELEASED);
  const [monitor, setMonitor] = useState(SeriesMonitor.ALL);
  const [type, setType] = useState(title.anime ? SeriesType.ANIME : SeriesType.UNSPECIFIED);
  const [newItems, setNewItems] = useState(settings?.monitorNewItems || NewItemMonitor.ALL);
  const [seasonFolder, setSeasonFolder] = useState(settings?.seasonFolder ?? true);
  const [search, setSearch] = useState(true);
  const mutation = series ? add.series : add.movie;

  const libraryOptions = libraries.filter((l) => l.kind === (series ? LibraryKind.SHOWS : LibraryKind.MOVIES)).map((l) => ({ value: l.id, label: l.name }));
  const profileOptions = profiles.filter((p) => p.kind === title.mediaKind).map((p) => ({ value: p.id, label: p.name }));
  const missing =
    libraryOptions.length === 0
      ? t("discover.addWantedNoLibrary")
      : profileOptions.length === 0
        ? t("discover.addWantedNoProfile")
        : series && title.tvdbId <= 0
          ? t("discover.addWantedNoTvdb")
          : undefined;

  // The title's page then shows it wanted, with Watch opening its item.
  const opened = () => onClose();
  const submit = () =>
    series
      ? add.series.mutate(
          {
            libraryId: target.libraryId,
            tvdbId: title.tvdbId,
            qualityProfileId: target.qualityProfileId,
            seriesType: type,
            seasonFolder,
            monitor,
            monitorNewItems: newItems,
            search,
          },
          { onSuccess: opened },
        )
      : add.movie.mutate(
          { libraryId: target.libraryId, tmdbId: title.tmdbId, qualityProfileId: target.qualityProfileId, minimumAvailability: availability, search },
          { onSuccess: opened },
        );

  const choice = <T extends number>(label: string, value: T, options: [T, string][], onChange: (v: T) => void, detail?: string) => (
    <Row label={label} detail={detail}>
      <Select value={String(value)} label={label} className="w-56" options={options.map(([v, l]) => ({ value: String(v), label: l }))} onChange={(v) => onChange(Number(v) as T)} />
    </Row>
  );
  const monitors: [SeriesMonitor, string][] = [
    [SeriesMonitor.ALL, t("discover.monitorAll")],
    [SeriesMonitor.FUTURE, t("discover.monitorFuture")],
    [SeriesMonitor.MISSING, t("discover.monitorMissing")],
    [SeriesMonitor.EXISTING, t("discover.monitorExisting")],
    [SeriesMonitor.RECENT, t("discover.monitorRecent")],
    [SeriesMonitor.PILOT, t("discover.monitorPilot")],
    [SeriesMonitor.FIRST_SEASON, t("discover.monitorFirstSeason")],
    [SeriesMonitor.LAST_SEASON, t("discover.monitorLastSeason")],
    [SeriesMonitor.NONE, t("discover.monitorNone")],
  ];
  return (
    <div className="-mx-6 flex min-h-0 flex-col gap-4 overflow-y-auto px-6 pb-1">
      {missing ? (
        <p className="type-callout text-text-secondary">{missing}</p>
      ) : (
        <Group footer={series ? t("discover.addWantedSeriesFooter") : t("discover.addWantedMovieFooter")}>
          <Row label={t("requestsettings.library")}>
            <Select value={target.libraryId} label={t("requestsettings.library")} className="w-56" options={libraryOptions} onChange={(libraryId) => setTarget((x) => ({ ...x, libraryId }))} />
          </Row>
          <Row label={t("requestsettings.profile")}>
            <Select
              value={target.qualityProfileId}
              label={t("requestsettings.profile")}
              className="w-56"
              options={profileOptions}
              onChange={(qualityProfileId) => setTarget((x) => ({ ...x, qualityProfileId }))}
            />
          </Row>
          {series ? (
            <>
              {choice(t("discover.monitor"), monitor, monitors, setMonitor)}
              {choice(
                t("requestsettings.monitorNew"),
                newItems,
                [
                  [NewItemMonitor.ALL, t("requestsettings.monitorAll")],
                  [NewItemMonitor.NONE, t("requestsettings.monitorNone")],
                ],
                setNewItems,
                t("requestsettings.monitorNewDetail"),
              )}
              {choice(
                t("discover.seriesType"),
                type,
                [
                  [SeriesType.UNSPECIFIED, t("discover.seriesTypeAuto")],
                  [SeriesType.STANDARD, t("discover.seriesTypeStandard")],
                  [SeriesType.DAILY, t("discover.seriesTypeDaily")],
                  [SeriesType.ANIME, t("discover.seriesTypeAnime")],
                ],
                setType,
              )}
              <Row label={t("requestsettings.seasonFolder")} detail={t("requestsettings.seasonFolderDetail")}>
                <Switch on={seasonFolder} label={t("requestsettings.seasonFolder")} onChange={setSeasonFolder} />
              </Row>
            </>
          ) : (
            choice(
              t("requestsettings.minimumAvailability"),
              availability,
              [
                [MinimumAvailability.ANNOUNCED, t("requestsettings.announced")],
                [MinimumAvailability.IN_CINEMAS, t("requestsettings.inCinemas")],
                [MinimumAvailability.RELEASED, t("requestsettings.released")],
              ],
              setAvailability,
            )
          )}
          <Row label={t("discover.searchNow")} detail={series ? t("discover.searchNowSeriesDetail") : t("discover.searchNowMovieDetail")} wrap>
            <Switch on={search} label={t("discover.searchNow")} onChange={setSearch} />
          </Row>
        </Group>
      )}
      {mutation.isError && <p className="type-callout break-words text-danger">{t("discover.addWantedFailed", { reason: errorMessage(mutation.error) })}</p>}
      <div className="flex justify-end gap-2">
        <Button variant="secondary" onClick={onClose}>
          {t("action.cancel")}
        </Button>
        <Button disabled={!!missing || mutation.isPending || !target.libraryId || !target.qualityProfileId} onClick={submit}>
          {mutation.isPending ? "…" : t("discover.addWantedAdd")}
        </Button>
      </div>
    </div>
  );
}
