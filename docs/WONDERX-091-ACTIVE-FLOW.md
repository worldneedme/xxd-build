# Active WONDERX V1 consumer UI and direct-start delivery

## Fixed inputs
Source repository: PRIVATE worldneedme/xxdmanfl0609.
Branch: codex/flclash-0.9.1-complete-20261021.
Candidate source SHA: 6c5c710f59273b40801b067dc9b4cf9cfa4892e1.
Workflow SHA: 067cb0cea5731be43b1e38b4ce007658f5acd84f.
Run: https://github.com/worldneedme/xxd-build/actions/runs/39658285587.
Planned release tag: wonderx-v1-0.9.1-consumer-ui-direct-start-20261021-1903.

## Approved scope
V1 tray/app/header identity; consumer dashboard; node name + runtime; expanded gold sidebar motto.
Remove homepage type/quality/org/ASN and expert modules without removing their optional tools.
One-time mobile card migration, retaining profile/credentials/settings.
Added at user request: remove first-launch disclaimer + typed restatement gates; direct entry and subscription import.
Keep legal text read-only in Tools, no fabricated acceptance. OS permissions and crash/cache recovery preserved.
No core/protocol changes: Mieru, Juicity/Hop, NaiveProxy, Sudoku and selection/subscription logic retained.

## Stages
IMPLEMENTATION: candidate committed.
HOSTED_GENERATION: PASSED run 39657947891.
STATIC_AND_FIVE_ROUNDS: RUNNING run 39658285587.
WINDOWS_EXE_ZIP_AND_ANDROID_APK: waits for the five-round gates.
PAIRED_RELEASE: NOT_STARTED; old release will remain unchanged.
DEVICE_TESTS: PENDING_DEVICE_TEST.
PRIVATE_RESTORE_AND_WRITER_CLEANUP: required at completion.

## Corrections and superseded runs
39655618408: cancelled before generation for inline sidebar slot argument correction.
39655732620: passed generation.
39656275507: static FAILED (missing internal sidebar field, unused enum import, test Config themeProps);
  corrected, no tests weakened; native checks PASSED.
39656632838: passed regeneration.
39657033978: static and native PASSED, full Flutter round 1 was CANCELLED on user-added direct-start request;
  no failed UI assertion in available output, no package published.
39657947891: passed generation of latest combined candidate.
No earlier incomplete run is counted as one of the five successful final rounds.

## Credential and repository ledger
Only xxd-build temporarily PUBLIC under user authorization; source remains PRIVATE.
Temporary normalization writer deploy key ID 167221912 and SOURCE_WRITE_KEY must be removed.
Preserve SOURCE_READ_KEY, DANGERX_READ_KEY, HKDEV_TEST_SUBSCRIPTION_URL and existing read-only deploy keys.
No VPS build/test/dependencies/artifact downloads; GitHub-hosted runners only.
No server/panel/DNS/firewall/production changes.

## Previous verified rollback
Delivered source c34637e714bbf3bdd429043de4f69d4ac18b8b2b.
Run https://github.com/worldneedme/xxd-build/actions/runs/39637446866 PASSED.
Release https://github.com/worldneedme/xxd-build/releases/tag/wonderx-v1-0.9.1-official-ui-20261021-1455.
EXE SHA256 59359605ed0ced361127cbcd442e461a014aa3d69a0bca88b16b63d12afbaaa6.
ZIP SHA256 595990bd84befef305093775e87b4befa010ee1636d55d9b729598296df7b31c.
APK SHA256 47461f36424da598dade2b0dc54dc74cb5122b156d1cbba8e516b33d7ed2282c.
Its temporary writer 167136428 was removed and build PRIVATE restored after that delivery.

## Important limitation
Android original signing secrets are absent: retain existing .dev test signing.
Do not promise overwrite compatibility or actual user-device connectivity from CI.


## 2026-10-21 19:09 UTC — concise multilingual subscription reminder
User superseded the long reminder with concise, warmer text: 订阅常更新 / 连接更顺心.
Seven source ARB locales updated. Runtime retained; former organization slot is static reminder only.
Source patch: d2671dff4e4fbc1c7a2c964555341562e5711102.
Workflow: 3c395b9e449bac1995956bbe2aacf38995f57652 (adds hosted intl_utils generation).
Normalization run: https://github.com/worldneedme/xxd-build/actions/runs/39659152638 — RUNNING.
Previous run 39658285587 FAILED static analysis: obsolete mandatory-consent tests and missing config import.
Those tests now cover read-only legal navigation/scrolling/back on desktop, phone and remote; unrelated dialogs retained.
Five full successful final-source rounds and packages remain NOT_STARTED; no new release delivered yet.
Temporary writer/secret cleanup and xxd-build PRIVATE restoration remain mandatory at completion.

Normalization 39659152638 PASSED; final generated source baaacf5d0d24990fe6e7b4e8bde4ae3feea16428.
Full five-round regression + paired package run: https://github.com/worldneedme/xxd-build/actions/runs/39659572718 — RUNNING.
Workflow run HEAD94ece9c2899817d7cc14ac1dead452512144f5c1; functional workflow3c395b9e449bac1995956bbe2aacf38995f57652.
Intended tag: wonderx-v1-0.9.1-consumer-ui-direct-start-20261021-1913 (NOT YET DELIVERED).

Full run39659572718 FAILED in Flutter round1 (4300 passed,4 failed,6 skipped); native PASSED.
Failures: optional legal page test, V1 header drag test, optional connections table sort/resize tests.
Packaging/publish SKIPPED. Expanded focused diagnostics run39660700593; source unchanged baaacf5d.
Do not count failed round as completed regression. Next: use actual exception details, repair minimal setup/behavior, normalize if source changes, restart all five rounds.


## 2026-10-21 19:25 UTC — Windows installer/shortcut naming addition
User requires installer display WONDERX V1 and desktop shortcut V1.
Source patch2c2334cf23a3277cee3acc6771b73e0963fa1b8c includes:
- AppId/executable/app-data unchanged; default desktop shortcut V1, explicit uninstall display WONDERX V1.
- Legacy WONDERX V1.lnk deleted only if its target equals this installation executable.
- Source packaging identity regression retained alongside actual Windows install smoke.
- Consumer test hosts: legal ProviderScope; direct optional ConnectionsTableCard; settled brand drag.
Normalization run39661357336 RUNNING; not yet delivered.
Workflow942099f26444565d2e6ea3c663bebd872ba1809d adds actual registry/V1.lnk/legacy-cleanup checks and scoped smoke cleanup.
Previous diagnostic39660700593 FAILED; native passed; package jobs skipped.
No production changes; source PRIVATE; build temporarily PUBLIC until delivery/cleanup.

Normalization39661357336 PASSED. Generated final candidate0b6781447ed133ac9967cacc52bd628001fb4c51.
Five-round unified run https://github.com/worldneedme/xxd-build/actions/runs/39661857289 RUNNING (2026-10-21T19:27:53Z).
Workflow run SHAa93b2f951f99c723a43f7d9edcd2d049e103cb84, functional942099f26444565d2e6ea3c663bebd872ba1809d.
Tagwonderx-v1-0.9.1-consumer-ui-v1-installer-20261021-1928 intended, not published yet.
Next: focused regression→five full rounds→paired Windows/Android checks→release→writer revocation/PRIVATE restoration.
