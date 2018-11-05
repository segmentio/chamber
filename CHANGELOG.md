# v2.3.1 vs v2.2.0

## Features

- [[d2451a0](https://github.com/segmentio/chamber/commit//d2451a028bc4d76e94838790e638c957cfc6ffc3)] Addition of a NULL Backend to provide a simple way of disabling backend lookups (maartenvanderhoef)
- [[9c97bdf](https://github.com/segmentio/chamber/commit//9c97bdf94803016ad723a3874638108461952f39)] Add go modules (#144) (Nick Irvine)

## Analytics

- [[23f8ddb](https://github.com/segmentio/chamber/commit//23f8ddb37a10705d1b05f479399451b09b251695)] update depdencies to use working version of analytics-go (Rob McQueen)
- [[9d49cf8](https://github.com/segmentio/chamber/commit//9d49cf8221f971bd30a27f12111eb95f941f322d)] Reinstate analytics (#139) (Nick Irvine)
- [[1e7134a](https://github.com/segmentio/chamber/commit//1e7134a08e64076c4b1dcb375befc0344bf69772)] Revert "Reinstate analytics (#139)" (Rob McQueen)
- [[4b62d9e](https://github.com/segmentio/chamber/commit//4b62d9ec9fb662943194dd2022c0d69d87ce483c)] Reinstate analytics (#139) (Nick Irvine)
- [[964b9d3](https://github.com/segmentio/chamber/commit//964b9d3ca1ad63fcd925c9703e831ed35a9f6b61)] Revert "Merge pull request #136 from segmentio/systmeizer/add-analytics" (Nick Irvine)
- [[1365e40](https://github.com/segmentio/chamber/commit//1365e40e2df84362b931336ea498fc84785d70f5)] Merge pull request #136 from segmentio/systmeizer/add-analytics (Rob McQueen)
- [[47c82aa](https://github.com/segmentio/chamber/commit//47c82aa3794e8f0597efee6fce3ff5c60bacbdc6)] Adding analytics for usage metrics (Rob McQueen)

# Fixes

- [[0317aaf](https://github.com/segmentio/chamber/commit//0317aafa99c746b7948478c0148b483b9970163c)] Remove extra history entry for the current parameter version (#157) (Michael F Booth)
- [[cf28a31](https://github.com/segmentio/chamber/commit//cf28a318d02cca923e447ea19dd8ed4a3b06de1c)] Fix missing history and failed reads beyond secret version 50 (#158) (Michael F Booth)
- [[304fdba](https://github.com/segmentio/chamber/commit//304fdbaf0ed11f09b3bc8ad433fddaed7b3be757)] hotfix for broken build introduced by #95 also gofmt (Nick Irvine)
- [[c82e7fc](https://github.com/segmentio/chamber/commit//c82e7fc69d30f34077beec08ced54bed42840a6a)] added list sorting options (#95) (Bryce Hendrix)
- [[4fb5cd8](https://github.com/segmentio/chamber/commit//4fb5cd8d9ab160edb667148c837adcdb7c681812)] Fix 2 typos (Joseph Herlant)
- [[28b12a8](https://github.com/segmentio/chamber/commit//28b12a81659689a16a38ba0f70647e1cdd0fd9f1)] Quote environment variables. (#150) (Joshua Carp)
- [[2fd07a7](https://github.com/segmentio/chamber/commit//2fd07a7bbe7f316eada8e6bfa3a8212b3202343e)] Handle errors in secret store constructors. (#135) (Joshua Carp)
- [[7b5f2b8](https://github.com/segmentio/chamber/commit//7b5f2b859f2953302bff2ffcd9e9366946b298f4)] Use built-in pagination instead of loops. (#121) (Joshua Carp)
- [[7f09043](https://github.com/segmentio/chamber/commit//7f09043d32dc2b5faf01883ebc0784894900da83)] remove debug line (Rob McQueen)

# v2.2.0 vs v2.1.1

## deb/rpm packaging, circle 2.0

- [[f08dc82](https://github.com/segmentio/chamber/commit/f08dc82bd3d8490815e0fa1a48a1cfae9847e622)] add pointer to Installation wiki page (#132) (Nick Irvine)
- [[b8363c1](https://github.com/segmentio/chamber/commit/b8363c1f26630ba937bd22d67db7b940aa6459de)] Add deb/rpm packages (#129) (Nick Irvine)
- [[1f9b223](https://github.com/segmentio/chamber/commit/1f9b223377221cd770afa08c9298a49d110edfd5)] add missing govendor sync (Nick Irvine)
- [[b38f851](https://github.com/segmentio/chamber/commit/b38f85116c22aab95a1aa40eb36a3ccf08e37820)] move circle config to the right place :facepalm: (Nick Irvine)
- [[07d0605](https://github.com/segmentio/chamber/commit/07d06054f97b2ae124c491051afd8a70c49da8ca)] Circle 2.0 (#125) (Nick Irvine)
- [[e743d3b](https://github.com/segmentio/chamber/commit/e743d3bcb0a98f15ae66907be0014b0b38a17b16)] Add chamber-$(VERSION).sha256sums generation (#126) (Nick Irvine)

### Global `-verbose` flag

- [[e61fff1](https://github.com/segmentio/chamber/commit/e61fff176837b0d3e3c9240b3ee3d9e037fb94d8)] Fix flag shorthand conflict. (#134) (Joshua Carp)
- [[c9f8f68](https://github.com/segmentio/chamber/commit/c9f8f68d526406e6488e7b7eb12c1fd3c4631fa3)] Print env var keys on exec with global `-v` flag (Yarek Tyshchenko)

### Experimental s3 backend

- [[53ad806](https://github.com/segmentio/chamber/commit/53ad8063018ef756bb82a82ce8ad377ded18df7d)] fix writing keys for new services: (Daniel Fuentes)
- [[3d34550](https://github.com/segmentio/chamber/commit/3d34550e6eebcfe9925d58685ebf637201e92e24)] remove default bucket (Daniel Fuentes)
- [[ff5fb8f](https://github.com/segmentio/chamber/commit/ff5fb8f7e5f500ee17c09fc0ff14adae28724aa8)] cleanup `getStore()` (Daniel Fuentes)
- [[67ef9eb](https://github.com/segmentio/chamber/commit/67ef9ebe56af22e51b3b42505be1b655f278bc87)] set retries when creating session (Daniel Fuentes)
- [[bf4dbb4](https://github.com/segmentio/chamber/commit/bf4dbb45e5b10ea6d0c1f5ef08fd18c2cc52b7e6)] Make secret backend configurable (Daniel Fuentes)
- [[dcb44cf](https://github.com/segmentio/chamber/commit/dcb44cf730accf5f8b5997dc0f7f110792aa304c)] add some backend performance benchmarks (Daniel Fuentes)
- [[928e1c6](https://github.com/segmentio/chamber/commit/928e1c60d0e33eb4690c51bd6506125730a42758)] add S3Store implementation: (Daniel Fuentes)
- [[d1a43b5](https://github.com/segmentio/chamber/commit/d1a43b5391ad924afa8cf4df6e6fcec371b78143)] move session creation out of SSMStore constructor for reuse (Daniel Fuentes)
- [[b4edc16](https://github.com/segmentio/chamber/commit/b4edc1668a1aab7e0829ca808470ef3a1b93648c)] update readme to remove default bucket (Daniel Fuentes)
- [[576b28a](https://github.com/segmentio/chamber/commit/576b28a403f17aa3c820ce5d22dfd62fe0c38224)] add note in the readme about s3 backend (Daniel Fuentes)

## Fixes

- [[d9c77ea](https://github.com/segmentio/chamber/commit/d9c77eaa1bcfb82ae448f7b41fb1962ab5712e2b)] Support Variable Depths and improve regex (#118) (Gonzalo Peci)
- [[29a1f2b](https://github.com/segmentio/chamber/commit/29a1f2b7468fcb1bcc27b944bcb3ea2793bd8a13)] Merge pull request #122 from jmcarp/bail-out-pagination (Rob McQueen)
- [[d76fdd1](https://github.com/segmentio/chamber/commit/d76fdd1576a14ebe516a54191046e2490eb584da)] Return original error on reading parameter. (#124) (Joshua Carp)
- [[0134e37](https://github.com/segmentio/chamber/commit/0134e37e3809fd20b6f7984e31cc5209530e7ad6)] Stop paginating after finding a matching parameter. (Joshua Carp)
- [[e8febaa](https://github.com/segmentio/chamber/commit/e8febaadf29776f59ec1c428817c64f9592a0a19)] Update CHAMBER_NO_PATHS example in README (John Boggs)
