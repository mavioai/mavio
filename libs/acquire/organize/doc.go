// Package organize names movie files and folders, ported from Radarr's
// file name builder: a Naming configuration holds the folder and file
// formats, whose {Token} placeholders BuildFileName and MovieFolder fill
// from a Movie and a File. The case of a token's name decides the case of
// its value, the separator inside a token replaces the spaces of its
// value, and prefix and suffix characters inside the braces appear only
// when the value is not empty.
package organize
