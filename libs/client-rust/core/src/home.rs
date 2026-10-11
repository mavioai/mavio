//! Home across servers (`docs/clients.md` §5.4), as Plex builds it: one
//! Continue Watching across servers by last activity; a title on several
//! servers once, from the server the user played it on last, else the
//! first in their order; the other hubs server by server; Trending once.

use std::collections::HashMap;
use std::time::SystemTime;

use mavio_api::proto::mavio::acquisition::v1::{DiscoveredTitle, MediaKind};
use mavio_api::proto::mavio::home::v1::Hub;
use mavio_api::proto::mavio::library::v1::{HubKind, Item, ItemKind};

/// The providers whose IDs tell that two servers have the same title.
const MATCHING_PROVIDERS: &[&str] = &["tmdb", "tvdb", "imdb"];

/// What is known of one server's hubs.
#[derive(Debug, Clone)]
pub enum Hubs {
    /// The server has not answered yet, and nothing is kept of it.
    Loading,
    /// The server's hubs as it answered.
    Fresh(Vec<Hub>),
    /// The hubs kept from the last answer, when they were kept: the server
    /// is unreachable, or has not answered yet.
    Cached(Vec<Hub>, SystemTime),
    /// The server is unreachable and nothing is kept of it, with why.
    Unavailable(String),
}

impl Hubs {
    fn hubs(&self) -> &[Hub] {
        match self {
            Hubs::Fresh(h) | Hubs::Cached(h, _) => h,
            Hubs::Loading | Hubs::Unavailable(_) => &[],
        }
    }
}

/// One server's part of Home, in the user's order of servers.
#[derive(Debug, Clone)]
pub struct ServerHubs {
    /// The server.
    pub server_id: String,
    /// The name the server shows.
    pub server_name: String,
    /// Its hubs.
    pub hubs: Hubs,
}

/// Another server's copy of a title.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct Version {
    /// The other server.
    pub server_id: String,
    /// The title's item there.
    pub item_id: String,
}

/// An item on Home, with the other servers' copies of the same title.
#[derive(Debug, Clone, PartialEq)]
pub struct Entry {
    /// The server of the item.
    pub server_id: String,
    /// The item.
    pub item: Item,
    /// The same title on other servers.
    pub versions: Vec<Version>,
}

/// A trending title of discovery.
#[derive(Debug, Clone, PartialEq)]
pub struct Trending {
    /// The title, as the discovering server describes it.
    pub title: DiscoveredTitle,
    /// The servers that have it, whose item can be played instead of
    /// requesting it.
    pub available: Vec<Version>,
}

/// What a section shows.
#[derive(Debug, Clone, PartialEq)]
pub enum Content {
    /// Items of the libraries.
    Items(Vec<Entry>),
    /// Trending titles; requests go to the section's server.
    Titles(Vec<Trending>),
    /// The server has not answered yet.
    Loading,
    /// The server is unreachable, with why.
    Unavailable(String),
}

/// A row of Home.
#[derive(Debug, Clone, PartialEq)]
pub struct Section {
    /// Stable across updates: "continue-watching", "trending", or the
    /// server's ID and its hub's.
    pub id: String,
    /// The kind of hub; unspecified for a server's placeholder.
    pub kind: HubKind,
    /// The server of the section; unset for Continue Watching.
    pub server_id: Option<String>,
    /// That server's name.
    pub server_name: Option<String>,
    /// The hub's ID on its server, which pages its items.
    pub hub_id: Option<String>,
    /// The library of the hub, if one.
    pub library_name: Option<String>,
    /// What a seeded hub is drawn from.
    pub seed_name: Option<String>,
    /// Whether the hub has more items than shown.
    pub more: bool,
    /// Whether the section comes from the offline store.
    pub offline: bool,
    /// What it shows.
    pub content: Content,
}

impl Section {
    fn new(id: String, kind: HubKind) -> Self {
        Section {
            id,
            kind,
            server_id: None,
            server_name: None,
            hub_id: None,
            library_name: None,
            seed_name: None,
            more: false,
            offline: false,
            content: Content::Loading,
        }
    }
}

/// The matching keys of a title: its kind with each of its IDs of the
/// matching providers.
fn keys<'a>(class: &'a str, ids: impl Iterator<Item = (&'a String, &'a String)>) -> Vec<String> {
    let mut out: Vec<String> = ids
        .filter(|(p, v)| MATCHING_PROVIDERS.contains(&p.as_str()) && !v.is_empty())
        .map(|(p, v)| format!("{class}/{p}/{v}"))
        .collect();
    out.sort();
    out
}

