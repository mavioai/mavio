#!/usr/bin/env bash
# Generates a small library for trying playback with the development
# player: movies that a browser plays directly, remuxed, with subtitles,
# several audio tracks or transcoded, and a series, with posters named in
# NFO files. Usage: dev-library.sh [dir], default .fixtures/dev-library.
# Serve it with: mavio -dev -dev-library <dir>
set -eu
L=${1:-$(cd "$(dirname "$0")/../.." && pwd)/.fixtures/dev-library}; M="$L/Movies"; T="$L/Shows"
rm -rf "$L"; mkdir -p "$M" "$T/Test Show/Season 01"
F="-hide_banner -loglevel error -y"
src() { echo "-f lavfi -i testsrc2=size=1280x720:rate=24 -f lavfi -i sine=frequency=$1:sample_rate=48000"; }
srt() { printf '1\n00:00:02,000 --> 00:00:06,000\n%s: first line\n\n2\n00:00:30,000 --> 00:00:35,000\n%s: thirty seconds\n\n3\n00:01:30,000 --> 00:01:35,000\n%s: ninety seconds\n' "$1" "$1" "$1"; }
poster() {
  ffmpeg $F -f lavfi -i "testsrc2=size=400x600,hue=h=$2" -frames:v 1 "$1/poster.jpg"
  root=movie; [ "${3:-}" = show ] && root=tvshow
  nfo="$1/$(basename "$1").nfo"; [ "$root" = tvshow ] && nfo="$1/tvshow.nfo"
  printf '<?xml version="1.0" encoding="UTF-8"?>\n<%s><title>%s</title><plot>Mavio test media.</plot><thumb aspect="poster">%s/poster.jpg</thumb></%s>\n' "$root" "$(basename "$1" | sed 's/ ([0-9]*)$//')" "$1" "$root" > "$nfo"
}
has() { ffmpeg -hide_banner -encoders 2>/dev/null | grep -q " $1 "; }
TMP=$(mktemp -d); srt English > $TMP/en.srt; srt 中文 > $TMP/zh.srt

d="$M/Direct Play (2024)"; mkdir -p "$d"
ffmpeg $F $(src 440) -t 180 -c:v libx264 -preset veryfast -g 48 -c:a aac -b:a 128k -movflags +faststart "$d/Direct Play (2024).mp4"
poster "$d" 0

d="$M/Remux With Subtitles (2024)"; mkdir -p "$d"
ffmpeg $F $(src 523) -i $TMP/en.srt -i $TMP/zh.srt -t 180 -map 0:v -map 1:a -map 2:s -map 3:s \
  -c:v libx264 -preset veryfast -g 48 -c:a aac -ac 2 -c:s srt \
  -metadata:s:s:0 language=eng -metadata:s:s:1 language=chi -disposition:s:0 default "$d/Remux With Subtitles (2024).mkv"
poster "$d" 60

d="$M/Multiple Audio (2024)"; mkdir -p "$d"
ffmpeg $F $(src 330) -f lavfi -i sine=frequency=660:sample_rate=48000 -t 120 -map 0:v -map 1:a -map 2:a \
  -c:v libx264 -preset veryfast -g 48 -c:a:0 aac -c:a:1 ac3 -ac:a:1 6 \
  -metadata:s:a:0 language=eng -metadata:s:a:0 title="English Stereo" -metadata:s:a:1 language=jpn -metadata:s:a:1 title="Japanese 5.1" \
  "$d/Multiple Audio (2024).mkv"
poster "$d" 120

if has libx265; then
d="$M/HEVC HDR10 (2024)"; mkdir -p "$d"
ffmpeg $F $(src 392) -t 120 -pix_fmt yuv420p10le -c:v libx265 -preset ultrafast \
  -x265-params "log-level=error:keyint=48:hdr10=1:colorprim=bt2020:transfer=smpte2084:colormatrix=bt2020nc:master-display=G(13250,34500)B(7500,3000)R(34000,16000)WP(15635,16450)L(10000000,50):max-cll=1000,400" \
  -color_primaries bt2020 -color_trc smpte2084 -colorspace bt2020nc -c:a aac "$d/HEVC HDR10 (2024).mkv"
poster "$d" 180
else echo "libx265 missing: skipping HEVC HDR10" >&2; fi

if has libsvtav1; then
d="$M/AV1 Opus (2024)"; mkdir -p "$d"
SVT_LOG=1 ffmpeg $F $(src 494) -t 90 -c:v libsvtav1 -preset 12 -g 48 -c:a libopus -b:a 96k "$d/AV1 Opus (2024).webm"
poster "$d" 240
else echo "libsvtav1 missing: skipping AV1 Opus" >&2; fi

for e in 1 2; do
  ffmpeg $F $(src $((300 + e * 100))) -i $TMP/en.srt -t 90 -map 0:v -map 1:a -map 2:s -c:v libx264 -preset veryfast -g 48 -c:a aac -c:s srt \
    -metadata:s:s:0 language=eng "$T/Test Show/Season 01/Test Show S01E0$e.mkv"
done
poster "$T/Test Show" 300 show
rm -rf "$TMP"
echo "Generated $L"
