# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## Release Policy

- `CHANGELOG.md` is the source of truth for release versions.
- `packaging/Info.plist` `CFBundleShortVersionString` must match the latest released changelog version without the leading `v`.
- `packaging/Info.plist` `CFBundleVersion` is the monotonically increasing Apple build number for distributable app bundles.

## Unreleased

## [v1.5.0] - 2026-11-14
### Added
- Global CPU Limit with presets and custom budgets from 1% to 100% of total machine CPU capacity
  - Preserves stricter per-app limits.
  - Global CPU limiting excludes macOS protected processes.
### Fixed
- Group instances of the same app without losing processes when Aggregate By Name is disabled.

## [v1.4.1] - 2026-09-12
### Fixed
- Set the build deployment target to macOS 14.0.

## [v1.4.0] - 2026-09-06
### Added
- Background only CPU limiting, with `In Background` and `Always` modes for preset and custom limits.

## [v1.3.0] - 2026-06-30
### Added
- Persisted theme preference with default/backfill behavior (Thanks @kerem0x !)
### Updated
- git ignore to exclude built opentamer binaries.

## [v1.2.0] - 2026-06-20
### Added
- Ability to launch OpenTamer on login.


## [v1.1.0] - 2026-06-19
### Added
- Preference to swap between per-process and system normalized CPU usage display.


## [v1.0.0] - 2026-06-19
### Released
- My open source macOS menu bar utility for controlling app CPU usage.