fn item_class(item: &Item) -> &'static str {
    match item.kind.as_ref().and_then(|k| k.as_known()) {
        Some(ItemKind::ITEM_KIND_MOVIE) => "movie",
        Some(ItemKind::ITEM_KIND_SERIES) => "series",
        Some(ItemKind::ITEM_KIND_EPISODE) => "episode",
        Some(ItemKind::ITEM_KIND_SEASON) => "season",
        Some(ItemKind::ITEM_KIND_MUSIC_ALBUM) => "album",
        _ => "other",
    }
}

fn item_keys(item: &Item) -> Vec<String> {
    keys(item_class(item), item.external_ids.iter())
}

fn title_keys(title: &DiscoveredTitle) -> Vec<String> {
    let class = match title.media_kind.as_ref().and_then(|k| k.as_known()) {
        Some(MediaKind::MEDIA_KIND_MOVIE) => "movie",
        Some(MediaKind::MEDIA_KIND_SERIES) => "series",
        _ => "other",
    };
    keys(class, title.external_ids.iter())
}

fn last_played(item: &Item) -> Option<SystemTime> {
    item.user_data
        .as_option()?
        .last_played_time
        .as_option()
        .and_then(|t| SystemTime::try_from(t.clone()).ok())
}

fn item_id(item: &Item) -> &str {
    item.id.as_deref().unwrap_or_default()
}

/// Where each title is: its copies on the servers, grouped by matching
/// keys, and the server whose copy Home shows.
#[derive(Default)]
struct Titles {
    /// The group of each key.
    groups: HashMap<String, usize>,
    /// Union-find parents of the groups, which merge when an item shares
    /// keys with two of them.
    parents: Vec<usize>,
    /// The copies seen of each group: server index, item ID, last played.
    copies: Vec<Vec<(usize, String, Option<SystemTime>)>>,
}

impl Titles {
    fn root(&mut self, mut g: usize) -> usize {
        while self.parents[g] != g {
            self.parents[g] = self.parents[self.parents[g]];
            g = self.parents[g];
        }
        g
    }

    fn group(&mut self, keys: &[String]) -> Option<usize> {
        if keys.is_empty() {
            return None;
        }
        let mut found: Option<usize> = None;
        for k in keys {
            if let Some(&g) = self.groups.get(k) {
                let g = self.root(g);
                found = Some(match found {
                    Some(f) if f != g => {
                        let moved = std::mem::take(&mut self.copies[g]);
                        self.copies[f].extend(moved);
                        self.parents[g] = f;
                        f
                    }
                    _ => g,
                });
            }
        }
        let g = found.unwrap_or_else(|| {
            self.parents.push(self.parents.len());
            self.copies.push(Vec::new());
            self.parents.len() - 1
        });
        for k in keys {
            self.groups.insert(k.clone(), g);
        }
        Some(g)
    }

    fn add(&mut self, server: usize, item: &Item) {
        if let Some(g) = self.group(&item_keys(item)) {
            let id = item_id(item).to_owned();
            let copies = &mut self.copies[g];
            if !copies.iter().any(|(s, i, _)| *s == server && *i == id) {
                copies.push((server, id, last_played(item)));
            }
        }
    }

    fn of(&mut self, keys: &[String]) -> Option<usize> {
        let g = keys.iter().find_map(|k| self.groups.get(k).copied())?;
        Some(self.root(g))
    }

    /// The server whose copy of the group Home shows: the one played last,
    /// else the first in the user's order.
    fn preferred(&mut self, g: usize) -> usize {
        let g = self.root(g);
        self.copies[g]
            .iter()
            .min_by(|a, b| b.2.cmp(&a.2).then(a.0.cmp(&b.0)))
            .map_or(usize::MAX, |c| c.0)
    }

    fn versions(&mut self, g: usize, servers: &[ServerHubs], except: usize) -> Vec<Version> {
        let g = self.root(g);
        let mut out: Vec<Version> = Vec::new();
        for (s, id, _) in &self.copies[g] {
            if *s != except && !out.iter().any(|v| v.server_id == servers[*s].server_id) {
                out.push(Version {
                    server_id: servers[*s].server_id.clone(),
                    item_id: id.clone(),
                });
            }
        }
        out
    }
}

fn kind(hub: &Hub) -> Option<HubKind> {
    hub.kind.as_ref().and_then(|k| k.as_known())
}

