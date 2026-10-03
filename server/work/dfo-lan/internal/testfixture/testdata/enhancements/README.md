# Enhancement history fixtures

Six complete retired exports are stored as gzip test fixtures. `testfixture.EnhancementPath` / `EnhancementsDir` decompress into per-test temporary directories and verify each original SHA256. Missing or corrupt fixtures fail the test.

Runtime preparation uses `gamedata.Source.Enhancements` and the current read-only server PVF with `configs/pvf-enhancement-policy.json`; these snapshots never provide runtime fallback. No formulas, explicit policy values, instance expiration behavior, source identity or player storage changed.

| File | Original bytes | gzip bytes | Original SHA256 |
| --- | ---: | ---: | --- |
| reinforcement-tickets.json | 3251025 | 65833 | `8bbeae8b169b7e497d1fe44379c0c6033880e495c859d23649167c6404dfa793` |
| reinforcement-gold.json | 75534 | 4449 | `027dddc534e28bce80bb5905b6ec84f0f38703682f2e38cf143f66cbcaf8bd4f` |
| amplify-grimoire.json | 204485 | 6438 | `7c46704b5bb0a741a232154d23ce8f8ad586d281a99c9b845793a03016bda715` |
| amplify-upgrade.json | 61429 | 4966 | `007838a55923a641078be3ef2cc644147ca3b55ad57a7f93defad5d3e1f70f01` |
| amplify-tickets.json | 4306992 | 76687 | `80e12f24eecedeced73c2e4c557339cd0fe5ee01661dc91bc7554dbfa68dc0a4` |
| enchant-beads.json | 715897 | 46812 | `3419fba4fef95135fcc889778f6dbace2e773eb175eba9403d075b51b33c6f31` |

The historical snapshots record earlier PVF epochs. Current `8b2a9f83` native content was already active before cleanup and has exactly 1,430 amplify-ticket expiration text differences (historical years replaced by 2099, all other date characters retained); every other typed field matches after source metadata normalization. The old ordinary reinforcement-ticket export also omitted 926 native expiration headers. The cleanup keeps the complete historical consumer regressions and separately pins the current native content hash. It does not make historical dates runtime policy.
