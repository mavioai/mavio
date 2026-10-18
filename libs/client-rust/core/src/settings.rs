//! The settings the servers keep for the user: the playback preferences
//! the user's devices share, the users a session on a standalone server
//! switches between (`docs/accounts.md` §4.4), how the user signs in to a
//! standalone server, and the user's devices signed in.

use std::time::SystemTime;

use mavio_api::proto::mavio::auth::v1::SwitchableUser as SwitchableUserProto;
use mavio_api::proto::mavio::session::v1::{
    ListSessionsRequest, RevokeSessionRequest, Session as SessionProto,
};
use mavio_api::proto::mavio::user::v1::{
    ConfirmTotpRequest, DeletePasskeyRequest, DeleteTotpRequest, GetCurrentUserRequest,
    PlaybackPreferences as PlaybackPreferencesProto, SetPasswordRequest, SetPinRequest,
    StartTotpRequest, SubtitleMode as SubtitleModeProto, User,
};

use crate::error::Result;
use crate::servers::Servers;

/// When subtitles show.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum SubtitleMode {
    /// As the video marks its streams: their default and forced flags.
    Default,
    /// Always.
    Always,
    /// When the audio is not in a preferred language.
    Foreign,
    /// Only forced subtitles.
    Forced,
    /// Never.
    Never,
    /// When the audio differs from the preferred languages, and forced
    /// ones otherwise.
    Smart,
}

impl SubtitleMode {
    fn from_proto(m: Option<SubtitleModeProto>) -> Self {
        match m {
            Some(SubtitleModeProto::SUBTITLE_MODE_ALWAYS) => SubtitleMode::Always,
            Some(SubtitleModeProto::SUBTITLE_MODE_FOREIGN) => SubtitleMode::Foreign,
            Some(SubtitleModeProto::SUBTITLE_MODE_FORCED) => SubtitleMode::Forced,
            Some(SubtitleModeProto::SUBTITLE_MODE_NONE) => SubtitleMode::Never,
            Some(SubtitleModeProto::SUBTITLE_MODE_SMART) => SubtitleMode::Smart,
            _ => SubtitleMode::Default,
        }
    }

    fn to_proto(self) -> SubtitleModeProto {
        match self {
            SubtitleMode::Default => SubtitleModeProto::SUBTITLE_MODE_UNSPECIFIED,
            SubtitleMode::Always => SubtitleModeProto::SUBTITLE_MODE_ALWAYS,
            SubtitleMode::Foreign => SubtitleModeProto::SUBTITLE_MODE_FOREIGN,
            SubtitleMode::Forced => SubtitleModeProto::SUBTITLE_MODE_FORCED,
            SubtitleMode::Never => SubtitleModeProto::SUBTITLE_MODE_NONE,
            SubtitleMode::Smart => SubtitleModeProto::SUBTITLE_MODE_SMART,
        }
    }
}

/// The user's defaults for choosing the audio and subtitles of what they
/// play, which the servers apply.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct PlaybackPreferences {
    /// ISO 639-2/B codes in order of preference.
    pub audio_languages: Vec<String>,
    /// ISO 639-2/B codes in order of preference.
    pub subtitle_languages: Vec<String>,
    /// When subtitles show.
    pub subtitle_mode: SubtitleMode,
    /// Whether the audio stream the video marks default plays rather than
    /// one in a preferred language.
    pub play_default_audio_track: bool,
}

impl PlaybackPreferences {
    pub(crate) fn from_proto(p: &PlaybackPreferencesProto) -> Self {
        PlaybackPreferences {
            audio_languages: p.audio_languages.clone(),
            subtitle_languages: p.subtitle_languages.clone(),
            subtitle_mode: SubtitleMode::from_proto(p.subtitle_mode.and_then(|m| m.as_known())),
            // The server plays the default track unless told otherwise.
            play_default_audio_track: p.play_default_audio_track.unwrap_or(true),
        }
    }

    pub(crate) fn to_proto(&self) -> PlaybackPreferencesProto {
        PlaybackPreferencesProto {
            audio_languages: self.audio_languages.clone(),
            subtitle_languages: self.subtitle_languages.clone(),
            subtitle_mode: Some(self.subtitle_mode.to_proto().into()),
            play_default_audio_track: Some(self.play_default_audio_track),
            ..Default::default()
        }
    }
}

