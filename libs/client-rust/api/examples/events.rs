//! Signs in to a server and prints the events of the device's stream.
//!
//! ```text
//! cargo run --example events -- http://localhost:8096 <user> <password> [count]
//! ```

use connectrpc::client::{ClientConfig, HttpClient};
use mavio_api::connect::mavio::auth::v1::AuthServiceClient;
use mavio_api::connect::mavio::session::v1::EventServiceClient;
use mavio_api::proto::mavio::auth::v1::{Device, LoginRequest};
use mavio_api::proto::mavio::session::v1::{SubscribeRequest, SubscribeResponse};

#[tokio::main(flavor = "current_thread")]
async fn main() -> Result<(), Box<dyn std::error::Error>> {
    let args: Vec<String> = std::env::args().collect();
    let [_, server, name, password, rest @ ..] = args.as_slice() else {
        return Err("usage: events <server> <user> <password> [count]".into());
    };
    let count: usize = rest.first().map_or(Ok(3), |c| c.parse())?;
    let base: http::Uri = server.parse()?;
    let http = HttpClient::plaintext();

    let auth = AuthServiceClient::new(http.clone(), ClientConfig::new(base.clone()));
    let login = auth
        .login(LoginRequest {
            name: Some(name.clone()),
            password: Some(password.clone()),
            device: Some(Device {
                id: Some("mavio-api-events".into()),
                name: Some("Events example".into()),
                client: Some("mavio-api".into()),
                ..Default::default()
            })
            .into(),
            ..Default::default()
        })
        .await?
        .into_owned();
    let token = login.access_token.unwrap_or_default();

    let config =
        ClientConfig::new(base).with_default_header("authorization", format!("Bearer {token}"));
    let events = EventServiceClient::new(http, config);
    let mut stream = events.subscribe(SubscribeRequest::default()).await?;
    for _ in 0..count {
        let Some(msg) = stream.message::<SubscribeResponse>().await? else {
            break;
        };
        let event = msg
            .to_owned_message()
            .event
            .into_option()
            .unwrap_or_default();
        println!("{:?}", event.kind);
    }
    Ok(())
}
