// Package organize names movie and episode files and their folders,
// ported from the file name builders of Radarr and Sonarr.
//
// A Naming configuration holds the movie formats, whose {Token}
// placeholders BuildFileName and MovieFolder fill from a Movie and a File;
// a SeriesNaming configuration holds the episode, series folder and season
// folder formats, which BuildEpisodeFileName, SeriesFolder and SeasonFolder
// fill from a Series, its Episodes and an EpisodeFile. The case of a
// token's name decides the case of its value, the separator inside a token
// replaces the spaces of its value, and prefix and suffix characters inside
// the braces appear only when the value is not empty. Series formats also
// write "{{" and "}}" as literal braces.
//
// The port differs from Radarr and Sonarr where they depend on .NET or on
// services this package does not have:
//
//   - Formats are split at "/" and "\", and the components are joined with
//     "/" whatever the platform; empty components are dropped. Sonarr keeps
//     a "\" in a series or season folder format as part of the name.
//   - Token truncation such as {Series Title:16} counts characters, not
//     UTF-16 code units. Sonarr's episode title fitting counts UTF-8 bytes
//     and is ported as such, against Linux limits of 255 bytes per file
//     name and 4096 per path.
//   - Number formats such as {season:00} support only "0" and "#"
//     placeholders, which is all Sonarr's number patterns allow; any other
//     .NET numeric format writes the plain number.
//   - An unknown quality writes nothing for every quality token, where
//     Sonarr would write the proper and real labels alone.
//   - Media information and custom formats are inputs: neither builder
//     refreshes stale media information or computes custom formats.
//   - An absolute episode number of zero means unknown, and an episode file
//     already in the library is marked by EpisodeFile.Imported rather than
//     by a database ID.
//   - Case mapping is culture-invariant.
package organize