/// A user of a standalone server whom its session can switch to.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct SwitchableUser {
    /// The user's ID.
    pub id: String,
    /// The user's name.
    pub name: String,
    /// Whether the user administers the server.
    pub admin: bool,
    /// Whether the user is managed, without a password of their own.
    pub managed: bool,
    /// Whether switching to the user asks for their PIN.
    pub has_pin: bool,
    /// Whether the session acts as the user now.
    pub current: bool,
}

impl SwitchableUser {
    pub(crate) fn from_proto(u: &SwitchableUserProto) -> Self {
        SwitchableUser {
            id: u.id.clone().unwrap_or_default(),
            name: u.name.clone().unwrap_or_default(),
            admin: u.admin.unwrap_or_default(),
            managed: u.managed.unwrap_or_default(),
            has_pin: u.has_pin.unwrap_or_default(),
            current: u.current.unwrap_or_default(),
        }
    }
}

/// How the user signs in to a standalone server.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct AccountSecurity {
    /// Whether the user is managed, without credentials of their own; the
    /// rest is empty then.
    pub managed: bool,
    /// Whether the user signs in with a password of the server's, rather
    /// than through another provider.
    pub has_password: bool,
    /// The user's passkeys.
    pub passkeys: Vec<Passkey>,
    /// Whether sign-ins with a password ask for an authenticator app's code.
    pub has_totp: bool,
    /// Whether switching to the user asks for their PIN.
    pub has_pin: bool,
}

/// A passkey of the user's.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct Passkey {
    /// The passkey's ID.
    pub id: String,
    /// What it was named, such as the browser it was made in.
    pub name: String,
    /// The host it works at.
    pub host: String,
    /// When it was made.
    pub created: Option<SystemTime>,
}

/// The key of an authenticator app being set up, which a code from the app
/// confirms.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct TotpSetup {
    /// The key in base32.
    pub secret: String,
    /// The `otpauth:` URI that opens the key in an authenticator app.
    pub uri: String,
}

/// A device signed in to a server as the user.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct DeviceSession {
    /// The session's ID.
    pub id: String,
    /// The device's name.
    pub device_name: String,
    /// The app it signed in with.
    pub client: String,
    /// Whether its event stream is open.
    pub online: bool,
    /// Whether it is this device.
    pub current: bool,
    /// When it was last seen.
    pub last_seen: Option<SystemTime>,
    /// The name of what it plays, if anything.
    pub playing: Option<String>,
}

impl AccountSecurity {
    fn from_user(u: &User) -> Self {
        if u.managed.unwrap_or_default() {
            return AccountSecurity {
                managed: true,
                has_password: false,
                passkeys: Vec::new(),
                has_totp: false,
                has_pin: u.has_pin.unwrap_or_default(),
            };
        }
        AccountSecurity {
            managed: false,
            has_password: u.auth_provider.as_deref().unwrap_or_default().is_empty(),
            passkeys: u
                .passkeys
                .iter()
                .map(|p| Passkey {
                    id: p.id.clone().unwrap_or_default(),
                    name: p.name.clone().unwrap_or_default(),
                    host: p.rp_id.clone().unwrap_or_default(),
                    created: p
                        .create_time
                        .clone()
                        .into_option()
                        .and_then(|t| t.try_into().ok()),
                })
                .collect(),
            has_totp: u.has_totp.unwrap_or_default(),
            has_pin: u.has_pin.unwrap_or_default(),
        }
    }
}

impl DeviceSession {
    fn from_proto(s: &SessionProto) -> Self {
        let playing = s.now_playing.as_option().and_then(|p| p.item.as_option());
        DeviceSession {
            id: s.id.clone().unwrap_or_default(),
            device_name: s.device_name.clone().unwrap_or_default(),
            client: s.client.clone().unwrap_or_default(),
            online: s.online.unwrap_or_default(),
            current: s.current.unwrap_or_default(),
            last_seen: s
                .last_seen_time
                .clone()
                .into_option()
                .and_then(|t| t.try_into().ok()),
            playing: playing
                .and_then(|i| i.name.clone())
                .filter(|n| !n.is_empty()),
        }
    }
}

