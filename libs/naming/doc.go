// Package naming parses media file and folder names into structured
// information: movies with their year, stacks of parts and alternate
// versions, extras, 3D formats and stubs; episodes, seasons and series;
// multi-disc albums; audiobooks; books; and the flags of external subtitle,
// audio and lyric files. It is a pure library with no I/O.
//
// The rules are Jellyfin's (DefaultOptions) and keep their .NET regular
// expression syntax, run by github.com/dlclark/regexp2. A Parser compiles
// a set of rules once and is safe for concurrent use.
package naming
