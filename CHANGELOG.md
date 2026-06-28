# Changelog

All notable changes to this project are documented in this file.

## [0.2.0] - 2026-06-28

### Added

- Group and supergroup chat support — groups and supergroups are now listed alongside private chats.
- Channel support — broadcast channels appear in the chat list.
- Bot chat support — bot conversations are listed and accessible.
- Chat type tags: `[G]` for groups, `[C]` for channels, `[B]` for bots.
- Pinned tag `[PIN]` combined with type tags: e.g. `[PIN] [G]`.
- Per-type color gradient for tags that fade to gray with list position.
- Chat list gradient uses terminal palette colors (adapts to system/terminal theme changes).
- Smooth interpolation between palette anchor stops for gradients.
- Folder filter matching extended to groups and channels (Telegram folder flags: Groups, Broadcasts).
- Duplicate dialog deduplication in both adapter and UI layers.
- Layout stability fix: deep-scroll overlap/shift no longer occurs due to problematic Unicode characters.

### Changed

- Chat list no longer restricted to private chats — all dialog types are shown.
- Stable collision-free chat IDs across peer types (users, groups, channels).
- `oneLine` text normalization now strips combining marks and non-single-width glyphs to prevent layout shift.
- Panel row rendering uses a strict single-line fitter (`fitPanelLine`) to prevent content overflow.

## [0.1.0] - 2026-06-28

### Added

- Initial terminal Telegram client implementation based on Bubble Tea and gotd/td.
- Auth flows for phone/code, optional 2FA password, and QR login.
- Private chat list loading and rendering.
- Message history loading and pagination.
- Outgoing message send flow.
- Reply target selection and reply metadata rendering in history.
- Periodic refresh for chats and active conversation.
- Local persistence for Telegram session and bbolt cache.
- Use-case and ports refactor toward clean architecture boundaries.
- Focused unit tests for use-case policy modules.

### Changed

- Message selection persistence across periodic refreshes.
- Keyboard model: up/down for scroll, ctrl+up/down for message selection.
- Split right pane into message history and dedicated compose panel.
- Chat-list readability improvements including separators and gradient styling.
- Timeout-bound contexts for UI-triggered remote operations.

### Security

- Hardened local data handling with restrictive filesystem permissions.
- Symlink and file-type checks when preparing cache/session/log files.
- Strong random message ID generation for outgoing Telegram requests.
- Safer close-path handling for update channel lifecycle.
