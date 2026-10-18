//! The settings the servers keep for the user: the playback preferences
//! the user's devices share, and the users a session on a standalone server
//! switches between (`docs/accounts.md` §4.4).

use mavio_api::proto::mavio::auth::v1::SwitchableUser as SwitchableUserProto;
use mavio_api::proto::mavio::user::v1::{
    PlaybackPreferences as PlaybackPreferencesProto, SubtitleMode as SubtitleModeProto,
};

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
}
