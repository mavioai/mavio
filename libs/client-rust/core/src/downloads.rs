//! Downloads (`docs/clients.md` §5.1): the queue and the state of each
//! download, kept in the offline store. The core asks the server for the
//! file to keep; the app moves its bytes with the platform's background
//! transfer and reports how far it got.

use std::sync::Arc;
use std::time::SystemTime;

use mavio_api::proto::mavio::library::v1::GetItemRequest;
use mavio_api::proto::mavio::playback::v1::ClientCapabilities;

use crate::error::{Error, Result};
use crate::reporting::{PlaybackOptions, Reporter};
use crate::servers::Servers;
use crate::store::{DownloadRecord, DownloadState, Store};

/// A transfer for the app to run.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct Transfer {
    /// The server of the item.
    pub server_id: String,
    /// The item downloaded.
    pub item_id: String,
    /// The file to fetch, served as an attachment; its ID is the
    /// credential, so it needs no headers.
    pub url: String,
}

/// The downloads of every server, in the order they were queued.
#[derive(Clone)]
pub struct Downloads {
    servers: Arc<Servers>,
    store: Arc<Store>,
    reporter: Reporter,
}

impl Downloads {
    /// The downloads kept in store.
    pub fn new(servers: Arc<Servers>, store: Arc<Store>, reporter: Reporter) -> Self {
        Downloads {
            servers,
            store,
            reporter,
        }
    }

    /// Every download.
    pub fn list(&self) -> Result<Vec<DownloadRecord>> {
        self.store.downloads()
    }

    fn get(&self, server_id: &str, item_id: &str) -> Result<DownloadRecord> {
        self.list()?
            .into_iter()
            .find(|d| d.server_id == server_id && d.item_id() == item_id)
            .ok_or_else(|| Error::NotFound(format!("download of {item_id}")))
    }

    fn set(
        &self,
        server_id: &str,
        item_id: &str,
        f: impl FnOnce(&mut DownloadRecord),
    ) -> Result<DownloadRecord> {
        let mut d = self.get(server_id, item_id)?;
        f(&mut d);
        self.store.put_download(&d)?;
        Ok(d)
    }

    /// Queues an item, keeping its description for offline use, and returns
    /// the transfer to start. Queuing it again restarts it.
    pub async fn add(
        &self,
        server_id: &str,
        item_id: &str,
        capabilities: ClientCapabilities,
    ) -> Result<Transfer> {
        let conn = self.servers.connection(server_id)?;
        let item = conn
            .items()
            .get_item(GetItemRequest {
                id: Some(item_id.to_owned()),
                ..Default::default()
            })
            .await?
            .into_owned()
            .item
            .into_option()
            .ok_or_else(|| Error::Server(format!("item {item_id} came without its item")))?;
        let transfer = self.transfer(server_id, item_id, capabilities).await?;
        self.store.put_download(&DownloadRecord {
            server_id: server_id.to_owned(),
            item,
            state: DownloadState::Queued,
            file: String::new(),
            received: 0,
            total: 0,
            error: String::new(),
            added: SystemTime::now(),
        })?;
        Ok(transfer)
    }

    /// The transfer of a paused or failed download, whose URL the server
    /// makes anew; it is queued again.
    pub async fn resume(
        &self,
        server_id: &str,
        item_id: &str,
        capabilities: ClientCapabilities,
    ) -> Result<Transfer> {
        self.get(server_id, item_id)?;
        let transfer = self.transfer(server_id, item_id, capabilities).await?;
        self.set(server_id, item_id, |d| {
            d.state = DownloadState::Queued;
            d.error.clear();
        })?;
        Ok(transfer)
    }

    async fn transfer(
        &self,
        server_id: &str,
        item_id: &str,
        capabilities: ClientCapabilities,
    ) -> Result<Transfer> {
        let options = PlaybackOptions {
            download: true,
            ..Default::default()
        };
        let session = self
            .reporter
            .start(server_id, item_id, capabilities, options)
            .await?;
        Ok(Transfer {
            server_id: server_id.to_owned(),
            item_id: item_id.to_owned(),
            url: session.playback().url.clone(),
        })
    }

