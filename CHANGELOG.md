## [1.23.2](https://github.com/posit-dev/team-operator/compare/v1.23.1...v1.23.2) (2026-04-17)


### Bug Fixes

* add /mnt/load-balancer to XDG_CONFIG_DIRS so Workbench reads load-balancer config ([#129](https://github.com/posit-dev/team-operator/issues/129)) ([44140f0](https://github.com/posit-dev/team-operator/commit/44140f09ffffa2f5678c24b742381e01b0c950a1))

## [1.23.1](https://github.com/posit-dev/team-operator/compare/v1.23.0...v1.23.1) (2026-04-16)


### Bug Fixes

* prevent status-patch reconcile storm in Site and child controllers ([#128](https://github.com/posit-dev/team-operator/issues/128)) ([72fdeeb](https://github.com/posit-dev/team-operator/commit/72fdeeb9f4f354a10037aac403b68c22adc2715a))


### Reverts

* remove dynamicLabels from session pod config ([#113](https://github.com/posit-dev/team-operator/issues/113)) ([#126](https://github.com/posit-dev/team-operator/issues/126)) ([74058d3](https://github.com/posit-dev/team-operator/commit/74058d3805f15331f986db6fe6653678643ab0a5))

# [1.23.0](https://github.com/posit-dev/team-operator/compare/v1.22.0...v1.23.0) (2026-04-14)


### Bug Fixes

* only tag docker release after corresponding image is pushed ([#122](https://github.com/posit-dev/team-operator/issues/122)) ([c1532c0](https://github.com/posit-dev/team-operator/commit/c1532c038b5a95bd16ad34b53c7331744e6071c3))


### Features

* add session label controller for Workbench session pods ([#123](https://github.com/posit-dev/team-operator/issues/123)) ([f0f8146](https://github.com/posit-dev/team-operator/commit/f0f8146a98e69b954fe58a96c06251cffa28bf2e))

# [1.22.0](https://github.com/posit-dev/team-operator/compare/v1.21.1...v1.22.0) (2026-04-01)


### Features

* add optional SCIM provisioning support for Workbench ([#120](https://github.com/posit-dev/team-operator/issues/120)) ([531e45f](https://github.com/posit-dev/team-operator/commit/531e45f96da52eacb397082ca516de18c993a580))

## [1.21.1](https://github.com/posit-dev/team-operator/compare/v1.21.0...v1.21.1) (2026-04-01)


### Bug Fixes

* prevent non-deterministic reconcile loops ([#121](https://github.com/posit-dev/team-operator/issues/121)) ([745819c](https://github.com/posit-dev/team-operator/commit/745819c3f83fbbbf69d8284bb87a1f4d813e8101))

# [1.21.0](https://github.com/posit-dev/team-operator/compare/v1.20.0...v1.21.0) (2026-03-13)


### Features

* add dynamicLabels to session pod config ([#113](https://github.com/posit-dev/team-operator/issues/113)) ([6b97627](https://github.com/posit-dev/team-operator/commit/6b97627fa4d582c87b64978984f194549629f8ea))

# [1.20.0](https://github.com/posit-dev/team-operator/compare/v1.19.0...v1.20.0) (2026-03-11)


### Features

* improve status on team operator resources ([#94](https://github.com/posit-dev/team-operator/issues/94)) ([eb3f7b0](https://github.com/posit-dev/team-operator/commit/eb3f7b0f006918cfcf292bc8e25105ff5dade65d))

# [1.19.0](https://github.com/posit-dev/team-operator/compare/v1.18.0...v1.19.0) (2026-03-10)


### Features

* operator applies own CRDs at startup to ensure schema matches binary ([#98](https://github.com/posit-dev/team-operator/issues/98)) ([ded621e](https://github.com/posit-dev/team-operator/commit/ded621ea4462e7ef1809d485b90ec94fb6bbb949))

# [1.18.0](https://github.com/posit-dev/team-operator/compare/v1.17.0...v1.18.0) (2026-03-09)


### Features

* add PPM OIDC/SSO web UI configuration ([#112](https://github.com/posit-dev/team-operator/issues/112)) ([234d796](https://github.com/posit-dev/team-operator/commit/234d79670a10093e1fea90e08ead033a0c2ec89a))

# [1.17.0](https://github.com/posit-dev/team-operator/compare/v1.16.2...v1.17.0) (2026-03-09)


### Features

* extend enable/disable/teardown pattern to Workbench, Package Manager, and Chronicle ([#99](https://github.com/posit-dev/team-operator/issues/99)) ([64fc5b2](https://github.com/posit-dev/team-operator/commit/64fc5b27d2562be1e14dc1e3088306161b949c9c))

## [1.16.2](https://github.com/posit-dev/team-operator/compare/v1.16.1...v1.16.2) (2026-03-04)


### Bug Fixes

* mismatched image tags causing ImagePullBackOff ([#108](https://github.com/posit-dev/team-operator/issues/108)) ([4181072](https://github.com/posit-dev/team-operator/commit/418107279bb0bf10168f7e6f7d65901269666b2d))

## [1.16.1](https://github.com/posit-dev/team-operator/compare/v1.16.0...v1.16.1) (2026-02-27)


### Bug Fixes

* default operator image tag to chart appVersion instead of latest ([#107](https://github.com/posit-dev/team-operator/issues/107)) ([0ee55c1](https://github.com/posit-dev/team-operator/commit/0ee55c1a37992bfa5591875b001985c7a4cef2dd))

# [1.16.0](https://github.com/posit-dev/team-operator/compare/v1.15.0...v1.16.0) (2026-02-26)


### Features

* auto-generate Edit Config page from Site CRD ([#39](https://github.com/posit-dev/team-operator/issues/39)) ([#79](https://github.com/posit-dev/team-operator/issues/79)) ([101dab1](https://github.com/posit-dev/team-operator/commit/101dab1f6e675ee0c35b0d12747ed6dc7b545677))

# [1.15.0](https://github.com/posit-dev/team-operator/compare/v1.14.0...v1.15.0) (2026-02-24)


### Features

* add local integration testing infrastructure (envtest + kind) ([#54](https://github.com/posit-dev/team-operator/issues/54)) ([a5b5751](https://github.com/posit-dev/team-operator/commit/a5b5751fec092a3d830d159b37ae557214902c89))

# [1.14.0](https://github.com/posit-dev/team-operator/compare/v1.13.0...v1.14.0) (2026-02-23)


### Features

* allow disabling Connect without data loss, add explicit teardown field ([#93](https://github.com/posit-dev/team-operator/issues/93)) ([5fab352](https://github.com/posit-dev/team-operator/commit/5fab35203c93835404e8dd8f67d455baad092ee9))

# [1.13.0](https://github.com/posit-dev/team-operator/compare/v1.12.0...v1.13.0) (2026-02-20)


### Features

* add passthrough config mechanism for Connect, Package Manager, and Workbench ([#75](https://github.com/posit-dev/team-operator/issues/75)) ([e42c2b9](https://github.com/posit-dev/team-operator/commit/e42c2b91468a891b994fa1304497ed0098fab12f))

# [1.12.0](https://github.com/posit-dev/team-operator/compare/v1.11.2...v1.12.0) (2026-02-19)


### Features

* migrate from aws-sdk-go v1 to v2 ([#50](https://github.com/posit-dev/team-operator/issues/50)) ([55708e7](https://github.com/posit-dev/team-operator/commit/55708e7526ebd9da56b70de30d2a302b29fb417c))

## [1.11.2](https://github.com/posit-dev/team-operator/compare/v1.11.1...v1.11.2) (2026-02-19)


### Bug Fixes

* use *int for audited jobs fields to preserve product defaults ([#89](https://github.com/posit-dev/team-operator/issues/89)) ([709e580](https://github.com/posit-dev/team-operator/commit/709e5800a364c0f676f4b1df443a2b4601efb66d))

## [1.11.1](https://github.com/posit-dev/team-operator/compare/v1.11.0...v1.11.1) (2026-02-19)


### Bug Fixes

* use *bool for RegisterOnFirstLogin to preserve Connect default ([#91](https://github.com/posit-dev/team-operator/issues/91)) ([ba13701](https://github.com/posit-dev/team-operator/commit/ba137014799790b9dd6a0e2b658d751ca6512a4b))

# [1.11.0](https://github.com/posit-dev/team-operator/compare/v1.10.1...v1.11.0) (2026-02-18)


### Features

* add support for disabling Connect OAuth2.RegisterOnFirstLogin ([#87](https://github.com/posit-dev/team-operator/issues/87)) ([32277d7](https://github.com/posit-dev/team-operator/commit/32277d7cc3773ddbfb9bb80309edd8524c4aa1a7))

## [1.10.1](https://github.com/posit-dev/team-operator/compare/v1.10.0...v1.10.1) (2026-02-18)


### Bug Fixes

* remove deprecated DefaultContentListView ([#84](https://github.com/posit-dev/team-operator/issues/84)) ([68f4ea5](https://github.com/posit-dev/team-operator/commit/68f4ea5c1bc3a43892e65495ad09cff5df91419b))

# [1.10.0](https://github.com/posit-dev/team-operator/compare/v1.9.0...v1.10.0) (2026-02-13)


### Features

* adds pdbs to workbench sessions ([#67](https://github.com/posit-dev/team-operator/issues/67)) ([a21e10e](https://github.com/posit-dev/team-operator/commit/a21e10ee0aea164fb35af3bba487ab186c16e332))

# [1.9.0](https://github.com/posit-dev/team-operator/compare/v1.8.1...v1.9.0) (2026-02-12)


### Features

* add audited jobs configuration support for Workbench ([#81](https://github.com/posit-dev/team-operator/issues/81)) ([78ccdf2](https://github.com/posit-dev/team-operator/commit/78ccdf21ef2fec6673fb3a3f2db65b90a1f5710a))

## [1.8.1](https://github.com/posit-dev/team-operator/compare/v1.8.0...v1.8.1) (2026-02-12)


### Bug Fixes

* **helm:** remove duplicate metrics service causing install failures ([#83](https://github.com/posit-dev/team-operator/issues/83)) ([8533b4f](https://github.com/posit-dev/team-operator/commit/8533b4f144629d357ca2d79b6d62be3d96d5ee41))

# [1.8.0](https://github.com/posit-dev/team-operator/compare/v1.7.0...v1.8.0) (2026-02-10)


### Features

* add dedicated health check endpoints for flightdeck ([#77](https://github.com/posit-dev/team-operator/issues/77)) ([484b78a](https://github.com/posit-dev/team-operator/commit/484b78adf0d2c8098fe58e0c0e2984296793a756))

# [1.7.0](https://github.com/posit-dev/team-operator/compare/v1.6.0...v1.7.0) (2026-02-09)


### Features

* add R 4.5.x default runtime image and configurable additionalRuntimeImages ([#72](https://github.com/posit-dev/team-operator/issues/72)) ([1c39720](https://github.com/posit-dev/team-operator/commit/1c39720dcefb3054852d0b20196f5b776a8dab70))

# [1.6.0](https://github.com/posit-dev/team-operator/compare/v1.5.0...v1.6.0) (2026-02-04)


### Features

* Add NetworkPolicy for flightdeck component ([#68](https://github.com/posit-dev/team-operator/issues/68)) ([fbdf600](https://github.com/posit-dev/team-operator/commit/fbdf600dd90ef143d757fefac19a1c2908e78b63))

# [1.5.0](https://github.com/posit-dev/team-operator/compare/v1.4.1...v1.5.0) (2026-02-03)


### Bug Fixes

* **rbac:** add namespace to RoleBinding for watch namespace permissions ([e0dd821](https://github.com/posit-dev/team-operator/commit/e0dd821eae7158045c421629c7b0805395679e1c))


### Features

* dispatch version update to PTD on release ([b0f99ff](https://github.com/posit-dev/team-operator/commit/b0f99ffc808c0408345a4f0acdce13a289db790d))

## [1.4.1](https://github.com/posit-dev/team-operator/compare/v1.4.0...v1.4.1) (2026-02-02)


### Bug Fixes

* comment out auth_proxy instead of deleting files ([91a2c74](https://github.com/posit-dev/team-operator/commit/91a2c744249e125387fd54eaba0e0d5c7abfbb38))
* **helm:** sync CRDs and fix helm-generate post-processing ([72657e8](https://github.com/posit-dev/team-operator/commit/72657e85a6b0e132c8e71a984586e9ca310d4fb2))
* include auth_proxy_service.yaml in commented out section ([9a10cd3](https://github.com/posit-dev/team-operator/commit/9a10cd3536333db7a632051b62d63bcb882f462d))

# [1.4.0](https://github.com/posit-dev/team-operator/compare/v1.3.2...v1.4.0) (2026-01-28)


### Features

* **flightdeck:** update UI styling to match Posit brand guidelines ([17e2a8f](https://github.com/posit-dev/team-operator/commit/17e2a8f154021ae204c3367ad1c2086ae891a158)), closes [#3276B5](https://github.com/posit-dev/team-operator/issues/3276B5)
* **workbench:** add init container for load balancer config ([4e76e8b](https://github.com/posit-dev/team-operator/commit/4e76e8b948ee5919ced1ffedc3aa3cbf8230516d))

## [1.3.2](https://github.com/posit-dev/team-operator/compare/v1.3.1...v1.3.2) (2026-01-23)


### Bug Fixes

* add just format target referenced by docs and claude agent ([479f3a0](https://github.com/posit-dev/team-operator/commit/479f3a097903f601319de8927beca35383281d20))

## [1.3.1](https://github.com/posit-dev/team-operator/compare/v1.3.0...v1.3.1) (2026-01-23)


### Bug Fixes

* correct comment for moved function ([25a522d](https://github.com/posit-dev/team-operator/commit/25a522dacbfabe113025d2df66f5256bfcc86324))
* set region for package manager and chronicle storage configs ([01e3242](https://github.com/posit-dev/team-operator/commit/01e3242006ffcbaa48e61cb82a4a883c4a0fae63))

# [1.3.0](https://github.com/posit-dev/team-operator/compare/v1.2.0...v1.3.0) (2026-01-21)


### Bug Fixes

* correct metrics service DNS name in certificate template ([d45d880](https://github.com/posit-dev/team-operator/commit/d45d8800613ac3942085777d6302e53653e0e713))
* remove duplicate metrics service and add tolerations docs ([e69c41c](https://github.com/posit-dev/team-operator/commit/e69c41cf5b7ecb4c28e04d2883b9671c64498b08))
* use templated name for metrics service ([dceea83](https://github.com/posit-dev/team-operator/commit/dceea83d53029fb40d9f9b0a70e6d7826e7eb3d3))


### Features

* add helm.sh/resource-policy: keep to all chart resources ([b55376f](https://github.com/posit-dev/team-operator/commit/b55376f6cdc50ea2a0ef62f3ad09b2383f267b6e))
* make resource-policy annotation configurable ([3ae9ada](https://github.com/posit-dev/team-operator/commit/3ae9adab9cd510ef0bf9ab8d31a809cda6cedd0a))

# [1.2.0](https://github.com/posit-dev/team-operator/compare/v1.1.0...v1.2.0) (2026-01-14)


### Bug Fixes

* add post-mutation label validation and simplify traefik signatures ([f5513a9](https://github.com/posit-dev/team-operator/commit/f5513a9314e5bd5cc1ca51ff26ee66fbf674dbf8))
* migrate FlightdeckReconciler to CreateOrUpdateResource ([585b970](https://github.com/posit-dev/team-operator/commit/585b970875018b28af8d9edc46c7f4973e2e69bc)), closes [#6](https://github.com/posit-dev/team-operator/issues/6)


### Features

* add CreateOrUpdateResource helper using controllerutil.CreateOrUpdate ([7f1ed12](https://github.com/posit-dev/team-operator/commit/7f1ed12bc7aa38bf36e544b575306bcb03797fa4))

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