impl Servers {
    /// How the user signs in to a server.
    pub async fn security(&self, id: &str) -> Result<AccountSecurity> {
        let r = self
            .connection(id)?
            .users()
            .get_current_user(GetCurrentUserRequest::default())
            .await?
            .into_owned();
        Ok(AccountSecurity::from_user(
            &r.user.into_option().unwrap_or_default(),
        ))
    }

    /// Changes the user's own password, with the current one; the server
    /// signs the user out elsewhere.
    pub async fn set_password(&self, id: &str, current: &str, new: &str) -> Result<()> {
        let user_id = self.get(id)?.user_id;
        self.connection(id)?
            .users()
            .set_password(SetPasswordRequest {
                user_id: Some(user_id),
                current_password: Some(current.to_owned()),
                new_password: Some(new.to_owned()),
                ..Default::default()
            })
            .await?;
        Ok(())
    }

    /// Sets the user's four-digit PIN, or removes it when pin is empty.
    pub async fn set_pin(&self, id: &str, pin: &str) -> Result<()> {
        let user_id = self.get(id)?.user_id;
        self.connection(id)?
            .users()
            .set_pin(SetPinRequest {
                user_id: Some(user_id),
                pin: Some(pin.to_owned()),
                ..Default::default()
            })
            .await?;
        Ok(())
    }

    /// Begins setting up an authenticator app, giving its key.
    pub async fn start_totp(&self, id: &str) -> Result<TotpSetup> {
        let r = self
            .connection(id)?
            .users()
            .start_totp(StartTotpRequest::default())
            .await?
            .into_owned();
        Ok(TotpSetup {
            secret: r.secret.unwrap_or_default(),
            uri: r.uri.unwrap_or_default(),
        })
    }

    /// Finishes setting up an authenticator app with a code from it.
    pub async fn confirm_totp(&self, id: &str, code: &str) -> Result<()> {
        self.connection(id)?
            .users()
            .confirm_totp(ConfirmTotpRequest {
                code: Some(code.to_owned()),
                ..Default::default()
            })
            .await?;
        Ok(())
    }

    /// Turns the user's authenticator app off.
    pub async fn delete_totp(&self, id: &str) -> Result<()> {
        let user_id = self.get(id)?.user_id;
        self.connection(id)?
            .users()
            .delete_totp(DeleteTotpRequest {
                user_id: Some(user_id),
                ..Default::default()
            })
            .await?;
        Ok(())
    }

    /// Deletes one of the user's passkeys.
    pub async fn delete_passkey(&self, id: &str, passkey_id: &str) -> Result<()> {
        let user_id = self.get(id)?.user_id;
        self.connection(id)?
            .users()
            .delete_passkey(DeletePasskeyRequest {
                user_id: Some(user_id),
                id: Some(passkey_id.to_owned()),
                ..Default::default()
            })
            .await?;
        Ok(())
    }

    /// The user's devices signed in to a server.
    pub async fn devices(&self, id: &str) -> Result<Vec<DeviceSession>> {
        let r = self
            .connection(id)?
            .sessions()
            .list_sessions(ListSessionsRequest::default())
            .await?
            .into_owned();
        Ok(r.sessions.iter().map(DeviceSession::from_proto).collect())
    }

