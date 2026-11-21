# Changelog

## [0.1.1](https://github.com/golem-mc/worldguard/compare/v0.1.0...v0.1.1) (2026-11-21)


### Bug Fixes

* **deps:** update golem-mc modules ([#49](https://github.com/golem-mc/worldguard/issues/49)) ([d759068](https://github.com/golem-mc/worldguard/commit/d759068862ae79ce2ee8ba869970949eed02c523))

## 0.1.0 (2026-11-21)


### Features

* **blacklist:** deny, log and report blacklisted blocks and items ([#22](https://github.com/golem-mc/worldguard/issues/22)) ([c6b745a](https://github.com/golem-mc/worldguard/commit/c6b745a96c28a9ec2390015891247b15f6aa0c7b))
* check host keys when a player logs in ([#36](https://github.com/golem-mc/worldguard/issues/36)) ([19e884e](https://github.com/golem-mc/worldguard/commit/19e884e95d213c6e6078d478c1286f87a233c3fa))
* **commands:** add /region with its sub-commands ([#20](https://github.com/golem-mc/worldguard/issues/20)) ([500ac4c](https://github.com/golem-mc/worldguard/commit/500ac4c4962dedb1bc201ab355a3c9ee20476151))
* **commands:** add /wg debug to simulate breaking, placing, interacting and damage ([#25](https://github.com/golem-mc/worldguard/issues/25)) ([d82f0c4](https://github.com/golem-mc/worldguard/commit/d82f0c46c0a6b0a3c8e253bf91a4a7fd6116e00c))
* **commands:** paste /wg report and /wg profile output with -p ([#27](https://github.com/golem-mc/worldguard/issues/27)) ([40c3d6b](https://github.com/golem-mc/worldguard/commit/40c3d6b12a5a1a2cd42f39b8ba24ee8b9b1731fa))
* enforce potion splashes, leashing, lectern books, soil moisture and knockback ([#34](https://github.com/golem-mc/worldguard/issues/34)) ([5252388](https://github.com/golem-mc/worldguard/commit/5252388cfab949d1bd68b3de58dc2f9a8a5d1439))
* name the plugin that blocked an action in wg debug reports ([#43](https://github.com/golem-mc/worldguard/issues/43)) ([33b06fe](https://github.com/golem-mc/worldguard/commit/33b06fe06a7730ee278483340366331f85331f83))
* **protection:** protect regions and apply the world settings ([#19](https://github.com/golem-mc/worldguard/issues/19)) ([b4a6fa5](https://github.com/golem-mc/worldguard/commit/b4a6fa5a197f0fc8190d77a341eaff5d7cd5dad3))
* reach general commands as /worldguard:name when another plugin holds the name ([#35](https://github.com/golem-mc/worldguard/issues/35)) ([5264a92](https://github.com/golem-mc/worldguard/commit/5264a92789598955512c1913653581861593c6ec))
* **regions:** add regions, flags, YAML storage and configuration ([#2](https://github.com/golem-mc/worldguard/issues/2)) ([86caadd](https://github.com/golem-mc/worldguard/commit/86caadd4e89ab8965a8fba14860f263a98791708))
* relay log records to a player running /wg reload ([#28](https://github.com/golem-mc/worldguard/issues/28)) ([ae47c52](https://github.com/golem-mc/worldguard/commit/ae47c5275fbe007b4e54ab4dbdcf42c5a07551b4))
* **sessions:** add region movement flags and the general, toggle and /wg commands ([#14](https://github.com/golem-mc/worldguard/issues/14)) ([083ce2d](https://github.com/golem-mc/worldguard/commit/083ce2db953b88cd6cda05958b6ef5d1138d5e99))
* stop powered sponges absorbing and refuse tipped arrows of blocked effects ([#37](https://github.com/golem-mc/worldguard/issues/37)) ([553e93a](https://github.com/golem-mc/worldguard/commit/553e93a5df720d37f82908f503612b3ddd8614e1))
* warn about vanilla spawn protection and follow /defaultgamemode ([#26](https://github.com/golem-mc/worldguard/issues/26)) ([d3b034d](https://github.com/golem-mc/worldguard/commit/d3b034df9bc9a025fdd3ccec7466cfe317a9dc8e))


### Bug Fixes

* check dispensers equipping armour against the blacklist's on-equip ([#41](https://github.com/golem-mc/worldguard/issues/41)) ([2145522](https://github.com/golem-mc/worldguard/commit/2145522fc818fe8e75f97b50fdbe13fb17594b4e))
* **deps:** require golem v0.1.0 ([#48](https://github.com/golem-mc/worldguard/issues/48)) ([e2ae62d](https://github.com/golem-mc/worldguard/commit/e2ae62d2d7b7c557b7d5a3ccc1d6263d4486d68b))
* **deps:** update golem-mc modules ([#47](https://github.com/golem-mc/worldguard/issues/47)) ([0aaa693](https://github.com/golem-mc/worldguard/commit/0aaa693866b7faeceaf738568ed29ad1fb594b38))
* enforce wind charge knockback in protected regions ([#39](https://github.com/golem-mc/worldguard/issues/39)) ([58f80fc](https://github.com/golem-mc/worldguard/commit/58f80fc37b2528865ef04f7e42de8ff65df8dcb3))
* judge fire spread, hopper moves, burning and primed TNT by their real cause ([#32](https://github.com/golem-mc/worldguard/issues/32)) ([c683cfd](https://github.com/golem-mc/worldguard/commit/c683cfd9632d4117125c596e1dd1b1356c2679c5))
* keep the client's title times when titles-always-use-default-times is false ([#29](https://github.com/golem-mc/worldguard/issues/29)) ([f992d50](https://github.com/golem-mc/worldguard/commit/f992d507a3fa41f1e83ef8bee0cd5bd5ea31e1c4))
* stop projectile hits logging a listener panic ([#40](https://github.com/golem-mc/worldguard/issues/40)) ([cd9a969](https://github.com/golem-mc/worldguard/commit/cd9a9694dd814efd6f76c3f0c5c5cbcc428c1e79))
* stop renovate bumping modules golem owns ([#46](https://github.com/golem-mc/worldguard/issues/46)) ([8c9548a](https://github.com/golem-mc/worldguard/commit/8c9548a372e60bfc4bff47131a786d41a275ccc8))