    /// The transfer started or moved on: the file the app writes, relative
    /// to its downloads folder, and the bytes so far.
    pub fn progress(
        &self,
        server_id: &str,
        item_id: &str,
        file: &str,
        received: u64,
        total: u64,
    ) -> Result<DownloadRecord> {
        self.set(server_id, item_id, |d| {
            d.state = DownloadState::Running;
            d.file = file.to_owned();
            d.received = received;
            d.total = total.max(received);
        })
    }

    /// The file is complete and plays offline.
    pub fn finished(&self, server_id: &str, item_id: &str) -> Result<DownloadRecord> {
        self.set(server_id, item_id, |d| {
            d.state = DownloadState::Done;
            d.total = d.total.max(d.received);
            d.received = d.total;
        })
    }

    /// The transfer failed, with why.
    pub fn failed(&self, server_id: &str, item_id: &str, error: &str) -> Result<DownloadRecord> {
        self.set(server_id, item_id, |d| {
            d.state = DownloadState::Failed;
            d.error = error.to_owned();
        })
    }

    /// The user paused the transfer.
    pub fn paused(&self, server_id: &str, item_id: &str) -> Result<DownloadRecord> {
        self.set(server_id, item_id, |d| d.state = DownloadState::Paused)
    }

    /// Forgets a download, returning its file for the app to delete.
    pub fn remove(&self, server_id: &str, item_id: &str) -> Result<String> {
        let d = self.get(server_id, item_id)?;
        self.store.remove_download(server_id, item_id)?;
        Ok(d.file)
    }
}

#[cfg(test)]
mod tests {
    use mavio_api::proto::mavio::library::v1::{GetItemResponse, Item};
    use mavio_api::proto::mavio::playback::v1::{StartPlaybackRequest, StartPlaybackResponse};

    use super::*;
    use crate::servers::tests::{fake_server, servers_with_store};

    #[tokio::test]
    async fn downloads_move_through_their_states() {
        let (servers, _, store) = servers_with_store();
        let servers = Arc::new(servers);
        let fake = fake_server("sa", "Home");
        fake.unary(
            "/mavio.library.v1.ItemService/GetItem",
            |_, req: GetItemRequest| {
                Ok(GetItemResponse {
                    item: Item {
                        id: req.id,
                        name: Some("Brazil".into()),
                        ..Default::default()
                    }
                    .into(),
                    ..Default::default()
                })
            },
        );
        fake.unary(
            "/mavio.playback.v1.PlaybackService/StartPlayback",
            |_, req: StartPlaybackRequest| {
                if req.download != Some(true) {
                    return Err(crate::fake::error("invalid_argument"));
                }
                Ok(StartPlaybackResponse {
                    playback_id: Some("p".into()),
                    url: Some("media/p/file".into()),
                    ..Default::default()
                })
            },
        );
        let addr = fake.serve().await;
        servers.sign_in(&addr, "me", "secret").await.unwrap();
        let reporter = Reporter::new(servers.clone(), store.clone());
        let downloads = Downloads::new(servers, store, reporter);

        let t = downloads
            .add("sa", "i1", ClientCapabilities::default())
            .await
            .unwrap();
        assert_eq!(t.url, format!("{addr}/media/p/file"));
        let d = downloads
            .progress("sa", "i1", "sa/i1.mkv", 10, 100)
            .unwrap();
        assert_eq!(
            (d.state, d.item.name.as_deref()),
            (DownloadState::Running, Some("Brazil"))
        );
        downloads.failed("sa", "i1", "connection lost").unwrap();
        downloads
            .resume("sa", "i1", ClientCapabilities::default())
            .await
            .unwrap();
        let d = downloads.finished("sa", "i1").unwrap();
        assert_eq!(
            (d.state, d.received, d.error.as_str()),
            (DownloadState::Done, 100, "")
        );
        assert_eq!(downloads.remove("sa", "i1").unwrap(), "sa/i1.mkv");
        assert!(downloads.list().unwrap().is_empty());
    }
}
