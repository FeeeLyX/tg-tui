# Changelog

All notable changes to this project are documented in this file.

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
