#!/bin/sh
# Makes the app's icon from the design system's (libs/design/brand): the
# Icon Composer document macOS shows, the mark on white or, in the dark, on
# black, and the PNG of the light icon, which Windows and Linux show. Run it
# on macOS after the brand's icon changes.
# Usage: scripts/icon.sh
set -eu
cd "$(dirname "$0")/.."
brand=../../libs/design/brand
cp "$brand/mark.svg" icon/AppIcon.icon/Assets/mark.svg
swift scripts/icon.swift "$brand/app-icon-light.svg" icon/icon.png 1024
