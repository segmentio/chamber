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
