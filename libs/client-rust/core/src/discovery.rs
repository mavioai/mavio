//! Finding servers on the local network (`docs/clients.md` §3): a UDP
//! broadcast to port 7359, which every server with local discovery on
//! answers with who it is and where.

use std::net::{Ipv4Addr, SocketAddr};
use std::time::Duration;

use serde::Deserialize;
use tokio::net::UdpSocket;

use crate::error::{Error, Result};

/// The port servers answer discovery on.
pub const DISCOVERY_PORT: u16 = 7359;

/// What a client broadcasts to find servers.
const REQUEST: &[u8] = b"who is MavioServer?";

/// A server found on the local network.
#[derive(Debug, Clone, PartialEq, Eq, Deserialize)]
pub struct FoundServer {
    /// The server's own ID, the same at every address.
    pub id: String,
    /// The name the server shows.
    pub name: String,
    /// Where it answers, such as `http://192.168.1.5:8686`.
    pub address: String,
    /// Its version.
    pub version: String,
}

/// Broadcasts a discovery request and gathers the answers that arrive
/// within wait, each server once.
pub async fn discover(wait: Duration) -> Result<Vec<FoundServer>> {
    discover_at(
        SocketAddr::from((Ipv4Addr::BROADCAST, DISCOVERY_PORT)),
        wait,
    )
    .await
}

async fn discover_at(to: SocketAddr, wait: Duration) -> Result<Vec<FoundServer>> {
    let io = |e: std::io::Error| Error::Unreachable(format!("discovery: {e}"));
    let socket = UdpSocket::bind((Ipv4Addr::UNSPECIFIED, 0))
        .await
        .map_err(io)?;
    socket.set_broadcast(true).map_err(io)?;
    socket.send_to(REQUEST, to).await.map_err(io)?;

    let mut found: Vec<FoundServer> = Vec::new();
    let mut buf = [0u8; 2048];
    let deadline = tokio::time::Instant::now() + wait;
    while let Ok(received) = tokio::time::timeout_at(deadline, socket.recv_from(&mut buf)).await {
        let Ok((n, _)) = received else { continue };
        // Anything else on the port is not a server's answer.
        let Ok(server) = serde_json::from_slice::<FoundServer>(&buf[..n]) else {
            continue;
        };
        if !found.iter().any(|s| s.id == server.id) {
            found.push(server);
        }
    }
    Ok(found)
}

#[cfg(test)]
mod tests {
    use super::*;

    #[tokio::test]
    async fn servers_answer_once_each() {
        let server = UdpSocket::bind((Ipv4Addr::LOCALHOST, 0)).await.unwrap();
        let addr = server.local_addr().unwrap();
        tokio::spawn(async move {
            let mut buf = [0u8; 64];
            let (n, from) = server.recv_from(&mut buf).await.unwrap();
            assert_eq!(&buf[..n], REQUEST);
            let reply =
                br#"{"id":"sa","name":"Home","address":"http://10.0.0.2:8686","version":"v1"}"#;
            server.send_to(b"noise", from).await.unwrap();
            server.send_to(reply, from).await.unwrap();
            server.send_to(reply, from).await.unwrap();
        });
        let got = discover_at(addr, Duration::from_millis(300)).await.unwrap();
        assert_eq!(
            got,
            [FoundServer {
                id: "sa".into(),
                name: "Home".into(),
                address: "http://10.0.0.2:8686".into(),
                version: "v1".into(),
            }]
        );
    }
}