/// Merges the servers' hubs into the sections of Home, the servers given
/// in the user's order.
pub fn merge(servers: &[ServerHubs]) -> Vec<Section> {
    let mut titles = Titles::default();
    for (s, server) in servers.iter().enumerate() {
        for hub in server.hubs.hubs() {
            for item in &hub.items {
                titles.add(s, item);
            }
        }
    }

    let mut out = Vec::new();
    if let Some(section) = continue_watching(servers, &mut titles) {
        out.push(section);
    }
    for s in 0..servers.len() {
        out.extend(server_sections(servers, s, &mut titles));
    }
    if let Some(section) = trending(servers, &mut titles) {
        out.push(section);
    }
    out
}

/// One Continue Watching across servers, by last activity: an item
/// without its own, such as a next episode, takes that of the item before
/// it on its server, which orders them by activity.
fn continue_watching(servers: &[ServerHubs], titles: &mut Titles) -> Option<Section> {
    let mut entries: Vec<(Option<SystemTime>, Entry)> = Vec::new();
    let mut offline = false;
    for (s, server) in servers.iter().enumerate() {
        let Some(hub) = server
            .hubs
            .hubs()
            .iter()
            .find(|h| kind(h) == Some(HubKind::HUB_KIND_CONTINUE_WATCHING))
        else {
            continue;
        };
        offline |= matches!(server.hubs, Hubs::Cached(..));
        let mut activity = None;
        for item in &hub.items {
            activity = last_played(item).or(activity);
            let versions = match titles.of(&item_keys(item)) {
                Some(g) if titles.preferred(g) != s => continue,
                Some(g) => titles.versions(g, servers, s),
                None => Vec::new(),
            };
            entries.push((
                activity,
                Entry {
                    server_id: server.server_id.clone(),
                    item: item.clone(),
                    versions,
                },
            ));
        }
    }
    if entries.is_empty() {
        return None;
    }
    // Stable: equal activity keeps the servers' order and their own.
    entries.sort_by_key(|e| std::cmp::Reverse(e.0));
    let mut section = Section::new(
        "continue-watching".into(),
        HubKind::HUB_KIND_CONTINUE_WATCHING,
    );
    section.offline = offline;
    section.content = Content::Items(entries.into_iter().map(|(_, e)| e).collect());
    Some(section)
}

/// A server's hubs other than Continue Watching and Trending, without the
/// titles Home shows from other servers; a placeholder while it loads or
/// when it is unreachable with nothing kept.
fn server_sections(servers: &[ServerHubs], s: usize, titles: &mut Titles) -> Vec<Section> {
    let server = &servers[s];
    let placeholder = |content| {
        let mut section = Section::new(
            format!("{}/", server.server_id),
            HubKind::HUB_KIND_UNSPECIFIED,
        );
        section.server_id = Some(server.server_id.clone());
        section.server_name = Some(server.server_name.clone());
        section.content = content;
        vec![section]
    };
    match &server.hubs {
        Hubs::Loading => return placeholder(Content::Loading),
        Hubs::Unavailable(why) => return placeholder(Content::Unavailable(why.clone())),
        Hubs::Fresh(_) | Hubs::Cached(..) => {}
    }
    let offline = matches!(server.hubs, Hubs::Cached(..));
    let mut out = Vec::new();
    for hub in server.hubs.hubs() {
        let k = kind(hub).unwrap_or(HubKind::HUB_KIND_UNSPECIFIED);
        if matches!(
            k,
            HubKind::HUB_KIND_CONTINUE_WATCHING | HubKind::HUB_KIND_TRENDING
        ) {
            continue;
        }
        let mut entries = Vec::new();
        for item in &hub.items {
            let versions = match titles.of(&item_keys(item)) {
                Some(g) if titles.preferred(g) != s => continue,
                Some(g) => titles.versions(g, servers, s),
                None => Vec::new(),
            };
            entries.push(Entry {
                server_id: server.server_id.clone(),
                item: item.clone(),
                versions,
            });
        }
        if entries.is_empty() {
            continue;
        }
        let hub_id = hub.id.clone().unwrap_or_default();
        let mut section = Section::new(format!("{}/{hub_id}", server.server_id), k);
        section.server_id = Some(server.server_id.clone());
        section.server_name = Some(server.server_name.clone());
        section.hub_id = Some(hub_id);
        section.library_name = hub.library_name.clone().filter(|n| !n.is_empty());
        section.seed_name = hub.seed_name.clone().filter(|n| !n.is_empty());
        section.more = hub.more.unwrap_or_default();
        section.offline = offline;
        section.content = Content::Items(entries);
        out.push(section);
    }
    out
}

