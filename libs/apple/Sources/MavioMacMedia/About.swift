#if os(macOS)
    import AppKit
    import MavioMac

    /// An item's Details section (libs/design/components.md, Details
    /// Screen): the tagline and overview, the leading crew, the tags and
    /// links, then the ratings, the facts and the chosen version's media.
    /// Genres, keywords and studios open their items in the library, and the
    /// crew their pages. A movie or series adds what the metadata providers
    /// tell of it beyond what the library keeps, as Seerr shows: its
    /// ratings, budget and takings, its next episode, the services streaming
    /// it and the logos linking it elsewhere. In television mode its two
    /// columns are regions of their own, and nothing links to the web.
    final class AboutSection: NSView {
        private weak var page: Page?
        private let details: Details
        private let source: Source?
        private var allStudios = false
        /// The movie or series as the server discovers it.
        private var title: TitleInfo?
        /// Its ratings elsewhere; nil until asked.
        private var titleRatings: [Rating]?
        private var task: Task<Void, Never>?

        private static let minStudios = 3

        /// The section of details, its version source.
        init(page: Page, details: Details, source: Source?) {
            self.page = page
            self.details = details
            self.source = source
            super.init(frame: .zero)
            translatesAutoresizingMaskIntoConstraints = false
            build()
            loadTitle()
        }

        @available(*, unavailable)
        required init?(coder: NSCoder) { fatalError() }

        private var theme: Theme { page?.theme ?? .desktop }
        private var card: Card { details.card }
        private var mediaKind: MediaKind? { card.kind == .movie ? .movie : card.kind == .series ? .series : nil }
        /// A movie's or series' TMDB ID, which discovery knows it by; 0 otherwise.
        private var tmdbId: Int32 { mediaKind == nil ? 0 : Int32(details.externalIds["tmdb"] ?? "") ?? 0 }
        private var titled: Bool { tmdbId > 0 }
        private var inSeries: Bool { card.kind == .series || card.kind == .season || card.kind == .episode }
        private var studioKey: String { inSeries ? MavioStrings.itemNetworks : MavioStrings.itemStudio }

        /// Up to six of the crew, the creators, directors and writers first.
        private var crew: [CreditEntry] {
            let rank = { (c: CreditEntry) in CreditJob.crewOrder.firstIndex(of: c.job) ?? .max }
            return Array(
                details.credits.enumerated().filter { !$0.element.job.isCast }
                    .sorted { (rank($0.element), $0.offset) < (rank($1.element), $1.offset) }
                    .map(\.element)
                    .prefix(6))
        }

        private var tags: [String] { card.kind == .episode || card.kind == .season ? [] : details.tags }

        /// A genre, keyword or studio opening the library's items of it, when
        /// the server is reachable.
        private func narrowed(genre: String? = nil, tag: String? = nil, studio: String? = nil) -> Route? {
            guard let libraryId = details.libraryId, !details.offline else { return nil }
            return .filteredLibrary(
                serverId: card.serverId, libraryId: libraryId,
                filter: Filter(played: nil, favorite: false, genre: genre, tag: tag, studio: studio))
        }

        /// The values of a fact that open the library, by the fact's caption.
        private var links: [String: [(String, Route?)]] {
            var links: [String: [(String, Route?)]] = [:]
            links[MavioStrings.itemGenres] = details.genres.map { ($0, narrowed(genre: $0)) }
            links[studioKey] = shownStudios.map { ($0, narrowed(studio: $0)) }
            return links
        }

        private var facts: [(String, String)] {
            let day = { (d: Date?) in d?.formatted(date: .abbreviated, time: .omitted) }
            let unbroken = { (s: String) in s.replacingOccurrences(of: " ", with: "\u{00A0}") }
            var rows: [(String, String?)] = []
            // A movie's releases on one row, as on its discovered title, what
            // the library keeps first.
            let releases =
                card.kind == .movie
                ? [
                    (MavioStrings.discoverTheatricalRelease, details.premiere, title?.theatricalReleaseDate),
                    (MavioStrings.itemDigitalRelease, details.digitalRelease, title?.digitalReleaseDate),
                    (MavioStrings.itemPhysicalRelease, details.physicalRelease, title?.physicalReleaseDate),
                ].compactMap { label, kept, told in
                    (day(kept) ?? dayText(told ?? "")).map { "\(label)  \(unbroken($0))" }
                }
                : []
            if !releases.isEmpty {
                rows.append(
                    (
                        releases.count > 1 ? MavioStrings.discoverReleaseDates : MavioStrings.discoverReleaseDate,
                        releases.joined(separator: "\n")
                    ))
            } else {
                rows.append(
                    (
                        card.kind == .series
                            ? MavioStrings.itemFirstAired
                            : card.kind == .episode ? MavioStrings.itemAired : MavioStrings.itemReleased,
                        day(details.premiere)
                    ))
            }
            rows.append(
                (
                    MavioStrings.itemStatus,
                    card.kind == .series && details.seriesStatus == .continuing
                        ? MavioStrings.itemContinuing
                        : card.kind == .series && details.seriesStatus == .ended
                            ? MavioStrings.itemEnded : title?.productionStatus.text
                ))
            if card.kind == .series {
                rows.append((MavioStrings.itemLastAired, details.seriesStatus == .ended ? day(details.endDate) : nil))
                rows.append((MavioStrings.itemNextAirDate, dayText(title?.nextAirDate ?? "")))
            }
            rows.append((MavioStrings.itemRuntime, card.kind == .series ? nil : card.runtime.map(minutesText)))
            rows.append((MavioStrings.itemRated, details.officialRating))
            rows.append(
                (MavioStrings.itemGenres, details.genres.isEmpty ? nil : details.genres.joined(separator: ", ")))
            rows.append((studioKey, shownStudios.isEmpty ? nil : shownStudios.joined(separator: ", ")))
            rows.append((MavioStrings.itemCountry, countries.isEmpty ? nil : countries.joined(separator: "\n")))
            let language =
                details.originalLanguage ?? title.flatMap { $0.originalLanguage.isEmpty ? nil : $0.originalLanguage }
            rows.append((MavioStrings.itemOriginalLanguage, language.map(languageText)))
            rows.append(
                (MavioStrings.itemOriginalTitle, details.originalTitle.flatMap { $0 == card.title ? nil : $0 }))
            rows.append((MavioStrings.itemBudget, title.flatMap { moneyText($0.budget) }))
            rows.append((MavioStrings.itemRevenue, title.flatMap { moneyText($0.revenue) }))
            rows.append((MavioStrings.itemDateAdded, day(details.dateAdded)))
            return rows.compactMap { k, v in v.map { (k, $0) } }
        }

        /// The countries with their flags when the library keeps those the
        /// metadata providers tell, else as the library keeps them.
        private var countries: [String] {
            let stored = details.countries
            let told = title?.productionLocations ?? []
            if let title, !title.productionCountries.isEmpty,
                stored.isEmpty || (stored.count == told.count && stored.allSatisfy(told.contains))
            {
                return title.productionCountries.map(countryText)
            }
            return stored.isEmpty ? told : stored
        }

        private var shownStudios: [String] {
            allStudios ? details.studios : Array(details.studios.prefix(Self.minStudios))
        }

        private var media: [(String, String)] {
            guard let source else { return [] }
            let video = source.video.map { v in
                [source.resolution, v.codec.uppercased(), source.range].compactMap(\.self).joined(separator: " · ")
            }
            let file = [source.fileName, source.sizeText].compactMap(\.self).joined(separator: "\n")
            let rows: [(String, String?)] = [
                (MavioStrings.itemVideo, video),
                (
                    MavioStrings.itemAudioTracks,
                    source.audio.isEmpty ? nil : source.audio.map(audioText).joined(separator: "\n")
                ),
                (
                    MavioStrings.itemSubtitleTracks,
                    source.subtitles.isEmpty ? nil : source.subtitles.map(subtitleText).joined(separator: "\n")
                ),
                (MavioStrings.itemFile, file.isEmpty ? nil : file),
            ]
            return rows.compactMap { k, v in v.map { (k, $0) } }
        }

        private var ratings: Bool { (details.communityRating ?? 0) > 0 || (details.criticRating ?? 0) > 0 }

        /// The pages about it as the library tells, for what is not titled,
        /// but those of a site with a logo; none in television mode, which
        /// opens no web pages.
        private var pages: [ExternalLink] {
            titled || theme.isTV ? [] : details.links.filter { siteLink(name: $0.name, url: $0.url) == nil }
        }

        /// The pages of sites with a logo, shown by it as a discovered
        /// title's links are.
        private var sitePages: [TitleLink] {
            titled || theme.isTV ? [] : details.links.compactMap { siteLink(name: $0.name, url: $0.url) }
        }

        // MARK: Building

        private func build() {
            subviews.forEach { $0.removeFromSuperview() }
            guard let page else { return }
            let theme = theme
            let overview = details.overview
            let empty =
                overview == nil && !ratings && !titled && crew.isEmpty && tags.isEmpty && facts.isEmpty
                && media.isEmpty && pages.isEmpty && sitePages.isEmpty
            guard !empty else { return }
            let hasLeading =
                overview != nil || !crew.isEmpty || !tags.isEmpty || !pages.isEmpty || !sitePages.isEmpty
                || (titled && !theme.isTV)
            var trailingViews: [NSView] = []
            if let ratings = ratingsView() { trailingViews.append(ratings) }
            if let facts = factGrid(facts, links: links, page: page) { trailingViews.append(facts) }
            if let providers = watchProviders() { trailingViews.append(providers) }
            if let media = factGrid(media, links: [:], page: page) { trailingViews.append(media) }
            let trailing = RegionColumn(trailingViews, spacing: 32)
            var columns: [NSView] = []
            if hasLeading {
                let leading = RegionColumn(leadingViews(page), spacing: MavioSpacing.s20)
                leading.widthAnchor.constraint(lessThanOrEqualToConstant: 440).isActive = true
                let preferred = leading.widthAnchor.constraint(equalToConstant: 440)
                preferred.priority = .defaultHigh
                preferred.isActive = true
                columns.append(leading)
            }
            columns.append(trailing)
            let row = NSStackView.horizontal(columns, spacing: 64, alignment: .top)
            row.distribution = .fill
            trailing.setContentHuggingPriority(.defaultLow, for: .horizontal)
            let stack = NSStackView.vertical([page.heading(MavioStrings.itemDetails), row], spacing: MavioSpacing.s16)
            row.widthAnchor.constraint(equalTo: stack.widthAnchor).isActive = true
            addFilling(
                stack,
                insets: NSEdgeInsets(
                    top: MavioSpacing.s32, left: theme.layout.margin, bottom: MavioSpacing.s16,
                    right: theme.layout.margin))
            FocusEngine.active(for: self)?.schedule()
        }

        private func leadingViews(_ page: Page) -> [NSView] {
            let theme = theme
            var views: [NSView] = []
            if let overview = details.overview {
                var text: [NSView] = []
                if let tagline = details.tagline {
                    let field = NSTextField.label(
                        tagline, theme.fonts.headline, color: theme.colors.textPrimary, lines: 0)
                    field.preferredMaxLayoutWidth = 440
                    text.append(field)
                }
                let field = NSTextField.label(
                    overview, theme.fonts.body, color: theme.colors.textSecondary, lines: 0, selectable: !theme.isTV)
                field.preferredMaxLayoutWidth = 440
                text.append(field)
                views.append(NSStackView.vertical(text, spacing: MavioSpacing.s8))
            }
            let crew = crew
            if !crew.isEmpty {
                let grid = FlowGrid(minimum: 160, gutter: MavioSpacing.s24, rowSpacing: MavioSpacing.s16)
                grid.setItems(
                    crew.map { c in
                        let job = NSTextField.label(
                            c.job == .other ? c.role ?? c.job.label : c.job.label, theme.fonts.headline,
                            color: theme.colors.textPrimary)
                        let name: NSView =
                            c.personId.isEmpty
                            ? NSTextField.label(c.name, theme.fonts.callout, color: theme.colors.textSecondary)
                            : page.textLink(c.name, theme.fonts.callout, color: theme.colors.textSecondary) {
                                [weak page, serverId = card.serverId] in
                                page?.open(.person(serverId: serverId, personId: c.personId))
                            }
                        return NSStackView.vertical([job, name], spacing: 0)
                    })
                grid.widthAnchor.constraint(greaterThanOrEqualToConstant: 340).isActive = true
                views.append(grid)
                let route = Route.credits(serverId: card.serverId, itemId: card.itemId, crew: true)
                let full = NSStackView.horizontal(
                    [
                        label(
                            MavioStrings.discoverFullCrew, theme.fonts.callout, weight: .semibold,
                            color: theme.colors.textPrimary),
                        IconView("forward", size: 16, color: theme.colors.textPrimary),
                    ], spacing: MavioSpacing.s4)
                let link = Pressable(full, look: .highlight, radius: MavioRadius.sm, theme: theme) { [weak page] in
                    page?.open(route)
                }
                link.setAccessibilityLabel(MavioStrings.discoverFullCrew)
                views.append(link)
            }
            if !tags.isEmpty {
                let pills = tags.map { tag -> NSView in
                    var action: (@MainActor () -> Void)?
                    if let route = narrowed(tag: tag) { action = { [weak page] in page?.open(route) } }
                    return page.chip(
                        tag, weight: .regular, border: theme.colors.border,
                        insets: .symmetric(horizontal: MavioSpacing.s12, vertical: MavioSpacing.s4), action: action)
                }
                let wrap = WrapStack(pills, spacing: MavioSpacing.s8, lineSpacing: MavioSpacing.s8)
                let caption = label(
                    MavioStrings.itemTags, theme.fonts.footnote, weight: .semibold, color: theme.colors.textSecondary)
                let stack = NSStackView.vertical([caption, wrap], spacing: MavioSpacing.s8)
                wrap.widthAnchor.constraint(equalTo: stack.widthAnchor).isActive = true
                views.append(stack)
            }
            let links =
                if let kind = mediaKind, titled, !theme.isTV {
                    titleLinks(
                        kind: kind, tmdbId: tmdbId, imdb: details.externalIds["imdb"] ?? "",
                        tvdb: details.externalIds["tvdb"] ?? "", ratings: titleRatings ?? [])
                } else {
                    sitePages
                }
            if !links.isEmpty {
                let logos = links.map { link -> NSView in
                    let logo = serviceLogo(link.logo, height: 24, width: link.width, tint: theme.colors.textPrimary)
                    let button = Pressable(logo, look: .highlight, radius: MavioRadius.sm, theme: theme) {
                        openWeb(link.url)
                    }
                    button.toolTip = link.name
                    button.setAccessibilityLabel(link.name)
                    return button
                }
                let wrap = WrapStack(logos, spacing: MavioSpacing.s20, lineSpacing: MavioSpacing.s12)
                wrap.alphaValue = 0.5
                views.append(inset(wrap, NSEdgeInsets(top: MavioSpacing.s8, left: 0, bottom: 0, right: 0)))
            }
            let pages = pages
            if !pages.isEmpty {
                let chips = pages.compactMap { link -> NSView? in
                    guard let url = URL(string: link.url) else { return nil }
                    return page.chip(link.name, icon: "external", fill: theme.colors.fill) { openWeb(url) }
                }
                views.append(WrapStack(chips, spacing: MavioSpacing.s8, lineSpacing: MavioSpacing.s8))
            }
            return views
        }

        /// A movie's or series' ratings elsewhere once known, else the
        /// library's.
        private func ratingsView() -> NSView? {
            if let kind = mediaKind, titled {
                let score = title.flatMap { $0.voteCount > 0 ? $0.communityRating : nil }
                guard let loaded = titleRatings else { return nil }
                if !shownRatings(loaded).isEmpty || score != nil {
                    return ratingMarksView(kind: kind, ratings: loaded, score: score)
                }
                return ratings ? ratingCards() : nil
            }
            return ratings ? ratingCards() : nil
        }

        /// Rotten Tomatoes' and IMDb's ratings, then TMDB's score when given,
        /// each by its logo and opening its page outside television mode.
        private func ratingMarksView(kind: MediaKind, ratings: [Rating], score: Double?) -> NSView {
            let theme = theme
            let marks = ratingMarks(kind: kind, tmdbId: tmdbId, ratings: ratings, score: score).map { m -> NSView in
                let row = NSStackView.horizontal(
                    [
                        serviceLogo(m.logo, height: m.height),
                        label(m.value, theme.fonts.callout, weight: .semibold, color: theme.colors.textPrimary),
                    ], spacing: 6)
                if let url = m.url, !theme.isTV {
                    let button = Pressable(row, look: .highlight, radius: MavioRadius.sm, theme: theme) { openWeb(url) }
                    button.setAccessibilityLabel("\(m.label) \(m.value)")
                    return button
                }
                row.setAccessibilityElement(true)
                row.setAccessibilityRole(.staticText)
                row.setAccessibilityLabel("\(m.label) \(m.value)")
                return row
            }
            return WrapStack(marks, spacing: MavioSpacing.s16, lineSpacing: MavioSpacing.s4)
        }

        /// The audience's rating out of 10 and the critics' score out of
        /// 100, in a row as a discovered title's ratings are, each named
        /// when pointed at.
        private func ratingCards() -> NSView {
            let theme = theme
            var marks: [(String, [NSView])] = []
            if let rating = details.communityRating, rating > 0 {
                let value = rating.formatted(.number.precision(.fractionLength(1)))
                let star = IconView(
                    "rating", fill: true, size: 20, color: NSColor(red: 1, green: 0.84, blue: 0.04, alpha: 1))
                marks.append(("\(MavioStrings.itemCommunityRating) \(value) / 10", [star, ratingText(value)]))
            }
            if let rating = details.criticRating, rating > 0 {
                let text = "\(MavioStrings.itemCriticRating) \(Int(rating.rounded()))%"
                marks.append((text, [ratingText(text)]))
            }
            let views = marks.map { name, views -> NSView in
                let row = NSStackView.horizontal(views, spacing: 6)
                if !theme.isTV { row.toolTip = name }
                row.setAccessibilityElement(true)
                row.setAccessibilityRole(.staticText)
                row.setAccessibilityLabel(name)
                return row
            }
            let wrap = WrapStack(views, spacing: MavioSpacing.s16, lineSpacing: MavioSpacing.s4)
            wrap.setAccessibilityElement(true)
            wrap.setAccessibilityRole(.group)
            wrap.setAccessibilityLabel(MavioStrings.itemRatings)
            return wrap
        }

        private func ratingText(_ text: String) -> NSTextField {
            let field = label(text, theme.fonts.callout, weight: .semibold, color: theme.colors.textPrimary)
            field.font = .monospacedDigitSystemFont(ofSize: theme.fonts.callout.size, weight: .semibold)
            return field
        }

        /// The services streaming it, by their logos.
        private func watchProviders() -> NSView? {
            let providers = title?.watchProviders ?? []
            guard !providers.isEmpty else { return nil }
            let theme = theme
            let logos = providers.map { p -> NSView in
                if !p.logoURL.isEmpty {
                    let art = ArtworkView(p.logoURL, title: p.name, theme: theme)
                    art.radius = 6
                    art.size(width: 32, height: 32)
                    art.alphaValue = 0.7
                    art.setAccessibilityElement(true)
                    art.setAccessibilityLabel(p.name)
                    return art
                }
                return NSTextField.label(p.name, theme.fonts.footnote, color: theme.colors.textPrimary)
            }
            let wrap = WrapStack(logos, spacing: MavioSpacing.s12, lineSpacing: MavioSpacing.s12)
            let stack = NSStackView.vertical(
                [
                    label(
                        MavioStrings.itemStreaming, theme.fonts.callout, weight: .semibold,
                        color: theme.colors.textPrimary), wrap,
                ],
                spacing: MavioSpacing.s8)
            wrap.widthAnchor.constraint(equalTo: stack.widthAnchor).isActive = true
            return stack
        }

        /// Facts, each a caption over its value, in as many columns as fit;
        /// a fact with links shows them in place of its value.
        private func factGrid(_ rows: [(String, String)], links: [String: [(String, Route?)]], page: Page) -> NSView? {
            guard !rows.isEmpty else { return nil }
            let theme = theme
            let grid = FlowGrid(minimum: 150, gutter: MavioSpacing.s32, rowSpacing: MavioSpacing.s20)
            grid.setItems(
                rows.map { key, value in
                    var views: [NSView] = [
                        label(key, theme.fonts.footnote, weight: .semibold, color: theme.colors.textSecondary)
                    ]
                    if let values = links[key], values.contains(where: { $0.1 != nil }) {
                        let words = values.enumerated().map { i, v -> NSView in
                            let text = i < values.count - 1 ? "\(v.0)," : v.0
                            if let route = v.1 {
                                return page.textLink(text, theme.fonts.callout, color: theme.colors.textPrimary) {
                                    [weak page] in page?.open(route)
                                }
                            }
                            return NSTextField.label(text, theme.fonts.callout, color: theme.colors.textPrimary)
                        }
                        views.append(WrapStack(words, spacing: 4, lineSpacing: 2))
                    } else {
                        let field = NSTextField.label(
                            value, theme.fonts.callout, color: theme.colors.textPrimary, lines: 0)
                        field.preferredMaxLayoutWidth = 150
                        views.append(field)
                    }
                    if key == studioKey, details.studios.count > Self.minStudios {
                        let more = NSStackView.horizontal(
                            [
                                NSTextField.label(
                                    allStudios ? MavioStrings.actionShowLess : MavioStrings.discoverShowMore,
                                    theme.fonts.callout, color: theme.colors.textSecondary),
                                IconView(
                                    allStudios ? "collapse" : "expand", size: 14, color: theme.colors.textSecondary),
                            ], spacing: MavioSpacing.s4)
                        views.append(
                            Pressable(more, look: .highlight, radius: MavioRadius.sm, theme: theme) { [weak self] in
                                guard let self else { return }
                                self.allStudios.toggle()
                                self.build()
                            })
                    }
                    let stack = NSStackView.vertical(views, spacing: MavioSpacing.s4)
                    for case let wrap as WrapStack in views {
                        wrap.widthAnchor.constraint(equalTo: stack.widthAnchor).isActive = true
                    }
                    return stack
                })
            return grid
        }

        // MARK: Loading

        /// Asks the server for the movie or series as it discovers it, and
        /// its ratings elsewhere; a server that does not discover tells
        /// nothing.
        private func loadTitle() {
            guard let kind = mediaKind, tmdbId > 0, !details.offline, let model = page?.model else { return }
            let serverId = card.serverId
            let tmdbId = tmdbId
            task = Task { [weak self] in
                async let told = try? model.title(serverId, kind, tmdbId)
                async let rated = try? model.ratings(serverId, kind, tmdbId)
                let (title, ratings) = await (told, rated)
                guard !Task.isCancelled, let self else { return }
                self.title = title
                self.titleRatings = ratings ?? []
                self.build()
            }
        }

        private func audioText(_ a: AudioStream) -> String {
            let channels =
                switch a.channels {
                case 6: "5.1"
                case 8: "7.1"
                case let n where n > 0: "\(n) ch"
                default: ""
                }
            return [a.title ?? language(a.language), a.codec.uppercased(), channels.isEmpty ? nil : channels]
                .compactMap(\.self).joined(separator: " · ")
        }

        private func subtitleText(_ s: SubtitleStream) -> String {
            [s.title ?? language(s.language), s.codec.uppercased()].compactMap(\.self).joined(separator: " · ")
        }

        private func language(_ code: String?) -> String? {
            guard let code, code != "und" else { return nil }
            return Locale.current.localizedString(forLanguageCode: code) ?? code
        }

        isolated deinit { task?.cancel() }
    }

    /// A column of views that is a region of its own in television mode,
    /// so that left and right cross to the other column only at its edge.
    final class RegionColumn: NSStackView, FocusRegion {
        /// Views one under another, spacing apart, each as wide as the column.
        init(_ views: [NSView], spacing: CGFloat) {
            super.init(frame: .zero)
            translatesAutoresizingMaskIntoConstraints = false
            orientation = .vertical
            alignment = .leading
            self.spacing = spacing
            setViews(views)
            for v in views where !(v is Pressable) { v.widthAnchor.constraint(equalTo: widthAnchor).isActive = true }
        }

        @available(*, unavailable)
        required init?(coder: NSCoder) { fatalError() }
    }
#endif
