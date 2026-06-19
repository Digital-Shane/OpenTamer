# Manage a disposable CPU-heavy process for QA.
qa action:
  @{{ \
    if action == "launch" { \
      "nohup yes OpenTamerQACPU >/dev/null 2>&1 & echo $!" \
    } else if action == "kill" { \
      "pkill -f '^yes'" \
    } else { \
      error("Usage: just qa launch|kill") \
    } \
  }}

# Build the macOS app bundle.
build:
  #!/usr/bin/env bash
  set -euo pipefail

  app="build/OpenTamer.app"
  contents="$app/Contents"
  codesign_identity="${OPENTAMER_CODESIGN_IDENTITY:--}"
  
  rm -rf "$app"
  mkdir -p "$contents/MacOS" "$contents/Resources"
  cp packaging/Info.plist "$contents/Info.plist"
  cp assets/opentamer-icon.icns "$contents/Resources/opentamer-icon.icns"
  export CGO_LDFLAGS="${CGO_LDFLAGS:-} -Wl,-no_warn_duplicate_libraries"
  go build -trimpath -o "$contents/MacOS/OpenTamer" ./cmd/opentamer
  if [ "$codesign_identity" = "-" ]; then
    codesign --force --sign "$codesign_identity" "$app"
  else
    codesign --force --timestamp --options runtime --sign "$codesign_identity" "$app"
  fi
  printf '%s\n' "{{ justfile_directory() }}/$app"

run: build
  pkill -f 'OpenTamer.app' || true
  sleep 0.25 # Opening too quickly after build makes macOS angry
  open build/OpenTamer.app