/// Trending once, from the first server in the user's order that
/// discovers, its titles marked available where any server has them.
fn trending(servers: &[ServerHubs], titles: &mut Titles) -> Option<Section> {
    let (server, hub) = servers.iter().find_map(|server| {
        let hub = server
            .hubs
            .hubs()
            .iter()
            .find(|h| kind(h) == Some(HubKind::HUB_KIND_TRENDING) && !h.titles.is_empty())?;
        Some((server, hub))
    })?;
    let list = hub
        .titles
        .iter()
        .map(|title| {
            let available = match titles.of(&title_keys(title)) {
                Some(g) => titles.versions(g, servers, usize::MAX),
                None => Vec::new(),
            };
            Trending {
                title: title.clone(),
                available,
            }
        })
        .collect();
    let mut section = Section::new("trending".into(), HubKind::HUB_KIND_TRENDING);
    section.server_id = Some(server.server_id.clone());
    section.server_name = Some(server.server_name.clone());
    section.hub_id = hub.id.clone();
    section.more = hub.more.unwrap_or_default();
    section.offline = matches!(server.hubs, Hubs::Cached(..));
    section.content = Content::Titles(list);
    Some(section)
}

#[cfg(test)]
mod tests {
    use std::time::{Duration, UNIX_EPOCH};

    use mavio_api::proto::mavio::user::v1::UserData;

    use super::*;

    fn item(id: &str, kind: ItemKind, ids: &[(&str, &str)], played_at: Option<u64>) -> Item {
        Item {
            id: Some(id.into()),
            name: Some(id.into()),
            kind: Some(kind.into()),
            external_ids: ids
                .iter()
                .map(|(p, v)| (p.to_string(), v.to_string()))
                .collect(),
            user_data: played_at
                .map(|s| UserData {
                    last_played_time: Some((UNIX_EPOCH + Duration::from_secs(s)).into()).into(),
                    ..Default::default()
                })
                .into(),
            ..Default::default()
        }
    }

    fn movie(id: &str, tmdb: &str, played_at: Option<u64>) -> Item {
        item(id, ItemKind::ITEM_KIND_MOVIE, &[("tmdb", tmdb)], played_at)
    }

    fn hub(id: &str, kind: HubKind, items: Vec<Item>) -> Hub {
        Hub {
            id: Some(id.into()),
            kind: Some(kind.into()),
            items,
            ..Default::default()
        }
    }

    fn server(id: &str, hubs: Hubs) -> ServerHubs {
        ServerHubs {
            server_id: id.into(),
            server_name: id.to_uppercase(),
            hubs,
        }
    }

    /// The sections as "<id>: <server>:<item>…".
    fn summary(sections: &[Section]) -> Vec<String> {
        sections
            .iter()
            .map(|s| {
                let content = match &s.content {
                    Content::Items(e) => e
                        .iter()
                        .map(|e| format!("{}:{}", e.server_id, item_id(&e.item)))
                        .collect::<Vec<_>>()
                        .join(" "),
                    Content::Titles(t) => t
                        .iter()
                        .map(|t| {
                            let on: Vec<_> =
                                t.available.iter().map(|v| v.server_id.as_str()).collect();
                            format!(
                                "{}[{}]",
                                t.title.name.as_deref().unwrap_or_default(),
                                on.join(",")
                            )
                        })
                        .collect::<Vec<_>>()
                        .join(" "),
                    Content::Loading => "loading".into(),
                    Content::Unavailable(_) => "unavailable".into(),
                };
                format!("{}: {content}", s.id)
            })
            .collect()
    }

    const CW: HubKind = HubKind::HUB_KIND_CONTINUE_WATCHING;
    const RA: HubKind = HubKind::HUB_KIND_RECENTLY_ADDED;

    #[test]
    fn continue_watching_spans_servers_by_last_activity() {
        let a = server(
            "a",
            Hubs::Fresh(vec![hub(
                "continue-watching",
                CW,
                vec![
                    movie("m1", "1", Some(300)),
                    // A next episode: no activity of its own.
                    item("e1", ItemKind::ITEM_KIND_EPISODE, &[], None),
                    movie("m2", "2", Some(100)),
                ],
            )]),
        );
        let b = server(
            "b",
            Hubs::Fresh(vec![hub(
                "continue-watching",
                CW,
                vec![movie("m3", "3", Some(200))],
            )]),
        );
        assert_eq!(
            summary(&merge(&[a, b])),
            ["continue-watching: a:m1 a:e1 b:m3 a:m2"],
        );
    }

