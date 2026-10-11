//! The offline store (`docs/clients.md` §5.1): SQLite with the servers the
//! user added, what the screens last showed of each, so that the app
//! starts on cached content, the progress made while a server was
//! unreachable, and the downloads.

use std::path::Path;
use std::sync::Mutex;
use std::time::{Duration, SystemTime, UNIX_EPOCH};

use buffa::Message;
use rusqlite::{Connection, OptionalExtension, params};

use crate::error::{Error, Result};

/// The schema, one migration per entry; `PRAGMA user_version` counts those
/// applied. Applied migrations never change.
const MIGRATIONS: &[&str] = &["
    CREATE TABLE servers (
        id TEXT PRIMARY KEY,
        address TEXT NOT NULL,
        name TEXT NOT NULL,
        version TEXT NOT NULL,
        user_id TEXT NOT NULL,
        user_name TEXT NOT NULL,
        position INTEGER NOT NULL
    );
    CREATE TABLE cache (
        server_id TEXT NOT NULL REFERENCES servers (id) ON DELETE CASCADE,
        key TEXT NOT NULL,
        value BLOB NOT NULL,
        updated_ms INTEGER NOT NULL,
        PRIMARY KEY (server_id, key)
    );
    CREATE TABLE pending_progress (
        server_id TEXT NOT NULL REFERENCES servers (id) ON DELETE CASCADE,
        item_id TEXT NOT NULL,
        position_ms INTEGER,
        played INTEGER NOT NULL,
        at_ms INTEGER NOT NULL,
        PRIMARY KEY (server_id, item_id)
    );
    CREATE TABLE downloads (
        server_id TEXT NOT NULL REFERENCES servers (id) ON DELETE CASCADE,
        item_id TEXT NOT NULL,
        item BLOB NOT NULL,
        state INTEGER NOT NULL,
        file TEXT NOT NULL,
        received INTEGER NOT NULL,
        total INTEGER NOT NULL,
        error TEXT NOT NULL,
        added_ms INTEGER NOT NULL,
        PRIMARY KEY (server_id, item_id)
    );
"];

/// A server the user added and is signed in to; its access token is kept
/// by the app's [`TokenStore`](crate::TokenStore), not here.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct ServerRecord {
    /// The server's own ID, the same at every address.
    pub id: String,
    /// The URL the server was last reached at, such as
    /// `https://media.example.com/mavio`.
    pub address: String,
    /// The name the server shows.
    pub name: String,
    /// The server's version.
    pub version: String,
    /// The ID of the user signed in.
    pub user_id: String,
    /// The name of the user signed in.
    pub user_name: String,
}

/// Progress made while a server was unreachable, replayed once it answers:
/// the last position of an item, or that it was played to the end.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct PendingProgress {
    /// The server of the item.
    pub server_id: String,
    /// The item played.
    pub item_id: String,
    /// Unset when the item was played to the end.
    pub position: Option<Duration>,
    /// Whether it was played to the end.
    pub played: bool,
    /// When the progress was made.
    pub at: SystemTime,
}

/// The state of a download.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum DownloadState {
    /// Waiting for its turn.
    Queued,
    /// The app moves its bytes.
    Running,
    /// Stopped by the user, to resume later.
    Paused,
    /// Its file is complete and plays offline.
    Done,
    /// The transfer failed; it can be retried.
    Failed,
}

impl DownloadState {
    fn from_i64(v: i64) -> Self {
        match v {
            1 => DownloadState::Running,
            2 => DownloadState::Paused,
            3 => DownloadState::Done,
            4 => DownloadState::Failed,
            _ => DownloadState::Queued,
        }
    }

    fn to_i64(self) -> i64 {
        match self {
            DownloadState::Queued => 0,
            DownloadState::Running => 1,
            DownloadState::Paused => 2,
            DownloadState::Done => 3,
            DownloadState::Failed => 4,
        }
    }
}

/// An item downloaded, or to download, from a server.
#[derive(Debug, Clone, PartialEq)]
pub struct DownloadRecord {
    /// The server of the item.
    pub server_id: String,
    /// The item, as the server described it when it was queued.
    pub item: mavio_api::proto::mavio::library::v1::Item,
    /// How far it is.
    pub state: DownloadState,
    /// Where the app keeps the file, once it chose; relative to its
    /// downloads folder.
    pub file: String,
    /// The bytes received.
    pub received: u64,
    /// Unknown until the transfer starts.
    pub total: u64,
    /// Why it failed.
    pub error: String,
    /// When it was queued.
    pub added: SystemTime,
}

impl DownloadRecord {
    /// The item's ID.
    pub fn item_id(&self) -> &str {
        self.item.id.as_deref().unwrap_or_default()
    }
}

