#if os(macOS)
    import AppKit
    import MavioMac

    /// The episode and folder templates' defaults, which empty ones use.
    @MainActor private let templateDefaults: [WritableKeyPath<Mavio_Acquisition_V1_NamingSettings, String>: String] = [
        \.standardEpisodeFormat: "{Series Title} - S{season:00}E{episode:00} - {Episode Title} {Quality Full}",
        \.dailyEpisodeFormat: "{Series Title} - {Air-Date} - {Episode Title} {Quality Full}",
        \.animeEpisodeFormat: "{Series Title} - S{season:00}E{episode:00} - {Episode Title} {Quality Full}",
        \.seriesFolderFormat: "{Series Title}",
        \.seasonFolderFormat: "Season {season}",
        \.specialsFolderFormat: "Specials",
    ]

    /// The multi-episode styles, shown for episodes 6 to 8 of season 15.
    private let multiEpisodeStyles: [(String, Mavio_Acquisition_V1_MultiEpisodeStyle)] = [
        ("S15E06-E08", .prefixedRange), ("S15E06-08", .range), ("S15E06-07-08", .extend),
        ("S15E06-E07-E08", .scene), ("S15E06E07E08", .repeat), ("S15E06 - S15E07 - S15E08", .duplicate),
    ]

    /// The items of a list typed separated by commas, trimmed, without the empty ones.
    private func commaList(_ text: String) -> [String] {
        text.split(separator: ",").map { $0.trimmingCharacters(in: .whitespaces) }.filter { !$0.isEmpty }
    }

    /// Acquisition's Options (docs/clients.md §6.3.11), as the web UI's: how
    /// the server names what it imports, how it imports finished downloads,
    /// how often it checks download clients, which releases it takes, and
    /// the recycle bin replaced and deleted files go to; saved whole, or the
    /// changes cancelled.
    final class AcquisitionOptionsPage: FormController {
        private typealias Settings = Mavio_Acquisition_V1_AcquisitionSettings
        private typealias Naming = Mavio_Acquisition_V1_NamingSettings

        private var saved: Settings?
        private var draft = Settings()
        /// The extra files' extensions, as typed.
        private var extensions = ""
        /// The hardcoded subtitles' languages taken, as typed.
        private var whitelisted = ""
        private var failure: String?
        private var saveFailure: String?
        private var notice: String?
        private var saving = false
        private var saveBar: SaveBar?
        private var cancelButton: MavioButton?

        override func viewDidLoad() {
            title = MavioStrings.optionsOptions
            message = MavioStrings.optionsOptionsMessage
            super.viewDidLoad()
            run { [weak self] in await self?.load() }
        }

        override func sections() -> [NSView] {
            guard saved != nil else {
                if let failure { return [card([note(failure, failed: true)])] }
                return [card([spinnerRow()])]
            }
            let cancel = MavioButton(MavioStrings.actionCancel, kind: .secondary, theme: theme) { [weak self] in
                guard let self, let saved = self.saved else { return }
                self.edit(saved)
                self.saveFailure = nil
                self.render()
            }
            cancelButton = cancel
            let bar = SaveBar(MavioStrings.actionSave, theme: theme, extra: [cancel]) { [weak self] in self?.save() }
            saveBar = bar
            refresh()
            return [
                naming(), movieNaming(), episodeNaming(), importing(), downloadClients(), releases(), recycleBin(), bar,
            ]
        }

        /// Starts editing settings: the draft, and its lists as text.
        private func edit(_ settings: Settings) {
            draft = settings
            extensions = settings.extraFileExtensions.joined(separator: ", ")
            whitelisted = settings.whitelistedHardcodedSubs.joined(separator: ", ")
        }

        /// The settings as they would be saved: the lists from the text typed, the paths trimmed.
        private var outgoing: Settings {
            var s = draft
            s.extraFileExtensions = commaList(extensions)
            s.whitelistedHardcodedSubs = commaList(whitelisted)
            s.recycleBin = s.recycleBin.trimmingCharacters(in: .whitespaces)
            s.downloadClientWorkingFolders = s.downloadClientWorkingFolders.trimmingCharacters(in: .whitespaces)
            return s
        }

        private var changed: Bool { saved.map { !acquisitionSame($0, outgoing) } ?? false }

        /// Shows whether saving and cancelling may be pressed, without laying the form out again.
        private func refresh() {
            let changed = changed
            cancelButton?.isHidden = !changed || saving
            saveBar?.update(enabled: changed, busy: saving, notice: changed ? nil : notice, failure: saveFailure)
        }

        // MARK: Rows

        /// A switch of a setting; rows it shows or hides follow it.
        private func toggle(_ title: String, _ field: WritableKeyPath<Settings, Bool>, subtitle: String? = nil)
            -> FormRow
        {
            toggleRow(title, subtitle: subtitle, isOn: draft[keyPath: field]) { [weak self] on in
                self?.draft[keyPath: field] = on
                self?.render()
            }
        }

        /// A switch of a naming setting.
        private func namingToggle(_ title: String, _ field: WritableKeyPath<Naming, Bool>, subtitle: String)
            -> FormRow
        {
            toggleRow(title, subtitle: subtitle, isOn: draft.naming[keyPath: field]) { [weak self] on in
                self?.draft.naming[keyPath: field] = on
                self?.refresh()
            }
        }

        /// A number of a setting, within range.
        private func number(
            _ title: String, _ field: WritableKeyPath<Settings, Int32>, subtitle: String?,
            _ range: ClosedRange<Double> = 0...Double(Int32.max)
        ) -> FormRow {
            numberRow(title, subtitle: subtitle, value: Double(draft[keyPath: field]), range: range) { [weak self] v in
                self?.draft[keyPath: field] = Int32(v.rounded())
                self?.refresh()
            }
        }

        /// A template of a name, its default as the placeholder when it has one.
        private func template(_ title: String, _ field: WritableKeyPath<Naming, String>) -> FormRow {
            fieldRow(
                title, text: draft.naming[keyPath: field], placeholder: templateDefaults[field] ?? "", width: 384
            ) { [weak self] text in
                self?.draft.naming[keyPath: field] = text
                self?.refresh()
            }
        }

        /// A field of text, with an example as its placeholder.
        private func text(
            _ title: String, subtitle: String, _ value: String, placeholder: String,
            onChange: @escaping @MainActor (String) -> Void
        ) -> FormRow {
            fieldRow(title, subtitle: subtitle, text: value, placeholder: placeholder, width: 288) { [weak self] v in
                onChange(v)
                self?.refresh()
            }
        }

        /// A choice of an enumeration, its fallback chosen while it is unspecified.
        private func choice<E: SwiftProtobuf.Enum>(
            _ title: String, subtitle: String? = nil, _ value: E, fallback: E, _ options: [(String, E)],
            onChange: @escaping @MainActor (E) -> Void
        ) -> FormRow {
            choiceRow(title, subtitle: subtitle, options: options, selected: value.rawValue == 0 ? fallback : value) {
                [weak self] v in
                onChange(v)
                self?.refresh()
            }
        }

        // MARK: Groups

        private func naming() -> NSView {
            let colons: [(String, Mavio_Acquisition_V1_ColonReplacement)] = [
                (MavioStrings.optionsColonSmart, .smart), (MavioStrings.optionsColonDelete, .delete),
                (MavioStrings.optionsColonDash, .dash), (MavioStrings.optionsColonSpaceDash, .spaceDash),
                (MavioStrings.optionsColonSpaceDashSpace, .spaceDashSpace),
            ]
            return card(
                MavioStrings.optionsNaming,
                [
                    namingToggle(
                        MavioStrings.optionsReplaceIllegal, \.replaceIllegalCharacters,
                        subtitle: MavioStrings.optionsReplaceIllegalDetail),
                    choice(MavioStrings.optionsColon, draft.naming.colonReplacement, fallback: .smart, colons) {
                        [weak self] in self?.draft.naming.colonReplacement = $0
                    },
                ])
        }

        private func movieNaming() -> NSView {
            card(
                MavioStrings.optionsMovieNaming,
                [
                    namingToggle(
                        MavioStrings.optionsRenameMovies, \.renameMovies,
                        subtitle: MavioStrings.optionsRenameMoviesDetail),
                    template(MavioStrings.optionsMovieFolder, \.movieFolderFormat),
                    template(MavioStrings.optionsMovieFile, \.standardMovieFormat),
                ])
        }

        private func episodeNaming() -> NSView {
            card(
                MavioStrings.optionsEpisodeNaming, footer: MavioStrings.optionsNamingFooter,
                [
                    namingToggle(
                        MavioStrings.optionsRenameEpisodes, \.renameEpisodes,
                        subtitle: MavioStrings.optionsRenameMoviesDetail),
                    template(MavioStrings.optionsStandardEpisode, \.standardEpisodeFormat),
                    template(MavioStrings.optionsDailyEpisode, \.dailyEpisodeFormat),
                    template(MavioStrings.optionsAnimeEpisode, \.animeEpisodeFormat),
                    template(MavioStrings.optionsSeriesFolder, \.seriesFolderFormat),
                    template(MavioStrings.optionsSeasonFolder, \.seasonFolderFormat),
                    template(MavioStrings.optionsSpecialsFolder, \.specialsFolderFormat),
                    choice(
                        MavioStrings.optionsMultiEpisode, draft.naming.multiEpisodeStyle, fallback: .prefixedRange,
                        multiEpisodeStyles
                    ) { [weak self] in self?.draft.naming.multiEpisodeStyle = $0 },
                ])
        }

        private func importing() -> NSView {
            var rows: [NSView] = [
                toggle(
                    MavioStrings.optionsCompletedHandling, \.completedDownloadHandling,
                    subtitle: MavioStrings.optionsCompletedHandlingDetail),
                toggle(
                    MavioStrings.optionsHardlinks, \.copyUsingHardlinks, subtitle: MavioStrings.optionsHardlinksDetail),
                toggle(MavioStrings.optionsExtraFiles, \.importExtraFiles),
            ]
            if draft.importExtraFiles {
                rows.append(
                    text(
                        MavioStrings.optionsExtensions, subtitle: MavioStrings.optionsExtensionsDetail, extensions,
                        placeholder: "srt"
                    ) { [weak self] in self?.extensions = $0 })
            }
            rows.append(toggle(MavioStrings.optionsDeleteEmpty, \.deleteEmptyFolders))
            rows.append(toggle(MavioStrings.optionsSkipFreeSpace, \.skipFreeSpaceCheck))
            if !draft.skipFreeSpaceCheck {
                rows.append(
                    number(
                        MavioStrings.optionsMinimumFreeSpace, \.minimumFreeSpace,
                        subtitle: MavioStrings.optionsMinimumFreeSpaceDetail))
            }
            let titles: [(String, Mavio_Acquisition_V1_EpisodeTitleRequired)] = [
                (MavioStrings.optionsTitleAlways, .always), (MavioStrings.optionsTitleBulk, .bulkSeasonReleases),
                (MavioStrings.optionsTitleNever, .never),
            ]
            rows.append(
                choice(
                    MavioStrings.optionsEpisodeTitle, subtitle: MavioStrings.optionsEpisodeTitleDetail,
                    draft.episodeTitleRequired, fallback: .always, titles
                ) { [weak self] in self?.draft.episodeTitleRequired = $0 })
            rows.append(
                toggle(
                    MavioStrings.optionsUnmonitorMissing, \.autoUnmonitorPreviouslyDownloadedEpisodes,
                    subtitle: MavioStrings.optionsUnmonitorMissingDetail))
            rows.append(
                text(
                    MavioStrings.optionsWorkingFolders, subtitle: MavioStrings.optionsWorkingFoldersDetail,
                    draft.downloadClientWorkingFolders, placeholder: "_UNPACK_|_FAILED_"
                ) { [weak self] in self?.draft.downloadClientWorkingFolders = $0 })
            return card(MavioStrings.optionsImporting, rows)
        }

        private func downloadClients() -> NSView {
            var rows: [NSView] = [
                number(
                    MavioStrings.optionsCheckInterval, \.checkFinishedInterval,
                    subtitle: MavioStrings.optionsCheckIntervalDetail, 1...120),
                toggle(MavioStrings.optionsRedownload, \.autoRedownloadFailed),
            ]
            if draft.autoRedownloadFailed {
                rows.append(
                    toggle(MavioStrings.optionsRedownloadInteractive, \.autoRedownloadFailedFromInteractiveSearch))
            }
            return card(MavioStrings.optionsDownloadClients, rows)
        }

        private func releases() -> NSView {
            let propers: [(String, Mavio_Acquisition_V1_ProperDownload)] = [
                (MavioStrings.optionsPropersUpgrade, .preferAndUpgrade),
                (MavioStrings.optionsPropersNoUpgrade, .doNotUpgrade),
                (MavioStrings.optionsPropersNoPrefer, .doNotPrefer),
            ]
            var rows: [NSView] = [
                choice(MavioStrings.optionsPropers, draft.propers, fallback: .preferAndUpgrade, propers) {
                    [weak self] in self?.draft.propers = $0
                },
                toggle(MavioStrings.optionsIndexerFlags, \.preferIndexerFlags),
                number(
                    MavioStrings.optionsRssInterval, \.rssSyncInterval, subtitle: MavioStrings.optionsRssIntervalDetail,
                    0...120),
                number(
                    MavioStrings.optionsMaximumSize, \.maximumSize, subtitle: MavioStrings.optionsMaximumSizeDetail),
                number(MavioStrings.optionsMinimumAge, \.minimumAge, subtitle: MavioStrings.optionsMinimumAgeDetail),
                number(MavioStrings.optionsRetention, \.retention, subtitle: MavioStrings.optionsRetentionDetail),
                number(
                    MavioStrings.optionsAvailabilityDelay, \.availabilityDelay,
                    subtitle: MavioStrings.optionsAvailabilityDelayDetail, -365...365),
                toggle(MavioStrings.optionsHardcodedSubs, \.allowHardcodedSubs),
            ]
            if !draft.allowHardcodedSubs {
                rows.append(
                    text(
                        MavioStrings.optionsWhitelistedSubs, subtitle: MavioStrings.optionsWhitelistedSubsDetail,
                        whitelisted, placeholder: "kor, chi"
                    ) { [weak self] in self?.whitelisted = $0 })
            }
            return card(MavioStrings.optionsReleasesGroup, rows)
        }

        private func recycleBin() -> NSView {
            // Whether the cleanup shows follows the bin as it is typed.
            let hadBin = !draft.recycleBin.trimmingCharacters(in: .whitespaces).isEmpty
            var rows: [NSView] = [
                text(
                    MavioStrings.optionsRecycleBin, subtitle: MavioStrings.optionsRecycleBinDetail, draft.recycleBin,
                    placeholder: "/data/recycle"
                ) { [weak self] in
                    guard let self else { return }
                    self.draft.recycleBin = $0
                    let hasBin = !$0.trimmingCharacters(in: .whitespaces).isEmpty
                    if hasBin != hadBin { DispatchQueue.main.async { [weak self] in self?.renderKeepingField() } }
                }
            ]
            if hadBin {
                rows.append(
                    number(
                        MavioStrings.optionsCleanupDays, \.recycleBinCleanupDays,
                        subtitle: MavioStrings.optionsCleanupDaysDetail))
            }
            return card(MavioStrings.optionsRecycleBin, rows)
        }

        /// Draws the page again while the recycle bin's field is typed in,
        /// putting the caret back in it.
        private func renderKeepingField() {
            render()
            let label = MavioStrings.optionsRecycleBin
            guard let input = findInput(in: view, label: label) else { return }
            input.activate()
            input.field.currentEditor()?.selectedRange = NSRange(location: input.text.utf16.count, length: 0)
        }

        /// The text input labelled label under view.
        private func findInput(in view: NSView, label: String) -> TextInput? {
            if let input = view as? TextInput, input.field.accessibilityLabel() == label { return input }
            for sub in view.subviews {
                if let found = findInput(in: sub, label: label) { return found }
            }
            return nil
        }

        // MARK: Loading and saving

        private func load() async {
            do {
                let r = try await model.call(
                    server, Procedure.profiles("GetAcquisitionSettings"),
                    Mavio_Acquisition_V1_GetAcquisitionSettingsRequest(),
                    as: Mavio_Acquisition_V1_GetAcquisitionSettingsResponse.self)
                saved = r.settings
                edit(r.settings)
                failure = nil
            } catch is CancellationError {
                return
            } catch {
                failure = error.reason
            }
            render()
        }

        private func save() {
            endEditing()
            var request = Mavio_Acquisition_V1_UpdateAcquisitionSettingsRequest()
            request.settings = outgoing
            saving = true
            saveFailure = nil
            refresh()
            run { [weak self, model, server] in
                do {
                    let r = try await model.call(
                        server, Procedure.profiles("UpdateAcquisitionSettings"), request,
                        as: Mavio_Acquisition_V1_UpdateAcquisitionSettingsResponse.self)
                    guard let self else { return }
                    self.saved = r.settings
                    self.edit(r.settings)
                    self.notice = MavioStrings.acquisitionSaved
                    self.saving = false
                    self.render()
                } catch {
                    self?.saving = false
                    self?.saveFailure = error.reason
                    self?.refresh()
                }
            }
        }
    }
#endif
