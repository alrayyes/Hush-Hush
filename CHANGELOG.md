# Changelog

## [0.13.0](https://github.com/alrayyes/hush-hush/compare/v0.12.1...v0.13.0) (2026-08-30)


### Features

* **ci:** upload coverage to Codecov ([#93](https://github.com/alrayyes/hush-hush/issues/93)) ([0793e8c](https://github.com/alrayyes/hush-hush/commit/0793e8ca756aedb092af574820347de21c053460))

## [0.12.1](https://github.com/alrayyes/hush-hush/compare/v0.12.0...v0.12.1) (2026-08-30)


### Performance Improvements

* **release:** copy a precompiled binary instead of building in Docker ([#90](https://github.com/alrayyes/hush-hush/issues/90)) ([87b4a71](https://github.com/alrayyes/hush-hush/commit/87b4a71ff5f5da2c67dff339d45f287f63e9c8ac))

## [0.12.0](https://github.com/alrayyes/hush-hush/compare/v0.11.0...v0.12.0) (2026-08-30)


### Features

* implement the CLI delete command ([#82](https://github.com/alrayyes/hush-hush/issues/82)) ([dfcc373](https://github.com/alrayyes/hush-hush/commit/dfcc373820dd585a31bb5bd6f4ff6120ec974e31)), closes [#14](https://github.com/alrayyes/hush-hush/issues/14)
* implement the CLI update command ([#80](https://github.com/alrayyes/hush-hush/issues/80)) ([7ce7156](https://github.com/alrayyes/hush-hush/commit/7ce715686cd6e21ad85a6e7ac9861ed3093297af)), closes [#13](https://github.com/alrayyes/hush-hush/issues/13)
* use the hush-hush-go SDK as the CLI's transport ([#86](https://github.com/alrayyes/hush-hush/issues/86)) ([294f62c](https://github.com/alrayyes/hush-hush/commit/294f62ca4e40a64d4ba10e8dfab89d0a06b47237))

## [0.11.0](https://github.com/alrayyes/hush-hush/compare/v0.10.0...v0.11.0) (2026-08-28)


### Features

* add the HTTP client and CLI-server Pact contract tests ([#68](https://github.com/alrayyes/hush-hush/issues/68)) ([6cfdcd3](https://github.com/alrayyes/hush-hush/commit/6cfdcd3c607e3931d6fae7b9dc1d230fea956919))
* implement the CLI get command ([#75](https://github.com/alrayyes/hush-hush/issues/75)) ([8794ac1](https://github.com/alrayyes/hush-hush/commit/8794ac1e19570ba4d5e3eb33687e38026dab00d5))
* implement the CLI inject command ([#70](https://github.com/alrayyes/hush-hush/issues/70)) ([cef3460](https://github.com/alrayyes/hush-hush/commit/cef34608830de30e192b2ee73f7d75af041a0144))

## [0.10.0](https://github.com/alrayyes/hush-hush/compare/v0.9.0...v0.10.0) (2026-08-28)


### Features

* build and publish a container image on release ([#66](https://github.com/alrayyes/hush-hush/issues/66)) ([5764284](https://github.com/alrayyes/hush-hush/commit/5764284354d877536f09c0692742a73f309db0c9))

## [0.9.0](https://github.com/alrayyes/hush-hush/compare/v0.8.0...v0.9.0) (2026-08-28)


### Features

* implement the audit log query endpoint ([#64](https://github.com/alrayyes/hush-hush/issues/64)) ([ee6bbfa](https://github.com/alrayyes/hush-hush/commit/ee6bbfa08deb1a3b0c58a837541f254a401986a3))

## [0.8.0](https://github.com/alrayyes/hush-hush/compare/v0.7.0...v0.8.0) (2026-08-28)


### Features

* record an audit log entry for every create, read, update, and delete ([#62](https://github.com/alrayyes/hush-hush/issues/62)) ([7009bb4](https://github.com/alrayyes/hush-hush/commit/7009bb4b1a82fae1c43a7211c8849036925458a8))

## [0.7.0](https://github.com/alrayyes/hush-hush/compare/v0.6.0...v0.7.0) (2026-08-28)


### Features

* add an optional X-Caller header for audit log attribution ([#57](https://github.com/alrayyes/hush-hush/issues/57)) ([9eaf2a6](https://github.com/alrayyes/hush-hush/commit/9eaf2a6b4be3bc8dfe560b23cf950b1e0699cfc6))

## [0.6.0](https://github.com/alrayyes/hush-hush/compare/v0.5.0...v0.6.0) (2026-08-28)


### Features

* implement the delete endpoint ([#55](https://github.com/alrayyes/hush-hush/issues/55)) ([8457e36](https://github.com/alrayyes/hush-hush/commit/8457e362c690c40a4ae658d2139c3d829494f814))

## [0.5.0](https://github.com/alrayyes/hush-hush/compare/v0.4.0...v0.5.0) (2026-08-28)


### Features

* implement the update endpoint ([#54](https://github.com/alrayyes/hush-hush/issues/54)) ([a674dbb](https://github.com/alrayyes/hush-hush/commit/a674dbb764373a59980e5ba28370ae946f857cea))
* implement used_by lineage query ([#52](https://github.com/alrayyes/hush-hush/issues/52)) ([6167ab3](https://github.com/alrayyes/hush-hush/commit/6167ab328c73963168ecaf30b0e5929198b1b525))

## [0.4.0](https://github.com/alrayyes/hush-hush/compare/v0.3.0...v0.4.0) (2026-08-28)


### Features

* log every rejected and failed request, as structured JSON ([#49](https://github.com/alrayyes/hush-hush/issues/49)) ([2d8b6ab](https://github.com/alrayyes/hush-hush/commit/2d8b6ab63a13706ceac2d368360dc924d7651ef7))

## [0.3.0](https://github.com/alrayyes/hush-hush/compare/v0.2.0...v0.3.0) (2026-08-28)


### Features

* implement the get endpoint ([#46](https://github.com/alrayyes/hush-hush/issues/46)) ([9f110f7](https://github.com/alrayyes/hush-hush/commit/9f110f7a6f33c4f2b3f25510e46adeb28d43403f))


### Bug Fixes

* log a handler's 500s instead of swallowing the cause ([#48](https://github.com/alrayyes/hush-hush/issues/48)) ([59d68ba](https://github.com/alrayyes/hush-hush/commit/59d68bab321851ea225bbcca5c1987dbf3347a39))

## [0.2.0](https://github.com/alrayyes/hush-hush/compare/v0.1.0...v0.2.0) (2026-08-28)


### Features

* implement the create endpoint ([#36](https://github.com/alrayyes/hush-hush/issues/36)) ([8c64818](https://github.com/alrayyes/hush-hush/commit/8c64818884f54abfb3103a5735b5c88b2c6e29f9))

## [0.1.0](https://github.com/alrayyes/hush-hush/compare/v0.0.1...v0.1.0) (2026-08-28)


### Features

* add OpenAPI spec for the secret-objects, audit-log, and cli capabilities ([#23](https://github.com/alrayyes/hush-hush/issues/23)) ([d93c355](https://github.com/alrayyes/hush-hush/commit/d93c355cb9fb7cfbd87080bff9cb9e0ac6bd9e81))
* add SQLite schema for objects, used_by, and audit log ([#25](https://github.com/alrayyes/hush-hush/issues/25)) ([4e7a57a](https://github.com/alrayyes/hush-hush/commit/4e7a57a001418dae9cc7ca445b8e999ab30f204e))

## 0.0.1 (2026-08-28)


### Bug Fixes

* dockerfile job doesn't need safe.directory, vale/ltex vocabulary ([2002c49](https://github.com/alrayyes/hush-hush/commit/2002c49d1906aefdd088cf239d0852dad092d02e))
* git safe.directory for root-run containers in CI ([9d7d5c8](https://github.com/alrayyes/hush-hush/commit/9d7d5c8eaddb00fcf35c2aae3cdadb730d4681d8))
* git safe.directory for root-run containers in CI ([be27241](https://github.com/alrayyes/hush-hush/commit/be272412253e5c426de4a91b8bd6cf7c849cbb9d))