    #[test]
    fn a_title_on_several_servers_shows_once() {
        // Brazil (tmdb 68) is on both; played last on b. Amélie (194) is on
        // both, played on neither: shown from a, the first.
        let a = server(
            "a",
            Hubs::Fresh(vec![
                hub(
                    "continue-watching",
                    CW,
                    vec![movie("a-brazil", "68", Some(100))],
                ),
                hub(
                    "recently-added/l",
                    RA,
                    vec![movie("a-amelie", "194", None), movie("a-only", "5", None)],
                ),
            ]),
        );
        let b = server(
            "b",
            Hubs::Fresh(vec![
                hub(
                    "continue-watching",
                    CW,
                    vec![movie("b-brazil", "68", Some(200))],
                ),
                hub("recently-added/l", RA, vec![movie("b-amelie", "194", None)]),
            ]),
        );
        let sections = merge(&[a, b]);
        assert_eq!(
            summary(&sections),
            [
                "continue-watching: b:b-brazil",
                "a/recently-added/l: a:a-amelie a:a-only"
            ],
            "b's Recently Added is empty without Amélie and left out",
        );
        let Content::Items(cw) = &sections[0].content else {
            panic!()
        };
        assert_eq!(
            cw[0].versions,
            [Version {
                server_id: "a".into(),
                item_id: "a-brazil".into()
            }]
        );
    }

    #[test]
    fn ids_of_different_providers_link_one_title() {
        let a = server(
            "a",
            Hubs::Fresh(vec![hub(
                "recently-added/l",
                RA,
                vec![item(
                    "x",
                    ItemKind::ITEM_KIND_MOVIE,
                    &[("imdb", "tt1")],
                    None,
                )],
            )]),
        );
        let b = server(
            "b",
            Hubs::Fresh(vec![hub(
                "recently-added/l",
                RA,
                vec![item(
                    "y",
                    ItemKind::ITEM_KIND_MOVIE,
                    &[("tmdb", "9"), ("imdb", "tt1")],
                    None,
                )],
            )]),
        );
        // A movie and a series with the same TMDB number are different.
        let c = server(
            "c",
            Hubs::Fresh(vec![hub(
                "recently-added/l",
                RA,
                vec![item(
                    "z",
                    ItemKind::ITEM_KIND_SERIES,
                    &[("tmdb", "9")],
                    None,
                )],
            )]),
        );
        assert_eq!(
            summary(&merge(&[a, b, c])),
            ["a/recently-added/l: a:x", "c/recently-added/l: c:z"],
        );
    }

    #[test]
    fn servers_keep_their_places_while_loading_and_offline() {
        let kept = UNIX_EPOCH + Duration::from_secs(1);
        let servers = [
            server("a", Hubs::Loading),
            server(
                "b",
                Hubs::Cached(
                    vec![hub("recently-added/l", RA, vec![movie("m", "1", None)])],
                    kept,
                ),
            ),
            server("c", Hubs::Unavailable("refused".into())),
        ];
        let sections = merge(&servers);
        assert_eq!(
            summary(&sections),
            ["a/: loading", "b/recently-added/l: b:m", "c/: unavailable"],
        );
        assert!(sections[1].offline && !sections[0].offline);
    }

    #[test]
    fn trending_once_from_the_first_server_that_discovers() {
        let title = |name: &str, tmdb: &str| DiscoveredTitle {
            name: Some(name.into()),
            media_kind: Some(MediaKind::MEDIA_KIND_MOVIE.into()),
            external_ids: [("tmdb".to_string(), tmdb.to_string())]
                .into_iter()
                .collect(),
            ..Default::default()
        };
        let trending = |titles| Hub {
            id: Some("trending".into()),
            kind: Some(HubKind::HUB_KIND_TRENDING.into()),
            titles,
            ..Default::default()
        };
        let a = server(
            "a",
            Hubs::Fresh(vec![hub(
                "recently-added/l",
                RA,
                vec![movie("m", "1", None)],
            )]),
        );
        let b = server(
            "b",
            Hubs::Fresh(vec![trending(vec![title("Dune", "1"), title("Up", "2")])]),
        );
        let c = server("c", Hubs::Fresh(vec![trending(vec![title("Other", "3")])]));
        let sections = merge(&[a, b, c]);
        assert_eq!(
            summary(&sections),
            ["a/recently-added/l: a:m", "trending: Dune[a] Up[]"],
        );
        assert_eq!(
            sections[1].server_id.as_deref(),
            Some("b"),
            "requests go to b"
        );
    }
}
