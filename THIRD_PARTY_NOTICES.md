# Third-Party Notices

NookMux includes third-party open-source software. This file covers the components that require
specific attribution, carry obligations beyond attribution, or are modified by NookMux. It is a
curated list of the notable and obligation-bearing dependencies rather than an exhaustive inventory;
the complete dependency graphs are pinned in `go.mod` / `go.sum` for the Go backend and
`web/package.json` / `web/bun.lock` for the front-end.

The full license text for every component below is published in its upstream repository and is also
present in the local dependency directories after `go mod download` and `bun install`
(`go env GOMODCACHE` and `web/node_modules/`). The attribution notices that Apache-2.0 Section 4(d)
requires to be reproduced are quoted inline in the relevant sections.

NookMux itself is licensed under the GNU Affero General Public License v3.0; see `LICENSE`.

Projects that informed NookMux's design without contributing code — the gateways and tools listed in
the README acknowledgements — are credited in `README.md` / `README.en.md` and are outside the scope
of this file.

## new-api

- Source: `QuantumNous/new-api`
- Copyright: 2023-2026 QuantumNous
- License: GNU Affero General Public License v3.0

NookMux is a self-hosted customization built on a historical revision of new-api, and inherits
new-api's AGPL-3.0 licensing, its `Copyright (C) 2023-2026 QuantumNous` source-file headers, the
`New-Api-User` request-header contract, the console layout and data model, and the majority of the
relay, billing, storage and audit code. NookMux's own modifications remain subject to the same
AGPL-3.0 terms. The complete AGPL-3.0 text is distributed in `LICENSE`.

## one-api

- Source: `songquanpeng/one-api`
- Copyright: 2023 JustSong
- License: MIT License

new-api is itself a fork of one-api, the original single-binary LLM gateway. NookMux retains that
lineage in the relational schema, the model-ratio configuration and the legacy default SQLite
database name (`one-api.db`), and its startup banner still prints the original project credit
(`Original Project: OneAPI by JustSong`). The complete MIT License text is published in the upstream
repository.

## gin-contrib/sse

- Module: `github.com/gin-contrib/sse`
- Version: `v1.1.1`
- Copyright: 2014 Manuel Martínez-Almeida
- License: MIT License

The upstream package is linked unmodified through `gin-gonic/gin`, and NookMux additionally vendors a
modified copy of its SSE encoder as `internal/common/custom_event.go`. The vendored file keeps the
upstream copyright header and is adapted to the gateway's streaming path: it replaces the
`Event` / `Encode` pair with a mutex-guarded `CustomEvent` type and reduces `writeData` to a
pass-through framer for payloads the caller has already framed with a leading `data:` field, dropping
the upstream branches that JSON-encode structs, slices, maps and `[]byte`. The complete MIT License
text is published in the upstream repository.

## Go MySQL Driver

- Module: `github.com/go-sql-driver/mysql`
- Version: `v1.10.0`
- Copyright: 2012 The Go-MySQL-Driver Authors
- License: Mozilla Public License 2.0

Linked unmodified, through `gorm.io/driver/mysql`, for MySQL support. As required by MPL-2.0 Section
3.2, the Source Code Form for this version is available under the terms of the MPL at
https://github.com/go-sql-driver/mysql.

## AWS SDK for Go v2

- Modules: `github.com/aws/aws-sdk-go-v2` `v1.47.0`; `github.com/aws/aws-sdk-go-v2/credentials` `v1.20.5`; `github.com/aws/aws-sdk-go-v2/service/bedrock` `v1.58.0`; `github.com/aws/aws-sdk-go-v2/service/bedrockruntime` `v1.62.0`; `github.com/aws/smithy-go` `v1.28.1`
- Copyright: 2015 Amazon.com, Inc. or its affiliates; 2014-2015 Stripe, Inc.
- License: Apache License 2.0

Linked unmodified for the AWS Bedrock channel, covering model discovery (`service/bedrock`) and the
Converse / InvokeModel relay with streaming response decoding (`service/bedrockruntime`). The
rendered relaying, credential handling and channel bookkeeping remain owned by NookMux.

