# [1.1.0](https://github.com/posit-dev/team-operator/compare/v1.0.4...v1.1.0) (2026-01-13)


### Features

* **chart:** add tolerations and nodeSelector support for controller manager ([1f7deac](https://github.com/posit-dev/team-operator/commit/1f7deacd8232ebad9177a8659da1130537c05d78))

## [1.0.4](https://github.com/posit-dev/team-operator/compare/v1.0.3...v1.0.4) (2026-01-13)


### Bug Fixes

* **chart:** remove kustomize-adopt hook that fails on tainted clusters ([424ee67](https://github.com/posit-dev/team-operator/commit/424ee6740583783306272fbb63af3fb27dc176a7))
* **ci:** add ignore-error to Docker cache for resilience ([0453603](https://github.com/posit-dev/team-operator/commit/04536030791de01a02c6556f1c9fe3abdfaaeea3))
* **ci:** grant actions write permission for GHA cache ([f9e3d3d](https://github.com/posit-dev/team-operator/commit/f9e3d3d088ab7a429c38148b8f6e9b79bb8a6f5c))
* **ci:** improve cleanup timing and reduce unnecessary releases ([b8c4515](https://github.com/posit-dev/team-operator/commit/b8c4515583549fa2a2044add0c960894ce70bd2f))
* **ci:** push to GHCR on main branch before Docker Hub ([bb31e50](https://github.com/posit-dev/team-operator/commit/bb31e507ef1ac5aa042b152c727797661d0ae375))

## [1.0.3](https://github.com/posit-dev/team-operator/compare/v1.0.2...v1.0.3) (2026-01-13)


### Bug Fixes

* **ci:** filter cleanup to only delete adhoc images matching branch ([cd2404b](https://github.com/posit-dev/team-operator/commit/cd2404b9856077a8c25e33cd174cf22bb8d34223))
* **ci:** use correct Docker Hub repository names (ptd- prefix) ([49c4b27](https://github.com/posit-dev/team-operator/commit/49c4b2712e2d1b16d49bb95d36ce97ed71c73512))

## [1.0.2](https://github.com/posit-dev/team-operator/compare/v1.0.1...v1.0.2) (2026-01-13)


### Bug Fixes

* **ci:** add Keycloak CRDs and fix flightdeck workflow ([4f4328a](https://github.com/posit-dev/team-operator/commit/4f4328a203641fcca24741ed7ea9251ec389c30a))

## [1.0.1](https://github.com/posit-dev/team-operator/compare/v1.0.0...v1.0.1) (2026-01-12)


### Bug Fixes

* **ci:** fix Helm chart packaging tag detection ([e0319cd](https://github.com/posit-dev/team-operator/commit/e0319cd9838eba64a23a262534b8d3e975e73d28))

# 1.0.0 (2026-01-12)


### Bug Fixes

* update copyright year to 2023-2026 in all source files ([acc4246](https://github.com/posit-dev/team-operator/commit/acc424698b980764368f52fb369f6b393c5a342e))
* update license in README from Apache to MIT ([22d2549](https://github.com/posit-dev/team-operator/commit/22d254949fe6279f332bb20c6f76ea24e20a35f3))


### Features

* add CI/CD workflows ([04f0bb3](https://github.com/posit-dev/team-operator/commit/04f0bb368c44f4cea80431f01d023cb0fe3f05be))
* initial migration from rstudio/ptd ([befd001](https://github.com/posit-dev/team-operator/commit/befd0010b74b0684dbb7d1da7dd0e5e8e2ad6fb8))