/// The offline store. Its calls are short and synchronous; it is shared by
/// the core's tasks behind a mutex.
pub struct Store {
    conn: Mutex<Connection>,
}

fn millis(t: SystemTime) -> i64 {
    t.duration_since(UNIX_EPOCH)
        .map_or(0, |d| i64::try_from(d.as_millis()).unwrap_or(i64::MAX))
}

fn from_millis(ms: i64) -> SystemTime {
    UNIX_EPOCH + Duration::from_millis(u64::try_from(ms).unwrap_or(0))
}

impl Store {
    /// Opens the store at path, creating it or bringing its schema up to
    /// date.
    pub fn open(path: impl AsRef<Path>) -> Result<Self> {
        Self::init(Connection::open(path)?)
    }

    /// Opens a store that lives in memory, for tests and for apps that keep
    /// nothing.
    pub fn open_in_memory() -> Result<Self> {
        Self::init(Connection::open_in_memory()?)
    }

    fn init(conn: Connection) -> Result<Self> {
        conn.pragma_update(None, "foreign_keys", true)?;
        // In memory there is no journal to choose.
        let _ = conn.pragma_update(None, "journal_mode", "wal");
        let applied: i64 = conn.pragma_query_value(None, "user_version", |r| r.get(0))?;
        let applied = usize::try_from(applied).unwrap_or(0);
        if applied > MIGRATIONS.len() {
            return Err(Error::Store(format!(
                "schema version {applied} is newer than this app's {}",
                MIGRATIONS.len()
            )));
        }
        for (i, sql) in MIGRATIONS.iter().enumerate().skip(applied) {
            conn.execute_batch(&format!(
                "BEGIN; {sql}; PRAGMA user_version = {}; COMMIT;",
                i + 1
            ))?;
        }
        Ok(Store {
            conn: Mutex::new(conn),
        })
    }

