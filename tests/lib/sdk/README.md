# Test SDKs

These SDKs are used in the `main` end-to-end tests
and the SDK Store integration tests.
They are intended to be immutable,
to keep the old tests working
while updated ones are under development.

New SDKs can be added by placing them in `tests/lib/staging`.
A GitHub workflow will build and publish them before running integration tests.
The `build-for` architectures can be customized by listing them in `tests/lib/staging/<NAME>/platforms.json`.
By default only `amd64` is built.

After finalizing the SDK,
it should be moved to `tests/lib/sdk`.
A separate workflow prevents merging a PR with staged SDKs,
but no automation touches `tests/lib/sdk`.
It serves purely as a record of which SDKs are used in the tests.

Test SDKs can be built manually,
but ideally they should be published by an independent entity.
In any case the SDK definition should be stored in `tests/lib/sdk`.

An SDK can be removed at any time,
but this should only happen after all tests no longer use it.