Notices from `github.com/aws/aws-sdk-go-v2/NOTICE.txt`:

> AWS SDK for Go
> Copyright 2015 Amazon.com, Inc. or its affiliates. All Rights Reserved.
> Copyright 2014-2015 Stripe, Inc.

Notice from `github.com/aws/smithy-go/NOTICE`:

> Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.

## Prometheus Client Libraries

- Modules: `github.com/prometheus/client_golang` `v1.23.2`; `github.com/prometheus/client_model` `v0.6.2`; `github.com/prometheus/common` `v0.68.1`; `github.com/prometheus/procfs` `v0.20.1`
- Copyright: 2012-2015 The Prometheus Authors
- License: Apache License 2.0

Linked unmodified through `samber/hot`, the in-process cache used by the channel-affinity logic in
`internal/domain/channel`. NookMux does not register its own Prometheus collectors and does not
expose a Prometheus scrape endpoint.

Notices from `github.com/prometheus/client_golang/NOTICE`:

> Prometheus instrumentation library for Go applications
> Copyright 2012-2015 The Prometheus Authors
>
> This product includes software developed at
> SoundCloud Ltd. (http://soundcloud.com/).
>
> The following components are included in this product:
>
> perks - a fork of https://github.com/bmizerany/perks
> https://github.com/beorn7/perks
> Copyright 2013-2015 Blake Mizerany, Björn Rabenstein
> See https://github.com/beorn7/perks/blob/master/README.md for license details.
>
> Go support for Protocol Buffers - Google's data interchange format
> http://github.com/golang/protobuf/
> Copyright 2010 The Go Authors
> See source code for license details.

Notice from `github.com/prometheus/client_model/NOTICE`:

> Data model artifacts for Prometheus.
> Copyright 2012-2015 The Prometheus Authors
>
> This product includes software developed at
> SoundCloud Ltd. (http://soundcloud.com/).

Notice from `github.com/prometheus/common/NOTICE`:

> Common libraries shared by Prometheus Go components.
> Copyright 2015 The Prometheus Authors
>
> This product includes software developed at
> SoundCloud Ltd. (http://soundcloud.com/).

Notice from `github.com/prometheus/procfs/NOTICE`:

> procfs provides functions to retrieve system, kernel and process
> metrics from the pseudo-filesystem proc.
>
> Copyright 2014-2015 The Prometheus Authors
>
> This product includes software developed at
> SoundCloud Ltd. (http://soundcloud.com/).

## pquerna/otp

- Module: `github.com/pquerna/otp`
- Version: `v1.5.0`
- Copyright: 2014 Paul Querna
- License: Apache License 2.0

Linked unmodified through `internal/infra/security` for TOTP-based two-factor authentication. The
account binding, secret storage, session state and recovery-code policy remain owned by NookMux.

Notice from `github.com/pquerna/otp/NOTICE`:

> otp
> Copyright (c) 2014, Paul Querna
>
> This product includes software developed by
> Paul Querna (http://paul.querna.org/).

## YAML for Go

- Module: `gopkg.in/yaml.v3`
- Version: `v3.0.1`
- Copyright: 2011-2016 Canonical Ltd.
- License: Apache License 2.0 (portions ported from libyaml are under the MIT License, as noted in
  the upstream `LICENSE`)

Linked unmodified through `internal/i18n` for loading the YAML translation bundles used by backend
API response localization.

Notice from `gopkg.in/yaml.v3/NOTICE`:

> Copyright 2011-2016 Canonical Ltd.
>
> Licensed under the Apache License, Version 2.0 (the "License");
> you may not use this file except in compliance with the License.
> You may obtain a copy of the License at
>
>     http://www.apache.org/licenses/LICENSE-2.0
>
> Unless required by applicable law or agreed to in writing, software
> distributed under the License is distributed on an "AS IS" BASIS,
> WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
> See the License for the specific language governing permissions and
> limitations under the License.

## Pyroscope Go

- Module: `github.com/grafana/pyroscope-go`
- Version: `v1.4.2`
- Copyright: 2020 Pyroscope
- License: Apache License 2.0

Linked unmodified through `internal/infra/runtime` for optional continuous profiling. Profiling is
started only when `PYROSCOPE_URL` is configured, and the profiler configuration and lifecycle remain
owned by NookMux. This module is the reason the Prometheus client libraries above are linked into the
binary.

## go-epay

- Module: `github.com/Calcium-Ion/go-epay`
- Version: `v0.0.4`
- Copyright: Calcium-Ion
- License: MIT License (declared by the upstream repository; the `v0.0.4` module archive published to
  the Go module proxy does not include a `LICENSE` file)

Linked unmodified through `internal/httpapi/controller/topup` for the 易支付 (epay) payment
integration, providing purchase-link generation, MD5 signing and callback verification. Order
state, credit granting and reconciliation remain owned by NookMux.

## Lobe Icons

- Source: `@lobehub/icons` `5.18.0` (npm dependency of the management UI)
- Copyright: 2023 LobeHub
- License: MIT License

NookMux uses Lobe Icons to render upstream provider and vendor brand marks in the console, including
the channel preset picker and the `vendor_meta` icon names persisted in the database. The icons and
this notice do not grant any trademark rights in the marks they depict. The complete MIT License text
is published in the upstream repository.

## Brand Marks

- Source: `web/src/assets/brand-icons/` (18 SVG components), `web/public/pay-apple.png`,
  `web/public/pay-card.png`, `web/public/pay-google.png`, `web/public/waffo-logo-{dark,light}.svg`
- License: Not applicable — these are brand marks, and no license is granted

NookMux vendors a small set of third-party brand marks to label OAuth providers and payment methods
(for example GitHub, GitLab, Docker, Stripe, Telegram, Slack, WeChat, Apple Pay and Google Pay) as
in-tree components and static assets. Each mark remains the trademark of its respective owner. Its
presence grants no trademark, logo or brand license, and must not be read as implying any
endorsement or affiliation with the mark owner.

## Public Sans

- Source: `@fontsource-variable/public-sans` `5.3.0`
- Copyright: 2015 The Public Sans Project Authors
- License: SIL Open Font License 1.1

Self-hosted as a variable font through `@fontsource-variable`, so the console does not fetch web
fonts from a third-party CDN at runtime. The complete SIL Open Font License 1.1 text is published in
the upstream repository.

## Hugeicons and Lucide

- Sources: `@hugeicons/core-free-icons` `4.3.0` and `@hugeicons/react` `1.1.10` (MIT License);
  `lucide-react` `1.34.0` (ISC License)
- Copyright: Hugeicons; 2013-present Cole Bemis and Lucide Icons and Contributors
- Licenses: MIT License / ISC License

NookMux uses these icon sets unmodified as the console's UI iconography. The complete license texts
are published in the respective upstream repositories.

## Build-Time and Development Dependencies

- Modules: `github.com/hashicorp/golang-lru/v2` `v2.0.7` (MPL-2.0); `lightningcss` `1.32.0`
  (MPL-2.0); `@resvg/resvg-js` `2.4.1` (MPL-2.0); `dompurify` `3.4.5` (MPL-2.0 OR Apache-2.0);
  `caniuse-lite` `1.0.30001810` (CC-BY-4.0)

These components appear in the resolved dependency graphs but are not compiled into the released
server binary, and they are used unmodified:

- `github.com/hashicorp/golang-lru/v2` is reached through the pure-Go SQLite code generators
  (`modernc.org/ccgo`), not through the runtime path that NookMux compiles.
- `lightningcss` and `@resvg/resvg-js` are native build tooling used by the Tailwind / PostCSS
  pipeline and the charting stack; their native binaries never reach the browser.
- `dompurify` is a transitive dependency of the console's diagram rendering stack (`mermaid`) and is
  dual-licensed `MPL-2.0 OR Apache-2.0`; NookMux relies on the Apache-2.0 option.
- `caniuse-lite` is browserslist data used to resolve build targets.

For MPL-2.0 components, the Source Code Form is available under the terms of the MPL at the
corresponding upstream repository. The CC-BY-4.0 attribution for `caniuse-lite` is reproduced here:
Browser compatibility data by caniuse.com, licensed under CC BY 4.0.