    fn conn(&self) -> std::sync::MutexGuard<'_, Connection> {
        // A panic while holding the lock leaves SQLite consistent.
        self.conn.lock().unwrap_or_else(|e| e.into_inner())
    }

    /// The servers in the user's order.
    pub fn servers(&self) -> Result<Vec<ServerRecord>> {
        let conn = self.conn();
        let mut stmt = conn.prepare(
            "SELECT id, address, name, version, user_id, user_name FROM servers ORDER BY position, id",
        )?;
        let rows = stmt.query_map([], |r| {
            Ok(ServerRecord {
                id: r.get(0)?,
                address: r.get(1)?,
                name: r.get(2)?,
                version: r.get(3)?,
                user_id: r.get(4)?,
                user_name: r.get(5)?,
            })
        })?;
        Ok(rows.collect::<Result<_, _>>()?)
    }

    /// Adds a server after the others, or updates one already added,
    /// keeping its place.
    pub fn put_server(&self, s: &ServerRecord) -> Result<()> {
        self.conn().execute(
            "INSERT INTO servers (id, address, name, version, user_id, user_name, position)
             VALUES (?1, ?2, ?3, ?4, ?5, ?6, (SELECT COALESCE(MAX(position) + 1, 0) FROM servers))
             ON CONFLICT (id) DO UPDATE SET address = excluded.address, name = excluded.name,
                 version = excluded.version, user_id = excluded.user_id, user_name = excluded.user_name",
            params![s.id, s.address, s.name, s.version, s.user_id, s.user_name],
        )?;
        Ok(())
    }

    /// Removes a server with everything kept of it.
    pub fn remove_server(&self, id: &str) -> Result<()> {
        self.conn()
            .execute("DELETE FROM servers WHERE id = ?1", [id])?;
        Ok(())
    }

    /// Puts the servers in the order of ids; servers not listed follow in
    /// their present order.
    pub fn order_servers(&self, ids: &[String]) -> Result<()> {
        let mut conn = self.conn();
        let tx = conn.transaction()?;
        let rest: Vec<String> = {
            let mut stmt = tx.prepare("SELECT id FROM servers ORDER BY position, id")?;
            stmt.query_map([], |r| r.get::<_, String>(0))?
                .filter_map(Result::ok)
                .filter(|id| !ids.contains(id))
                .collect()
        };
        for (i, id) in ids.iter().chain(&rest).enumerate() {
            tx.execute(
                "UPDATE servers SET position = ?1 WHERE id = ?2",
                params![i as i64, id],
            )?;
        }
        tx.commit()?;
        Ok(())
    }

    /// Keeps a message the server answered, such as a screen's content,
    /// under key.
    pub fn put_cached(&self, server_id: &str, key: &str, msg: &impl Message) -> Result<()> {
        self.conn().execute(
            "INSERT INTO cache (server_id, key, value, updated_ms) VALUES (?1, ?2, ?3, ?4)
             ON CONFLICT (server_id, key) DO UPDATE SET value = excluded.value, updated_ms = excluded.updated_ms",
            params![server_id, key, msg.encode_to_vec(), millis(SystemTime::now())],
        )?;
        Ok(())
    }

    /// Returns the message kept under key, and when it was kept.
    pub fn cached<M: Message>(
        &self,
        server_id: &str,
        key: &str,
    ) -> Result<Option<(M, SystemTime)>> {
        let row: Option<(Vec<u8>, i64)> = self
            .conn()
            .query_row(
                "SELECT value, updated_ms FROM cache WHERE server_id = ?1 AND key = ?2",
                params![server_id, key],
                |r| Ok((r.get(0)?, r.get(1)?)),
            )
            .optional()?;
        row.map(|(value, at)| Ok((M::decode_from_slice(&value)?, from_millis(at))))
            .transpose()
    }

    /// Records progress to replay, replacing earlier progress of the item.
    pub fn queue_progress(&self, p: &PendingProgress) -> Result<()> {
        let position = p
            .position
            .map(|d| i64::try_from(d.as_millis()).unwrap_or(i64::MAX));
        self.conn().execute(
            "INSERT INTO pending_progress (server_id, item_id, position_ms, played, at_ms) VALUES (?1, ?2, ?3, ?4, ?5)
             ON CONFLICT (server_id, item_id) DO UPDATE SET position_ms = excluded.position_ms,
                 played = excluded.played, at_ms = excluded.at_ms",
            params![p.server_id, p.item_id, position, p.played, millis(p.at)],
        )?;
        Ok(())
    }

    /// The progress to replay to a server, oldest first.
    pub fn pending_progress(&self, server_id: &str) -> Result<Vec<PendingProgress>> {
        let conn = self.conn();
        let mut stmt = conn.prepare(
            "SELECT item_id, position_ms, played, at_ms FROM pending_progress WHERE server_id = ?1 ORDER BY at_ms",
        )?;
        let rows = stmt.query_map([server_id], |r| {
            Ok(PendingProgress {
                server_id: server_id.to_owned(),
                item_id: r.get(0)?,
                position: r
                    .get::<_, Option<i64>>(1)?
                    .map(|ms| Duration::from_millis(u64::try_from(ms).unwrap_or(0))),
                played: r.get(2)?,
                at: from_millis(r.get(3)?),
            })
        })?;
        Ok(rows.collect::<Result<_, _>>()?)
    }

    /// Forgets replayed progress, unless newer progress of the item was
    /// recorded meanwhile.
    pub fn remove_progress(&self, p: &PendingProgress) -> Result<()> {
        self.conn().execute(
            "DELETE FROM pending_progress WHERE server_id = ?1 AND item_id = ?2 AND at_ms <= ?3",
            params![p.server_id, p.item_id, millis(p.at)],
        )?;
        Ok(())
    }

    /// Adds a download, or replaces one of the same item.
    pub fn put_download(&self, d: &DownloadRecord) -> Result<()> {
        self.conn().execute(
            "INSERT INTO downloads (server_id, item_id, item, state, file, received, total, error, added_ms)
             VALUES (?1, ?2, ?3, ?4, ?5, ?6, ?7, ?8, ?9)
             ON CONFLICT (server_id, item_id) DO UPDATE SET item = excluded.item, state = excluded.state,
                 file = excluded.file, received = excluded.received, total = excluded.total, error = excluded.error",
            params![
                d.server_id,
                d.item_id(),
                d.item.encode_to_vec(),
                d.state.to_i64(),
                d.file,
                d.received as i64,
                d.total as i64,
                d.error,
                millis(d.added),
            ],
        )?;
        Ok(())
    }

    /// Removes a download's record; the app deletes its file.
    pub fn remove_download(&self, server_id: &str, item_id: &str) -> Result<()> {
        self.conn().execute(
            "DELETE FROM downloads WHERE server_id = ?1 AND item_id = ?2",
            params![server_id, item_id],
        )?;
        Ok(())
    }

    /// The downloads in the order they were added.
    pub fn downloads(&self) -> Result<Vec<DownloadRecord>> {
        let conn = self.conn();
        let mut stmt = conn.prepare(
            "SELECT server_id, item, state, file, received, total, error, added_ms FROM downloads
             ORDER BY added_ms, server_id, item_id",
        )?;
        let rows = stmt.query_map([], |r| {
            Ok((
                r.get::<_, String>(0)?,
                r.get::<_, Vec<u8>>(1)?,
                DownloadState::from_i64(r.get(2)?),
                r.get::<_, String>(3)?,
                r.get::<_, i64>(4)?,
                r.get::<_, i64>(5)?,
                r.get::<_, String>(6)?,
                from_millis(r.get(7)?),
            ))
        })?;
        let mut out = Vec::new();
        for row in rows {
            let (server_id, item, state, file, received, total, error, added) = row?;
            out.push(DownloadRecord {
                server_id,
                item: Message::decode_from_slice(&item)?,
                state,
                file,
                received: u64::try_from(received).unwrap_or(0),
                total: u64::try_from(total).unwrap_or(0),
                error,
                added,
            });
        }
        Ok(out)
    }
}

