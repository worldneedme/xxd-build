# Active WONDERX 0.9.1 flow

## Prior delivery completed
- Source c34637e714bbf3bdd429043de4f69d4ac18b8b2b.
- Workflow 83bb8315a2f1e3c8838ab181f1b06509593a6def.
- PASSED run: https://github.com/worldneedme/xxd-build/actions/runs/39637446866.
- Release: https://github.com/worldneedme/xxd-build/releases/tag/wonderx-v1-0.9.1-official-ui-20261021-1455.
- EXE SHA256 59359605ed0ced361127cbcd442e461a014aa3d69a0bca88b16b63d12afbaaa6.
- ZIP SHA256 595990bd84befef305093775e87b4befa010ee1636d55d9b729598296df7b31c.
- APK SHA256 47461f36424da598dade2b0dc54dc74cb5122b156d1cbba8e516b33d7ed2282c.
- Both platform jobs and paired publish passed, including five regression rounds and actual Windows installer/Core RPC validation.
- Build restored PRIVATE; temporary writer key 167136428 and SOURCE_WRITE_KEY removed. Read-only keys kept.

## Current authorized follow-up
Consumer UI and V1 branding only. No core/protocol changes.
Source candidate: 13236635f76b7ff4e012243c2ff2c1678db085c2.
Detailed source HANDOVER: docs/WONDERX-091-CONSUMER-UI-HANDOVER.md.
Five full regression rounds must pass on the generated immutable source before packaging.
Windows EXE/ZIP and Android arm64 APK must publish together under a NEW tag.
Original Android signing secret absent: existing .dev test signing, device acceptance pending.
All runs use GitHub-hosted runners; no local build/test/dependency/artifact activity.

## Current stages
Implementation: candidate committed.
Hosted generation: RUNNING — https://github.com/worldneedme/xxd-build/actions/runs/39655732620.
Temporary source writer deploy key: 167221912 (remove after hosted generation/delivery).
Earlier normalization 39655618408 was cancelled before generation to include the inline footer argument correction.
Five-round regression: NOT_STARTED.
Paired packaging and publish: NOT_STARTED.
PRIVATE restoration and temporary-key cleanup: required after delivery.