    /// Signs one of the user's other devices out of a server.
    pub async fn sign_out_device(&self, id: &str, session_id: &str) -> Result<()> {
        self.connection(id)?
            .sessions()
            .revoke_session(RevokeSessionRequest {
                id: Some(session_id.to_owned()),
                ..Default::default()
            })
            .await?;
        Ok(())
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn preferences_round_trip_and_default_to_the_default_track() {
        let p = PlaybackPreferences::from_proto(&PlaybackPreferencesProto::default());
        assert_eq!(p.subtitle_mode, SubtitleMode::Default);
        assert!(p.play_default_audio_track, "got = false, want = true");
        for mode in [
            SubtitleMode::Default,
            SubtitleMode::Always,
            SubtitleMode::Foreign,
            SubtitleMode::Forced,
            SubtitleMode::Never,
            SubtitleMode::Smart,
        ] {
            let p = PlaybackPreferences {
                audio_languages: vec!["jpn".into()],
                subtitle_languages: vec!["chi".into(), "eng".into()],
                subtitle_mode: mode,
                play_default_audio_track: false,
            };
            let got = PlaybackPreferences::from_proto(&p.to_proto());
            assert_eq!(got, p, "got = {got:?}, want = {p:?}");
        }
    }

    #[tokio::test]
    async fn security_and_devices_come_from_the_server_as_the_user() {
        use std::sync::{Arc, Mutex};

        use mavio_api::proto::mavio::library::v1::Item;
        use mavio_api::proto::mavio::session::v1::{
            ListSessionsResponse, NowPlaying, RevokeSessionResponse,
        };
        use mavio_api::proto::mavio::user::v1::{
            GetCurrentUserResponse, Passkey as PasskeyProto, SetPinResponse,
        };

        use crate::servers::tests::{fake_server, servers};

        let fake = fake_server("sa", "Home");
        let calls = Arc::new(Mutex::new(Vec::new()));
        fake.unary(
            "/mavio.user.v1.UserService/GetCurrentUser",
            |_, _: GetCurrentUserRequest| {
                Ok(GetCurrentUserResponse {
                    user: User {
                        id: Some("u".into()),
                        has_totp: Some(true),
                        passkeys: vec![PasskeyProto {
                            id: Some("p".into()),
                            name: Some("Safari".into()),
                            rp_id: Some("home.example".into()),
                            ..Default::default()
                        }],
                        ..Default::default()
                    }
                    .into(),
                    ..Default::default()
                })
            },
        );
        let c = calls.clone();
        fake.unary(
            "/mavio.user.v1.UserService/SetPin",
            move |_, req: SetPinRequest| {
                c.lock().unwrap().push(format!(
                    "pin {} {}",
                    req.user_id.unwrap_or_default(),
                    req.pin.unwrap_or_default()
                ));
                Ok(SetPinResponse::default())
            },
        );
        fake.unary(
            "/mavio.session.v1.SessionService/ListSessions",
            |_, req: ListSessionsRequest| {
                assert_eq!(req.all_users, None, "only the user's own");
                Ok(ListSessionsResponse {
                    sessions: vec![
                        SessionProto {
                            id: Some("s1".into()),
                            device_name: Some("Phone".into()),
                            current: Some(true),
                            ..Default::default()
                        },
                        SessionProto {
                            id: Some("s2".into()),
                            device_name: Some("TV".into()),
                            online: Some(true),
                            now_playing: NowPlaying {
                                item: Item {
                                    name: Some("Brazil".into()),
                                    ..Default::default()
                                }
                                .into(),
                                ..Default::default()
                            }
                            .into(),
                            ..Default::default()
                        },
                    ],
                    ..Default::default()
                })
            },
        );
        let c = calls.clone();
        fake.unary(
            "/mavio.session.v1.SessionService/RevokeSession",
            move |_, req: RevokeSessionRequest| {
                c.lock()
                    .unwrap()
                    .push(format!("revoke {}", req.id.unwrap_or_default()));
                Ok(RevokeSessionResponse::default())
            },
        );
        let address = fake.serve().await;
        let (s, _) = servers();
        s.sign_in(&address, "me", "secret").await.unwrap();

        let got = s.security("sa").await.unwrap();
        assert!(
            got.has_password && got.has_totp && !got.has_pin,
            "got = {got:?}"
        );
        assert_eq!(got.passkeys[0].host, "home.example");
        let got = s.devices("sa").await.unwrap();
        let got: Vec<_> = got
            .iter()
            .map(|d| (d.device_name.as_str(), d.current, d.playing.as_deref()))
            .collect();
        assert_eq!(got, [("Phone", true, None), ("TV", false, Some("Brazil"))]);

        s.set_pin("sa", "1234").await.unwrap();
        s.sign_out_device("sa", "s2").await.unwrap();
        assert_eq!(*calls.lock().unwrap(), ["pin u 1234", "revoke s2"]);
    }
}