#[cfg(test)]
mod tests {
    use mavio_api::proto::mavio::library::v1::Item;

    use super::*;

    fn server(id: &str) -> ServerRecord {
        ServerRecord {
            id: id.into(),
            address: format!("http://{id}:8686"),
            name: id.to_uppercase(),
            version: "v1".into(),
            user_id: "u".into(),
            user_name: "me".into(),
        }
    }

    fn ids(store: &Store) -> Vec<String> {
        store.servers().unwrap().into_iter().map(|s| s.id).collect()
    }

    #[test]
    fn servers_keep_their_order() {
        let store = Store::open_in_memory().unwrap();
        for id in ["a", "b", "c"] {
            store.put_server(&server(id)).unwrap();
        }
        let mut moved = server("a");
        moved.address = "https://a.example".into();
        store.put_server(&moved).unwrap();
        assert_eq!(ids(&store), ["a", "b", "c"], "updating keeps the place");
        assert_eq!(store.servers().unwrap()[0].address, "https://a.example");

        store.order_servers(&["c".into(), "a".into()]).unwrap();
        assert_eq!(ids(&store), ["c", "a", "b"]);
        store.remove_server("a").unwrap();
        assert_eq!(ids(&store), ["c", "b"]);
    }

    #[test]
    fn cache_round_trips_and_goes_with_its_server() {
        let store = Store::open_in_memory().unwrap();
        store.put_server(&server("a")).unwrap();
        let item = Item {
            id: Some("i".into()),
            name: Some("Brazil".into()),
            ..Default::default()
        };
        store.put_cached("a", "item/i", &item).unwrap();
        let (got, _) = store.cached::<Item>("a", "item/i").unwrap().unwrap();
        assert_eq!(got.name.as_deref(), Some("Brazil"));
        assert!(store.cached::<Item>("a", "item/j").unwrap().is_none());

        store.remove_server("a").unwrap();
        assert!(store.cached::<Item>("a", "item/i").unwrap().is_none());
    }

    #[test]
    fn progress_keeps_the_latest_and_survives_replay_of_older() {
        let store = Store::open_in_memory().unwrap();
        store.put_server(&server("a")).unwrap();
        let at = |s| UNIX_EPOCH + Duration::from_secs(s);
        let first = PendingProgress {
            server_id: "a".into(),
            item_id: "i".into(),
            position: Some(Duration::from_secs(60)),
            played: false,
            at: at(100),
        };
        store.queue_progress(&first).unwrap();
        let later = PendingProgress {
            position: Some(Duration::from_secs(90)),
            at: at(200),
            ..first.clone()
        };
        store.queue_progress(&later).unwrap();
        assert_eq!(
            store.pending_progress("a").unwrap(),
            std::slice::from_ref(&later)
        );

        // Replaying the first leaves the later.
        store.remove_progress(&first).unwrap();
        assert_eq!(store.pending_progress("a").unwrap().len(), 1);
        store.remove_progress(&later).unwrap();
        assert!(store.pending_progress("a").unwrap().is_empty());
    }

    #[test]
    fn downloads_round_trip() {
        let store = Store::open_in_memory().unwrap();
        store.put_server(&server("a")).unwrap();
        let mut d = DownloadRecord {
            server_id: "a".into(),
            item: Item {
                id: Some("i".into()),
                ..Default::default()
            },
            state: DownloadState::Queued,
            file: String::new(),
            received: 0,
            total: 0,
            error: String::new(),
            added: UNIX_EPOCH + Duration::from_secs(5),
        };
        store.put_download(&d).unwrap();
        (d.state, d.received, d.total) = (DownloadState::Running, 10, 100);
        store.put_download(&d).unwrap();
        assert_eq!(store.downloads().unwrap(), [d]);
        store.remove_download("a", "i").unwrap();
        assert!(store.downloads().unwrap().is_empty());
    }

    #[test]
    fn reopening_keeps_the_schema() {
        let dir = std::env::temp_dir().join(format!("mavio-store-{}", std::process::id()));
        std::fs::create_dir_all(&dir).unwrap();
        let path = dir.join("core.db");
        Store::open(&path)
            .unwrap()
            .put_server(&server("a"))
            .unwrap();
        assert_eq!(ids(&Store::open(&path).unwrap()), ["a"]);
        std::fs::remove_dir_all(dir).unwrap();
    }
}
