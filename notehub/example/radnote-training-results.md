# Skills for com.blues.radnote

*6 documents, assembled from the project by `notehub skills show all` on 2026-09-25.  Each is stored separately and appears here as stored, with its headings one level lower and its cross-references linked.*

## Contents

- [Radnote](#index-md) - `index.md`, index
- [Radnote](#product-md) - `product.md`, product,usage,mission,audiences,vocabulary,presentation,constraints
- [Operations](#operations-md) - `operations.md`, population,config,scenarios
- [Notefile inventory](#notefiles-md) - `notefiles.md`, notefiles,derivation,sentinels
- [Recipes](#recipes-md) - `recipes.md`, recipe
- [Open questions](#questions-md) - `questions.md`, questions,anomalies

---

<a id="index-md"></a>

## Radnote

*`index.md` | kinds: index | 2026-09-25 14:01 | 4274 bytes*

```yaml
kind: index
description: Initial Radnote training draft and coverage status
updated: 2026-09-25T18:01:36Z
sources:
  - H6: trainer, default session lookback of 90 days confirmed by enumerated read-back 2026-09-25
  - H5: trainer, query patterns, deployment scope and timezone confirmed by enumerated read-back 2026-09-25
  - H4: trainer, V2/V3 radiation-conversion equivalence reported 2026-09-25; read-back pending
  - F1: restricted V3 host firmware, version 3.2.2, reviewed 2026-09-25
  - H1: trainer, interview and enumerated read-back confirmed 2026-09-25
  - E1: project schema response, reviewed 2026-09-25
  - P: public deployment reports, technical articles and receiving surfaces, reviewed 2026-09-25
  - D1: public owner guide for the V2 product, public, revised 2022-07-06, reviewed 2026-09-25
```

Product generation and purpose: see [product.md](#product-md), [Product generation](#product-md--product-generation) and [Purpose](#product-md--purpose). Those three interview claims were confirmed by the trainer after enumerated read-back. Two guide-scoped rules were also confirmed; their owners are [operations.md](#operations-md) and [notefiles.md](#notefiles-md).

<a id="index-md--documentation"></a>

### Documentation

- API reference: https://dev.blues.io/api-reference/notehub-api
- OpenAPI specification: https://raw.githubusercontent.com/blues/notehub-js/main/openapi.yaml

<a id="index-md--what-is-recorded"></a>

### What is recorded

- [product.md](#product-md): confirmed purpose, documented field use, operating modes, power limits and public answer surfaces; deployment-specific policy remains open.
- [notefiles.md](#notefiles-md): 20 names with entries, all refreshed schema paths, V2-guide mechanisms, V3 fixed/tracking decoder and fixed-reading inclusion policy; shared V2/V3 conversion meaning is recorded; timing, encoding and module boundaries remain unresolved.
- [operations.md](#operations-md): recent-session population, UTC default with asker override, capture-based reading windows and geography rules; session lookback is established; actual applied settings remain open.
- [recipes.md](#recipes-md): conditional general workflow for rolling-window/geographic questions, including period and latest-state branches; no concrete operational result has been validated.
- [questions.md](#questions-md): open questions and offered materials.

<a id="index-md--training-status"></a>

### Training status

General product-purpose, field-use and radiation-conversion explanations have documentary/trainer support. The rolling-window/geographic workflow in [recipes.md](#recipes-md) is conditional on the actual statistic/scope, usable session evidence where deployment membership is needed, and usable measurement/location evidence. It includes latest-state and period branches but has not been exercised on a concrete operational query. No predictive model or universal operational answerability is claimed; duration/dose/forecast inferences need their own support.

UTC is the default with asker override, sessions within the default lookback define deployment eligibility, and recent radiation windows use captured time. These instructions and zero/missing fixed-reading exclusion were confirmed by enumerated read-back. V2/V3 conversion equivalence remains trainer-reported (`heard`) pending its own read-back; it is usable evidence with that origin, not a guessed rule. Remaining V2 timing, encoding and exceptional handling gaps are named in [questions.md](#questions-md).

First next step: use the general workflow for the actual question asked, resolving only the gaps that affect it. Neither a firmware default nor a source example establishes deployed settings. Unmatched questions can extend the workflow; unsupported inferences are identified specifically rather than claiming all questions are answered.

<a id="index-md--source-handling"></a>

### Source handling

The supplied case study was explicitly classified public. Related public reports and pages were read under the trainer's research authorization. The V2 owner guide is explicitly public and has been read; its descriptions are scoped to revision 2022-07-06. The V3 firmware was received, classified restricted and reviewed deeply for cloud-data meaning. Its source remains private; only paraphrased interface meanings are drafted with explicit revision scope. No unreleased plans or source architecture are included. Local display behavior is outside this cloud-data lesson at the trainer's request. Published-source meanings are paraphrased and scoped; source links, titles and identifying details remain outside this working copy.

---

<a id="product-md"></a>

## Radnote

*`product.md` | kinds: product, usage, mission, audiences, vocabulary, presentation, constraints | 2026-09-25 14:01 | 11400 bytes*

```yaml
kind: product,usage,mission,audiences,vocabulary,presentation,constraints
description: Product purpose and the people who use its answers
updated: 2026-09-25T18:01:36Z
sources:
  - H5: trainer, query patterns, deployment scope and timezone confirmed by enumerated read-back 2026-09-25
  - H2: trainer, enumerated guide read-back confirmed 2026-09-25; local-display scope excluded
  - H1: trainer, interview and enumerated read-back confirmed 2026-09-25
  - P1: public field-deployment case study, public, published 2024-08, reviewed 2026-09-25
  - P2: public deployment follow-up, public, published 2025-07, reviewed 2026-09-25
  - P3: public post-accident fieldwork account, public, published 2025-06, reviewed 2026-09-25
  - P4: public certification announcement, public, published 2026-01, reviewed 2026-09-25
  - P5: public technical article on device faults, public, published 2025-07, reviewed 2026-09-25
  - P6: public network description and map guidance, public, undated, reviewed 2026-09-25
  - P7: public data-provider usage guidance, public, revised 2026-07, reviewed 2026-09-25
  - P8: public downstream environmental dashboard, public, reviewed 2026-09-25
  - P9: public city radiation summary, public, reviewed 2026-09-25
  - P10: public device catalog, public, undated, reviewed 2026-09-25
  - D1: public owner guide for the V2 product, public, revised 2022-07-06, reviewed 2026-09-25
```

Initial draft. The three interview claims below were confirmed by the trainer after enumerated read-back. Other source-derived findings below remain documented. Confirmed guide measurement and buffering rules belong to [notefiles.md](#notefiles-md) and [operations.md](#operations-md). Operational rules remain incomplete. D1 describes the guide revision of 2022-07-06; later builds and installed configurations require the applicability check in Q11.

<a id="product-md--product-generation"></a>

### Product generation

- Radnote V2 produces the project data being discussed. `[stated:H1 2026-09-25]`

- The documented standard enclosure supports gamma-focused monitoring; a physically altered sensor enclosure changes the measurement context and must be identified before comparing readings. `[documented:D1 V2 guide revision 2022-07-06 conf:high]`

<a id="product-md--purpose"></a>

### Purpose

- Radnote is designed for ongoing radiation monitoring and alerting when conditions become anomalous. `[stated:H1 2026-09-25]`
- Its primary role is a resilient monitoring layer alongside official monitoring networks. `[stated:H1 2026-09-25]`

- Fixed-site monitoring is intended to build a local radiation baseline and retain changes over time; moving surveys associate measurements with the path traveled. A network may support studying spatial changes, but a forecast or health judgment needs evidence beyond this purpose statement. `[documented:D1 V2 guide revision 2022-07-06 conf:high]`

<a id="product-md--usage"></a>

### Usage

<a id="product-md--field-deployment"></a>

#### Field deployment

- Documented deployments use outdoor, fixed, unattended monitors with solar power and cellular transmission. Non-specialist hosts install them on poles or flat surfaces using mounting kits. The sources do not identify the exact hardware or firmware revisions. `[documented:P1 2024-08 deployment scope conf:high]`
- Installations include homes, offices, parks and research sites. Poor sunlight and ice covering the panel can limit charging; a published cold-weather account describes backup power but gives no guaranteed autonomy. `[documented:P2 2025-07 deployment scope conf:high]`
- Vertical installation is documented as part of the solar design. Correct orientation for this product's telemetry must still be established; see [questions.md](#questions-md), [Q8](#questions-md--q8-technical-examples-and-variant-applicability). `[documented:P5 2025-07 product scope conf:medium]`
- A post-accident fieldwork account reports adding monitors to communities preparing for returning residents. It also describes condition-check visits to existing radiation monitors without identifying all those models. `[documented:P3 2025-06 fieldwork scope conf:high]`

<a id="product-md--operating-modes-and-field-handling"></a>

#### Operating modes and field handling

- Fixed operation periodically records environmental measurements at one site. Mobile operation follows motion-detected journeys, commonly from a vehicle window. Hybrid operation adds periodic measurements while stationary. The supported physical uses include pole/fence mounting, a vehicle-window mount with a secondary restraint, and a temporary stand. `[documented:D1 V2 guide revision 2022-07-06 conf:high]`
- Sunlight can change after installation because of foliage, new construction or a moved mounting point. Theft, vandalism and physical movement can also break monitoring continuity. `[documented:D1 V2 guide revision 2022-07-06 conf:high]`
- Servicing can expose electronics to condensation collected inside the case. The guide calls for careful handling of accumulated water and resealing in dry conditions. This is a documented maintenance hazard, not an observed failure in this fleet. `[documented:D1 V2 guide revision 2022-07-06 conf:high]`

<a id="product-md--power-context"></a>

#### Power context

- The guide describes a solar-recharged lithium-polymer option and a long-life non-rechargeable lithium-thionyl-chloride option. It does not establish that both are installed together in every device. Battery type must be known before interpreting voltage or lifetime. `[documented:D1 V2 guide revision 2022-07-06 conf:high]`
- A primary battery may remain near 3.6 V until almost exhausted; voltage alone does not establish remaining life. Replacement history and the operating configuration are needed. `[documented:D1 V2 guide revision 2022-07-06 conf:high]`
- Cellular connection frequency and moving GPS operation are substantial energy costs relative to periodic radiation measurement. The guide's long unattended lifetime is a design estimate, not an observed guarantee or a forecast for an individual device. `[documented:D1 V2 guide revision 2022-07-06 conf:high]`

Actual maintenance schedules, battery installation history and the effects of replacements remain open; see Q2, Q7 and Q11.

<a id="product-md--question-patterns"></a>

### Question patterns

Support the questions people bring to the data rather than a fixed list of permitted questions. The trainer expects common requests to use a rolling window of recent readings from one device or a device set, often restricted to a geographic region. `[stated:H5 trainer 2026-09-25 conf:high]`

The general workflow is in [recipes.md](#recipes-md), [Rolling-window and geographic analysis](#recipes-md--rolling-window-and-geographic-analysis). Choose the statistic and output from the actual ask. Supporting arbitrary questions does not establish data for every answer; identify a missing field, coverage gap or unsupported inference specifically when it matters. Do not insist on a predefined question example before helping the asker.

<a id="product-md--audiences-and-presentation"></a>

### Audiences and presentation

<a id="product-md--documented-audiences"></a>

#### Documented audiences

| Audience | Documented context | Remaining limit |
|---|---|---|
| Residents and the wider public | Public radiation information near inhabited areas and nuclear facilities; additional information for returning communities. `[documented:P3,P6 2025-06 and undated public descriptions conf:high]` | Preferred decisions and permitted answer wording still require Q3. |
| Volunteer hosts and field operators | Install and maintain monitoring coverage; see Usage, Field deployment. | Alert recipients, response time and service criteria remain open in Q3. |
| Public authorities and emergency-information recipients | A dated announcement reports acceptance of one deployment for official information channels. `[documented:P4 2026-01 deployment scope conf:medium]` | No certificate scope or universal approval has been established; see Constraints. |
| Anyone not listed, including a relayed recipient | No permission or escalation owner has been established. | Ask who ultimately receives the answer and resolve Q3 before giving an interpretation that needs recipient approval. |

<a id="product-md--receiving-surfaces"></a>

#### Receiving surfaces

- A public radiation map is an established destination for the documented deployment. Its broader map combines multiple monitoring sources; map membership alone does not identify a Radnote. `[documented:P6 undated platform scope; reviewed 2026-09-25 conf:high]`
- Public city summaries display dose rate in µSv/h with a timestamp and a last-24-hour minimum/maximum. They combine automatic and manual measurements, so the city range is not a per-device statistic. `[documented:P9 city-summary scope; reviewed 2026-09-25 conf:high]`
- One downstream dashboard presents µSv/h, a status label, an API-receipt timestamp, a 15-minute refresh and a retained fallback value. These describe that display only, not the sensor's sampling interval or this project's ingestion. Its timezone, fallback behavior and status thresholds remain unverified. `[documented:P8 downstream-display scope; reviewed 2026-09-25 conf:high]`

The guide also describes external fixed and mobile survey views; see [operations.md](#operations-md), [External survey delivery](#operations-md--external-survey-delivery). The preferred shape, precision and baseline for each actual question remain open in Q3. Existing screen labels do not establish permission to make the same claim in an agent answer.

<a id="product-md--vocabulary"></a>

### Vocabulary

- Invalid fixed radiation readings: see [notefiles.md](#notefiles-md), [Fixed-reading inclusion policy](#notefiles-md--fixed-reading-inclusion-policy).
- Raw pulses, corrected count rate, dose rate and measurement quality: see [notefiles.md](#notefiles-md), [Shared radiation conversion](#notefiles-md--shared-radiation-conversion) and [V3 rich fixed fields](#notefiles-md--v3-rich-fixed-fields).

- “Overlay network” means the supplementary monitoring role described in Purpose; it does not itself define a sampling interval or an emergency threshold.
- “Real time” in public deployment material describes the monitoring service. Measurement time, upload delay and display refresh still need separate definitions; see [questions.md](#questions-md), [Q4](#questions-md--q4-measurement-and-anomalies) and [Q9](#questions-md--q9-screen-semantics-versus-measurement). `[documented:P1 2024-08 deployment-language scope conf:medium]`
- Counts per minute and radiation dose rate: see [notefiles.md](#notefiles-md), [_air.qo](#notefiles-md--air-qo).
- “Bad tube” in a public technical example concerns possible sensor trouble; see [notefiles.md](#notefiles-md), [Public fault example](#notefiles-md--public-fault-example). It is not a radiation-exceedance category.
- Fixed, mobile and hybrid: see Operating modes and field handling.
- Sampling, integration, upload, GPS update and tracking heartbeat: see [operations.md](#operations-md), [Configuration controls](#operations-md--configuration-controls) and [notefiles.md](#notefiles-md), [Measurement mechanism](#notefiles-md--measurement-mechanism).
- Journey and journey count: see [notefiles.md](#notefiles-md), [_track.qo](#notefiles-md--track-qo).
- The meaning of “anomalous” and the thresholds behind it remain open; see [questions.md](#questions-md), [Q4](#questions-md--q4-measurement-and-anomalies). The physical alarm setting is not an agreed public-health threshold.

<a id="product-md--constraints"></a>

### Constraints

<a id="product-md--public-interpretation"></a>

#### Public interpretation

- The public data provider describes its map readings as unverified. Its guidance calls for examining neighboring stations over the same period and relevant weather before interpreting an increase; official emergency instructions belong to the responsible authorities. `[documented:P7 revised 2026-07 provider scope conf:high]`
- A certification announcement does not establish approval for every variant, deployment or proposed decision. The certificate's scope has not been reviewed. `[documented:P4 2026-01 announcement scope conf:medium]`
- Device catalogs distinguish Radnote from older fixed and mobile instruments. Do not transfer those instruments' cadence or deployment history into a Radnote rule. `[documented:P10 undated catalog scope; reviewed 2026-09-25 conf:high]`

No radiation alert threshold, product-wide safety wording, escalation owner or response rule has been confirmed; see Q3 and Q4. The general analysis workflow is conditional on the query-specific scope, evidence and inference gates; see [recipes.md](#recipes-md), [Status](#recipes-md--status).

---

<a id="operations-md"></a>

## Operations

*`operations.md` | kinds: population, config, scenarios | 2026-09-25 14:01 | 16125 bytes*

```yaml
kind: population,config,scenarios
description: Population and operating rules still to establish
updated: 2026-09-25T18:01:36Z
sources:
  - H6: trainer, default session lookback of 90 days confirmed by enumerated read-back 2026-09-25
  - H5: trainer, query patterns, deployment scope and timezone confirmed by enumerated read-back 2026-09-25
  - F1: restricted V3 host firmware, version 3.2.2, reviewed 2026-09-25
  - H2: trainer, enumerated guide read-back confirmed 2026-09-25
  - H1: trainer, interview 2026-09-25, initial discovery incomplete
  - D1: public owner guide for the V2 product, public, revised 2022-07-06, reviewed 2026-09-25
```

Initial draft. Population is based on session activity within the preceding 90 days by default. Applied configuration rules remain incomplete. D1 supplies documented mechanisms for its 2022-07-06 revision; actual configuration and later-build applicability remain Q11.

<a id="operations-md--population"></a>

### Population

Count all devices with recent sessions as deployed. `[stated:H5 trainer 2026-09-25 conf:high]` The default recent-session lookback is 90 days. `[stated:H6 trainer 2026-09-25 conf:high]`

Apply the default lookback relative to the analysis reference time, unless the asker explicitly selects another deployment criterion. Do not reuse the radiation window as though it were the same as this session lookback. Where the asker supplies a device set directly, retain that requested scope and report session eligibility separately when relevant. Do not exclude a qualifying device merely because its label suggests a test or bench device; no such exclusion was requested. Refresh session evidence for the query rather than treating membership as permanent.

For a reproducible session test at reference time T, use session records' `session_began`, `work` and `session_ended` as evidence of start, activity and end, respectively. A device qualifies when at least one valid such timestamp lies from T minus the chosen lookback through T. Missing, zero or future timestamps supply no qualifying evidence; incomplete session history produces unknown eligibility, not an unsupported exclusion. This is the analysis predicate for the trainer's recent-session rule; field meanings are in [notefiles.md](#notefiles-md), [Session evidence](#notefiles-md--session-evidence). A null end alone does not establish recent activity.

Session eligibility and valid radiation coverage are separate. A deployed device can have no usable radiation in the requested window; retain it in the eligible population and report the missing coverage. Registration alone does not establish deployment. Session clocks and limits are in [notefiles.md](#notefiles-md), [Session evidence](#notefiles-md--session-evidence).

For historical questions, distinguish devices deployed at the historical time from devices deployed now whose history is requested. A lack of a recent session today does not invalidate an older measurement within an explicitly historical study. State which population the answer uses; clarify when the distinction changes the result.

<a id="operations-md--time-windows"></a>

### Time windows

Use UTC unless the asker specifies another timezone. `[stated:H5 trainer 2026-09-25 conf:high]`

Resolve a rolling window against a recorded analysis reference time and state its start, end and timezone. Use a consistent half-open interval when splitting adjacent buckets, and check actual boundary behavior in the retrieved data. Calendar-day wording uses the selected timezone; an unspecified duration such as recent or overnight still needs a duration/boundary choice. An explicit timezone takes precedence over this default.

For recent radiation analysis, use captured time `when` and exclude captures outside the requested reading window, even if they arrived recently. `[stated:H5 trainer 2026-09-25 conf:high]` The actual query window determines inclusion; no blanket numeric stale-data threshold has been supplied. Capture and arrival meanings are in [notefiles.md](#notefiles-md), [Event clocks and location](#notefiles-md--event-clocks-and-location).

Keep historical analysis available: an explicit historical request selects captured readings within that historical interval, rather than globally dropping old records. Questions specifically about data arrival use the arrival clock and label that choice. Do not silently interpret recent arrivals as recent radiation measurements.

<a id="operations-md--geographic-selection"></a>

### Geographic selection

A geographic query needs a resolved region: named boundary, polygon, radius or explicit device/site set, as appropriate to the ask. Clarify ambiguous place names and whether the question concerns devices located there now or measurements captured there during the window. Use the event location for the latter when its semantics and timing support it; do not silently assign historical readings to the device's latest position.

Location source and age are material near boundaries. Use [notefiles.md](#notefiles-md), [Event clocks and location](#notefiles-md--event-clocks-and-location) to distinguish event position, location source and location timestamp. Missing or unsuitable location means geographically unclassified, not automatically inside or outside. A fleet or label is a geography proxy only with a verified mapping. Report location-related exclusions and precision limits alongside the selected population. Exact acceptable fix age and boundary tolerance depend on the ask; no universal values have been established.

<a id="operations-md--configuration-controls"></a>

### Configuration controls

Sampling and uploading are separate: measurements can be buffered and delivered later. `[stated:H2 2026-09-25 V2 guide scope]`

Cloud environment settings take precedence over the physical interval switches. They are received during synchronization; a configured value alone does not prove when a device received or applied it. `[documented:D1 V2 guide revision 2022-07-06 conf:high]`

| API-visible key | Meaning in the documented guide | Limits |
|---|---|---|
| `_data_sampling_secs` | Interval between periodic measurement starts, in seconds. | Integration is separate; see [notefiles.md](#notefiles-md), [Measurement mechanism](#notefiles-md--measurement-mechanism). OFF in the mobile example disables periodic sampling, not journey measurements. |
| `_data_upload_secs` | Interval for cellular synchronization of recorded outbound data and inbound environment updates, in seconds. | Uploads may contain buffered measurements. Exact idle behavior is Q12. |
| `_gps_update_secs` | GPS attempt interval and journey-record cadence, in seconds. | Motion gating can suppress attempts; see Location and motion. |
| `_gps_tracking_secs` | Enables tracking when not OFF and sets its heartbeat period, in seconds. | It is not the journey sampling interval. The heartbeat's Notefile and fields are Q12. |
| `_rad_alarm_cpm` | Count-rate setting for the red local alarm indicator, in counts/minute. | No fleet value, equality boundary, reset rule or remote alert is established; Q4. |

All five V2-guide control meanings: `[documented:D1 V2 guide revision 2022-07-06 conf:high]`. Wire encodings for OFF, unset and inheritance remain open in Q11.

<a id="operations-md--location-and-motion"></a>

### Location and motion

A successful location can be reused on measurements when the accelerometer detects no movement. GPS attempts may therefore be skipped. For a GPS interval of five minutes or less, the guide describes GPS remaining active during motion and turning off after motion ceases. This does not prove a fresh fix for every note. `[documented:D1 V2 guide revision 2022-07-06 conf:high]`

Motion initiates a journey; its records carry a radiation measurement, time, location and journey identity. The guide's exact idle/stop behavior is internally unclear; see A3/Q12. `[documented:D1 V2 guide revision 2022-07-06 conf:medium]`

<a id="operations-md--example-configurations"></a>

### Example configurations

These are illustrative guide configurations, not observed fleet settings. The averaging mechanism belongs to [notefiles.md](#notefiles-md), [Measurement mechanism](#notefiles-md--measurement-mechanism).

| Example | Periodic sampling | Upload | Tracking heartbeat | GPS update |
|---|---:|---:|---:|---:|
| Solar fixed | 15 min | 6 h | OFF | 24 h |
| Primary-battery fixed | 1 h | 24 h | OFF | 24 h |
| Vehicle mobile | OFF | 1 h | 24 h | 5 sec |
| Hybrid | 15 min | 1 h | 24 h | 5 sec |

`[documented:D1 V2 guide revision 2022-07-06 illustrative configurations conf:high]`

<a id="operations-md--scenarios-and-lifecycle"></a>

### Scenarios and lifecycle

- Installation, weather and charging context: see [product.md](#product-md), [Field deployment](#product-md--field-deployment).
- Potential detector failure: see [notefiles.md](#notefiles-md), [Public fault example](#notefiles-md--public-fault-example).
- Buffering: see Configuration controls. Buffer capacity, overflow policy and exact idle timing remain unresolved.
- Measurement averaging: see [notefiles.md](#notefiles-md), [Measurement mechanism](#notefiles-md--measurement-mechanism). Exact silence limits, maintenance, replacements and resets remain open; see Q4, Q7, Q11 and Q12. No fleet-wide silence threshold has been established.

<a id="operations-md--receiving-surface-and-missing-dimensions"></a>

### Receiving surface and missing dimensions

Public receiving surfaces and their limitations: see [product.md](#product-md), [Receiving surfaces](#product-md--receiving-surfaces) and [Public interpretation](#product-md--public-interpretation). The actual route wiring, alert delivery, acknowledgment and field response have not been established. External weather, official decisions, maintenance records and the mapping from public stations to devices cannot be assumed to exist in this project's data; see [questions.md](#questions-md), [Q3](#questions-md--q3-query-specific-needs) and [Q9](#questions-md--q9-screen-semantics-versus-measurement).

<a id="operations-md--external-survey-delivery"></a>

### External survey delivery

These are capabilities of the guide's described integration, not evidence this project sends data there.

- Fixed survey delivery takes `_air.qo` data as described in [notefiles.md](#notefiles-md). `rad_name` and `rad_description` provide a readable label and location description. Delivery requires GPS coordinates or a configured address; the address keys are `rad_street1`, `rad_street2`, `rad_city`, `rad_state` and `rad_zip`. `[documented:D1 V2 guide revision 2022-07-06 integration scope conf:high]`
- Mobile delivery requires tracking mode, latitude and longitude for each measurement, and an assigned serial using `_sn`. A measurement lacking the required location or identity cannot be delivered by that integration. This is not evidence the source measurement was never recorded. `[documented:D1 V2 guide revision 2022-07-06 integration scope conf:high]`
- The receiving service groups mobile readings into surveys within a selected event; one survey's identifying label includes journey count, journey identity and device identity. These grouping meanings must be kept distinct from individual measurements; see [notefiles.md](#notefiles-md), [_track.qo](#notefiles-md--track-qo). `[documented:D1 V2 guide revision 2022-07-06 integration scope conf:high]`
- Display unit labels and wire units require reconciliation; see [questions.md](#questions-md), [A2](#questions-md--a2-fixed-survey-unit-label-omits-a-rate-denominator)/Q13. No external setup, delivery or alert configuration has been verified for this project.

<a id="operations-md--v3-configuration-and-cadence"></a>

### V3 configuration and cadence

Scope: restricted V3 firmware 3.2.2, reviewed 2026-09-25; not yet deployed in the population under discussion. This describes this version's interface behavior, not active V2 settings. Every rule in this section has this scope. `[documented:F1 V3 firmware 3.2.2 reviewed 2026-09-25 conf:high]` `[scope: v3 firmware, not yet deployed]`

| API-visible setting | Meaning and default in this version | Limit |
|---|---|---|
| `sample_mins` | Fixed sampling interval in whole-number minutes; compiled voltage-mode defaults: USB 5, high/normal 15, low 60, unmatched mode 0. | These are defaults, not observed device settings. |
| `sample_mins_ntn` | Fixed sampling override for eligible active satellite transport, in minutes. | Advertised unset. Eligibility requires a reported uplink capacity of at least 10,000 bytes. |
| `sample_mins_primary` | Fixed sampling override while primary power is reported, in minutes. | Advertised unset. |
| `sample_mins_usb` | Fixed sampling override while USB is reported, in minutes. | Advertised unset. |
| `alarm_usv` | Fixed alarm dose-rate threshold, µSv/h; default 0 disables new alarm decisions. | Decision and retained-state semantics belong to [notefiles.md](#notefiles-md), [V3 fixed alarm meaning](#notefiles-md--v3-fixed-alarm-meaning). |
| `_sync_outbound_mins`, `_sync_outbound_mins_ntn`, `_sync_outbound_mins_usb`, `_sync_outbound_mins_primary` | Upload scheduling, in minutes or voltage-mode selections; advertised unset. Firmware fallback: USB 60 min, high/normal/low 720 min, otherwise 0. | Module-owned scheduling; USB can also attempt continuous connection. Not a rigid arrival interval. |
| `_sync_inbound_mins`, `_sync_inbound_mins_ntn`, `_sync_inbound_mins_usb`, `_sync_inbound_mins_primary` | Inbound scheduling in minutes or voltage-mode selections; advertised unset. Firmware fallback: 10,080 minutes. | Module-owned scheduling, not guaranteed cloud-to-device latency. |
| `_gps_secs` | Location-update setting in seconds; advertised unset. Initial periodic request is 86,400 seconds. | Survey tracking requests its own cadence. |
| `rad_data`, `rad_data_ntn` | Names for rich and compact fixed-data streams. Defaults and collision handling belong to [notefiles.md](#notefiles-md), [V3 fixed stream selection](#notefiles-md--v3-fixed-stream-selection). | Names alone do not prove revision or schema. |
| `_sn` | Configured device serial string. | No particular serial is retained in this training. |

For sampling, a single integer or voltage-mode selection may be supplied; values are whole-number minutes. Exact `-` and empty values fall through. The first applicable nonnegative selection wins: eligible satellite, primary, USB, base, then compiled default. Negative selections fall through; positive integer selections 1–4 become five minutes. Fractional input is not supported; a value such as 0.5 is parsed as zero. A selected zero produces a two-year effective interval. A period change can still initiate one sample, and an in-progress attempt is not canceled solely by this change. Therefore zero is not an unconditional immediate-stop guarantee. Textual OFF and malformed strings are not validated encodings to recommend.

The source describes a different module-owned sync priority: satellite, USB, primary, base, then configured fallback. That module implementation and its applicable version were not supplied; Q15 remains open. Neither cascade establishes device/fleet/project environment precedence.

Fixed sampling is scheduled start-to-start using elapsed time; duration is owned by [notefiles.md](#notefiles-md), [V3 integration and timestamps](#notefiles-md--v3-integration-and-timestamps). If stabilization and measurement take longer than the selected interval, no backfilled measurements are manufactured. Changed settings can reset the schedule. Failed local record submission can lose that attempt; the same measured values are not requeued by this host. A successful local submission is not proof of cloud receipt.

The host consumes environment values already delivered to the device. Environment/USB changes can wake the sampler. Polling while awake and wake notifications do not establish when a remote setting arrived or was applied. Defaults exposed in an environment view are not a receipt acknowledgment.

Only `sample_mins_ntn`, `alarm_usv`, `_sync_outbound_mins_ntn` and `_sync_inbound_mins_ntn` are declared for satellite environment delivery in this version. Other settings and template registration require the ordinary session path. Actual module capability and provisioned templates must be established before interpreting a missing change.

<a id="operations-md--v3-survey-and-delivery-behavior"></a>

### V3 survey and delivery behavior

`[documented:F1 V3 firmware 3.2.2 reviewed 2026-09-25 conf:high]` `[scope: v3 firmware, not yet deployed]`

Survey selection and transport preference live in `vars.db`; see [notefiles.md](#notefiles-md), [vars.db](#notefiles-md--vars-db). They are not the V2 interval-switch settings. In survey mode, the host collects radiation while the module reports an active journey. Ending a journey or changing modes abandons an incomplete integration. Stationary survey mode does not become fixed sampling in this state machine.

This version requests five-second track points and a one-hour heartbeat. Completed radiation integrations are supplied separately; see [notefiles.md](#notefiles-md), [V3 tracking fields](#notefiles-md--v3-tracking-fields). Thus a track point does not establish a fresh measurement at that point, and the V2 guide's rolling-average description must not be transplanted to this version.

The firmware configures ordinary connectivity with optional satellite fallback when capable hardware is present. This does not settle which V2 devices have that capability (Q10). Fixed records use the transport-specific streams described in [notefiles.md](#notefiles-md). A boot sync and eligible alarm transition can request earlier delivery; requests do not establish arrival time, recipient notification or acknowledgment.

---

<a id="notefiles-md"></a>

## Notefile inventory

*`notefiles.md` | kinds: notefiles, derivation, sentinels | 2026-09-25 14:01 | 43025 bytes*

```yaml
kind: notefiles,derivation,sentinels
description: Observed Notefile roster with semantic inventory pending
updated: 2026-09-25T18:01:36Z
sources:
  - P16: public service API schema, reviewed 2026-09-25
  - H4: trainer, V2/V3 radiation-conversion equivalence reported 2026-09-25; read-back pending
  - P15: public module telemetry API reference, reviewed 2026-09-25
  - P14: public module encoding guide, reviewed 2026-09-25
  - P13: public module note API reference, reviewed 2026-09-25
  - H3: trainer, fixed-reading inclusion confirmed by enumerated read-back 2026-09-25
  - H2: trainer, enumerated guide read-back confirmed 2026-09-25
  - E3: refreshed complete project schema response, reviewed 2026-09-25
  - F1: restricted V3 host firmware, version 3.2.2, reviewed 2026-09-25
  - E1: project schema response, reviewed 2026-09-25
  - E2: bounded project event sample, reviewed 2026-09-25
  - P5: public technical article on device faults, public, published 2025-07, reviewed 2026-09-25
  - D1: public owner guide for the V2 product, public, revised 2022-07-06, reviewed 2026-09-25
```

Partial inventory. Public documentation supplies field meanings for `_air.qo` and `_track.qo`, and guide-scoped measurement mechanics. The trainer has established V2/V3 equivalence of radiation conversions and pulse-count meaning; see Shared radiation conversion. Full V2 wire encoding, timing and validity remain open. All schema-reported body paths have entries below, including explicit unknowns. Every unknown schema row is observed in E3 (2026-09-25, high confidence in the name/type only); its meaning, units, range, absence and validity remain unverified. V3 host-written payloads and template-only metadata have entries. Module-generated field coverage, actual V2 decoding and event-envelope coverage remain incomplete.

<a id="notefiles-md--sources-walked"></a>

### Sources walked

- Schema response: all 15 returned names included. `[observed:E1 2026-09-25 all schemas returned conf:high]`
- Event sample: five latest events in descending capture order, no time bound, more available; `_session.qo` and `_air.qo` both already in the roster. This sample establishes neither cadence nor fleet coverage. `[observed:E2 2026-09-25 n=5 has_more=true conf:high]`
- Public technical article: `_air.qo` and `_session.qo`, already on the roster. No new Notefile names.
- Public V2 owner guide: `_air.qo` and `_track.qo`, already on the roster. It also describes an unnamed tracking heartbeat; Q12 remains open.
- Restricted V3 firmware: adds `data.qo`, `data_ntn.qo`, `vars.db` and `_synclog.qi`; also names `_hub_management` in a configuration-storage comment. These five names were absent from both schema responses. Configurable names may add more at deployment.
- Refreshed schema response: same 15 names, with all returned body-property paths enumerated. `[observed:E3 2026-09-25 complete unpaged response conf:high]`
- Public API references were used for encoding and telemetry units; their generic example filenames are not evidence of product files.
- Existing skills: none were present at the start of this run.

<a id="notefiles-md--roster"></a>

### Roster

- `_req.qis` — E1
- `_health_host.qo` — E1
- `_rsp.qos` — E1
- `_temp.qo` — E1
- `sensor.qo` — E1
- `geofeed.db` — E1
- `_log.qo` — E1
- `_web.dbx` — E1
- `_health.qo` — E1
- `_session.qo` — E1, E2, P5
- `_watchdog.qo` — E1
- `_geolocate.qo` — E1
- `_env.dbs` — E1
- `_track.qo` — E1, D1
- `_air.qo` — E1, E2, P5, D1, F1
- `data.qo` — F1
- `data_ntn.qo` — F1
- `vars.db` — F1
- `_synclog.qi` — F1
- `_hub_management` — F1 (comment reference)

<a id="notefiles-md--fixed-reading-inclusion-policy"></a>

### Fixed-reading inclusion policy

Zero or missing radiation readings in `_air.qo`, `data.qo` and `data_ntn.qo` are invalid data readings and should be ignored in radiation analysis. `[stated:H3 trainer 2026-09-25 conf:high]`

Apply this to radiation measurement values, not every optional diagnostic field. Retain records when investigating invalid-reading incidence or device health; do not silently count them as valid radiation samples or replace missing values with zero in an average. The instruction does not classify survey `_track.qo` readings, positive saturated lower bounds, or absent housekeeping flags. A valid-sample count and an invalid/absent-data count are different denominators. For period statistics, exclude invalid readings from radiation values while reporting missing coverage separately. For a latest-reading answer, inspect the latest record first: an invalid latest result supplies no valid latest measurement. An older valid value may be labelled last known valid with its time, but must not be presented as a fresh current reading. The exact treatment of a partly populated radiation tuple is Q14.

<a id="notefiles-md--measurement-mechanism"></a>

### Measurement mechanism

Scope: D1, V2 guide revised 2022-07-06; applicability to later builds and active configuration is Q11. These rules describe measurements, not instantaneous values or guarantees between windows.

- After the detector power supply starts, the guide allows 30 seconds for stabilization before sampling. For periodic intervals greater than five minutes, it describes a five-minute integration and an averaged reported rate. Periodic intervals of five minutes or less are not fully specified. `[documented:D1 V2 guide revision 2022-07-06 conf:high]`
- Tracking uses a rolling one-minute average, so consecutive readings may overlap rather than represent independent intervals. `[stated:H2 2026-09-25 V2 guide scope]` The guide describes twelve five-second buckets. `[documented:D1 V2 guide revision 2022-07-06 conf:high]`
- The guide does not define whether an event timestamp is the start or end of its integration window, nor a conversion from these averages into time continuously above a threshold. Such duration claims remain unsupported. See Q4 and Q11.

<a id="notefiles-md--event-clocks-and-location"></a>

### Event clocks and location

These are service event-envelope fields, separate from `body` fields and applicable when returned. `[documented:P16 public service API schema reviewed 2026-09-25 conf:high]` They are not a guarantee of clock accuracy, current position or location precision.

| Field path | Type | Meaning and limit |
|---|---|---|
| `when` | Integer timestamp | Capture time on the device; chosen radiation-analysis clock. It is not necessarily an exact integration boundary. `[rungs:1 partial]` |
| `received` | Numeric timestamp | Receipt time at the service. Late arrival does not move capture time into a newer measurement window. `[rungs:1]` |
| `best_lat` | Number | Event latitude; assess source and age before geographic inclusion. `[rungs:1 partial]` |
| `best_lon` | Number | Event longitude; same location qualification as latitude. `[rungs:1 partial]` |
| `best_location_type` | String enum | `gps`, `triangulated` or `tower` in the schema; they do not imply equal precision. Unknown types need clarification. `[rungs:1 partial]` |
| `best_location_when` | Integer timestamp | Timestamp associated with the event's selected location. It can differ from capture time; exact freshness guarantees are unestablished. `[rungs:1 partial]` |
| `best_country` | String | Country label, a coarse geographic descriptor rather than a precise boundary test. `[rungs:1 partial]` |
| `best_location` | String | Readable location label; not a unique site identity. `[rungs:1 partial]` |
| `best_timezone` | String | Location-related timezone metadata; query timezone follows [operations.md](#operations-md), [Time windows](#operations-md--time-windows). `[rungs:1 partial]` |
| `event` | String | Event identity for avoiding duplicate retrievals; values are not stored in these skills. Separate transport representations may have different identities. `[rungs:1 partial]` |
| `device` | String | Originating device identity for grouping; joining changed identities needs an established mapping. `[rungs:1 partial]` |
| `file` | String | Notefile name, needed alongside revision/configuration to choose the decoder. `[rungs:1 partial]` |

Missing capture or location values are unknown evidence; do not fill with arrival time or a newer position. An event position can be reused while stationary, and mobile integrations span time; these fields alone do not prove every pulse was measured at one point. Selection rules belong to [operations.md](#operations-md), [Time windows](#operations-md--time-windows) and [Geographic selection](#operations-md--geographic-selection). This is a selected envelope inventory, not all service metadata.

<a id="notefiles-md--session-evidence"></a>

### Session evidence

A service session record exposes integer Unix timestamps `session_began` (start), `session_ended` (end) and `work` (latest work for that session). `[documented:P16 public service API schema reviewed 2026-09-25 conf:high]` `[rungs:1 partial]` A device's `last_activity` property is not described precisely enough in the reviewed schema to substitute for session evidence without validation. Zero/missing end values do not alone prove an ended session.

Apply [operations.md](#operations-md), [Population](#operations-md--population) to sufficiently complete session evidence. Session start, continuation and end are different evidence of activity; an ongoing session can have started before the lookback. A `_session.qo` event also has the general capture/receipt clocks; its body `opened`/`closed` flags and clocks still require their own semantics. Neither a session nor a recently received event guarantees a recent valid radiation reading.

<a id="notefiles-md--entries"></a>

### Entries

<a id="notefiles-md--req-qis"></a>

#### _req.qis

Observed name only; semantic and field inventory pending Q4 in [questions.md](#questions-md). No decision rule is available. `[observed:E1 2026-09-25 schema roster conf:high]`

The refreshed schema returned no property list for this name; this is not evidence that notes have empty bodies. `[observed:E3 2026-09-25 conf:high]`

<a id="notefiles-md--health-host-qo"></a>

#### _health_host.qo

Observed name only; semantic and field inventory pending Q4 in [questions.md](#questions-md). No decision rule is available. `[observed:E1 2026-09-25 schema roster conf:high]`

The refreshed schema returned no property list for this name; this is not evidence that notes have empty bodies. `[observed:E3 2026-09-25 conf:high]`

<a id="notefiles-md--rsp-qos"></a>

#### _rsp.qos

Observed name only; semantic and field inventory pending Q4 in [questions.md](#questions-md). No decision rule is available. `[observed:E1 2026-09-25 schema roster conf:high]`

<a id="notefiles-md--schema-paths-still-requiring-v2-decoding"></a>

##### Schema paths still requiring V2 decoding

| Path | Observed JSON type | Meaning and gap |
|---|---|---|
| `body.cell` | boolean | Unknown semantics; Q4/Q14. `[rungs:none]` |
| `body.connected` | boolean | Unknown semantics; Q4/Q14. `[rungs:none]` |
| `body.status` | string | Unknown semantics; Q4/Q14. `[rungs:none]` |
| `body.storage` | number | Unknown semantics; Q4/Q14. `[rungs:none]` |
| `body.time` | number | Unknown semantics; Q4/Q14. `[rungs:none]` |
| `body.usb` | boolean | Unknown semantics; Q4/Q14. `[rungs:none]` |

<a id="notefiles-md--temp-qo"></a>

#### _temp.qo

Observed name only; semantic and field inventory pending Q4 in [questions.md](#questions-md). No decision rule is available. `[observed:E1 2026-09-25 schema roster conf:high]`

The refreshed schema returned no property list for this name; this is not evidence that notes have empty bodies. `[observed:E3 2026-09-25 conf:high]`

<a id="notefiles-md--sensor-qo"></a>

#### sensor.qo

Observed name only; semantic and field inventory pending Q4 in [questions.md](#questions-md). No decision rule is available. `[observed:E1 2026-09-25 schema roster conf:high]`

The refreshed schema returned no property list for this name; this is not evidence that notes have empty bodies. `[observed:E3 2026-09-25 conf:high]`

<a id="notefiles-md--geofeed-db"></a>

#### geofeed.db

Observed name only; semantic and field inventory pending Q4 in [questions.md](#questions-md). No decision rule is available. `[observed:E1 2026-09-25 schema roster conf:high]`

<a id="notefiles-md--schema-paths-still-requiring-v2-decoding"></a>

##### Schema paths still requiring V2 decoding

| Path | Observed JSON type | Meaning and gap |
|---|---|---|
| `body.alert` | boolean | Unknown semantics; Q4/Q14. `[rungs:none]` |
| `body.captured` | number | Unknown semantics; Q4/Q14. `[rungs:none]` |
| `body.count` | number | Unknown semantics; Q4/Q14. `[rungs:none]` |
| `body.lat` | number | Unknown semantics; Q4/Q14. `[rungs:none]` |
| `body.lon` | number | Unknown semantics; Q4/Q14. `[rungs:none]` |
| `body.outbound_mins` | number | Unknown semantics; Q4/Q14. `[rungs:none]` |
| `body.radius_meters` | number | Unknown semantics; Q4/Q14. `[rungs:none]` |
| `body.sample_mins` | number | Unknown semantics; Q4/Q14. `[rungs:none]` |
| `body.usv_avg` | number | Unknown semantics; Q4/Q14. `[rungs:none]` |
| `body.usv_max` | number | Unknown semantics; Q4/Q14. `[rungs:none]` |
| `body.usv_min` | number | Unknown semantics; Q4/Q14. `[rungs:none]` |
| `body.warning` | boolean | Unknown semantics; Q4/Q14. `[rungs:none]` |

<a id="notefiles-md--log-qo"></a>

#### _log.qo

Observed name only; semantic and field inventory pending Q4 in [questions.md](#questions-md). No decision rule is available. `[observed:E1 2026-09-25 schema roster conf:high]`

<a id="notefiles-md--schema-paths-still-requiring-v2-decoding"></a>

##### Schema paths still requiring V2 decoding

| Path | Observed JSON type | Meaning and gap |
|---|---|---|
| `body.milliamp_hours` | number | Unknown semantics; Q4/Q14. `[rungs:none]` |
| `body.text` | string | Unknown semantics; Q4/Q14. `[rungs:none]` |
| `body.voltage` | number | Unknown semantics; Q4/Q14. `[rungs:none]` |

<a id="notefiles-md--web-dbx"></a>

#### _web.dbx

Observed name only; semantic and field inventory pending Q4 in [questions.md](#questions-md). No decision rule is available. `[observed:E1 2026-09-25 schema roster conf:high]`

The refreshed schema returned no property list for this name; this is not evidence that notes have empty bodies. `[observed:E3 2026-09-25 conf:high]`

<a id="notefiles-md--health-qo"></a>

#### _health.qo

Observed name only; semantic and field inventory pending Q4 in [questions.md](#questions-md). No decision rule is available. `[observed:E1 2026-09-25 schema roster conf:high]`

<a id="notefiles-md--schema-paths-still-requiring-v2-decoding"></a>

##### Schema paths still requiring V2 decoding

| Path | Observed JSON type | Meaning and gap |
|---|---|---|
| `body.method` | string | Unknown semantics; Q4/Q14. `[rungs:none]` |
| `body.milliamp_hours` | number | Unknown semantics; Q4/Q14. `[rungs:none]` |
| `body.text` | string | Unknown semantics; Q4/Q14. `[rungs:none]` |
| `body.voltage` | number | Unknown semantics; Q4/Q14. `[rungs:none]` |
| `body.voltage_mode` | string | Unknown semantics; Q4/Q14. `[rungs:none]` |

<a id="notefiles-md--session-qo"></a>

#### _session.qo

Body inventory is pending Q4. A public technical article additionally names top-level `orientation`; its interpretation for Radnote mounting remains open in Q8. `[observed:E1 2026-09-25 schema roster conf:high]` `[documented:P5 2025-07 article scope conf:medium]` `[rungs:none]`

<a id="notefiles-md--schema-paths-still-requiring-v2-decoding"></a>

##### Schema paths still requiring V2 decoding

| Path | Observed JSON type | Meaning and gap |
|---|---|---|
| `body.closed` | boolean | Unknown semantics; Q4/Q14. `[rungs:none]` |
| `body.opened` | boolean | Unknown semantics; Q4/Q14. `[rungs:none]` |
| `body.why` | string | Unknown semantics; Q4/Q14. `[rungs:none]` |

<a id="notefiles-md--watchdog-qo"></a>

#### _watchdog.qo

Observed name only; semantic and field inventory pending Q4 in [questions.md](#questions-md). No decision rule is available. `[observed:E1 2026-09-25 schema roster conf:high]`

<a id="notefiles-md--schema-paths-still-requiring-v2-decoding"></a>

##### Schema paths still requiring V2 decoding

| Path | Observed JSON type | Meaning and gap |
|---|---|---|
| `body.activity_event` | number | Unknown semantics; Q4/Q14. `[rungs:none]` |
| `body.activity_session` | number | Unknown semantics; Q4/Q14. `[rungs:none]` |
| `body.fleet` | string | Unknown semantics; Q4/Q14. `[rungs:none]` |
| `body.fleet_name` | string | Unknown semantics; Q4/Q14. `[rungs:none]` |
| `body.fleet_watchdog_mins` | number | Unknown semantics; Q4/Q14. `[rungs:none]` |

<a id="notefiles-md--geolocate-qo"></a>

#### _geolocate.qo

Observed name only; semantic and field inventory pending Q4 in [questions.md](#questions-md). No decision rule is available. `[observed:E1 2026-09-25 schema roster conf:high]`

<a id="notefiles-md--schema-paths-still-requiring-v2-decoding"></a>

##### Schema paths still requiring V2 decoding

| Path | Observed JSON type | Meaning and gap |
|---|---|---|
| `body.country` | string | Unknown semantics; Q4/Q14. `[rungs:none]` |
| `body.location` | string | Unknown semantics; Q4/Q14. `[rungs:none]` |
| `body.radios` | object | Unknown semantics; Q4/Q14. `[rungs:none]` |
| `body.radios.wifi` | number | Unknown semantics; Q4/Q14. `[rungs:none]` |

<a id="notefiles-md--env-dbs"></a>

#### _env.dbs

Observed name only; semantic and field inventory pending Q4 in [questions.md](#questions-md). No decision rule is available. `[observed:E1 2026-09-25 schema roster conf:high]`

The refreshed schema returned no property list for this name; this is not evidence that notes have empty bodies. `[observed:E3 2026-09-25 conf:high]`

<a id="notefiles-md--track-qo"></a>

#### _track.qo

For the separately scoped V3 host-supplied body, see V3 tracking fields.

Carries device-origin mobile radiation measurements associated with journeys in the guide's external-survey integration. Journey cadence and motion gating: see [operations.md](#operations-md), [Configuration controls](#operations-md--configuration-controls) and [Location and motion](#operations-md--location-and-motion). Averaging: see Measurement mechanism. Exact writer, templates and measurement-time field for this file have not been traced. `[documented:D1 V2 guide revision 2022-07-06 integration scope conf:high]`

| Field path | Meaning, units and scope | Rungs and remaining gaps |
|---|---|---|
| `body.jcount` | Numeric journey counter; advances with each new journey on that device. | `[documented:D1 V2 guide revision 2022-07-06 conf:high]` `[observed:E1 2026-09-25 schema conf:high]` `[rungs:1 partial]` Reset, persistence, range, sentinels: Q7/Q11. |
| `body.journey` | Numeric identity shared by the journey's readings; also Unix seconds for the journey's start. It is not each reading's measurement time. | `[documented:D1 V2 guide revision 2022-07-06 conf:high]` `[observed:E1 2026-09-25 schema conf:high]` `[rungs:1 partial]` Clock validity, collisions and resets: Q7/Q11. |
| `body.cpm` | Numeric gamma count rate in counts/minute. The downstream counts label does not make it an accumulated count. | `[documented:D1 V2 guide revision 2022-07-06 integration scope conf:high]` `[observed:E1 2026-09-25 schema conf:high]` `[rungs:1 partial]` Shared conversion: see Shared radiation conversion. Window, validity and low counts remain Q4/Q8/Q15. |

Aliases: both this file and `_air.qo` use `body.cpm`, but their measurement windows may differ; see Measurement mechanism. Do not silently combine fixed and moving measurements under one interval assumption.

D1 does not name the mobile integration's latitude/longitude paths. Service envelope candidates are documented in Event clocks and location; their use by that integration and capture-time relation remain Q9/Q11. Do not substitute an arbitrary available position.


<a id="notefiles-md--schema-paths-still-requiring-v2-decoding"></a>

##### Schema paths still requiring V2 decoding

| Path | Observed JSON type | Meaning and gap |
|---|---|---|
| `body.bearing` | number | Unknown semantics; Q4/Q14. `[rungs:none]` |
| `body.charging` | boolean | Unknown semantics; Q4/Q14. `[rungs:none]` |
| `body.cpm_count` | number, integral meaning | Raw pulse total associated with the radiation measurement, not a journey total; see Shared radiation conversion. Tracking window, encoding and validity remain separate questions. `[observed:E3 2026-09-25 schema conf:high]` `[rungs:1 partial]` |
| `body.cpm_secs` | number | Unknown semantics; Q4/Q14. `[rungs:none]` |
| `body.daily_charging_mins` | number | Unknown semantics; Q4/Q14. `[rungs:none]` |
| `body.distance` | number | Unknown semantics; Q4/Q14. `[rungs:none]` |
| `body.dop` | number | Unknown semantics; Q4/Q14. `[rungs:none]` |
| `body.hdop` | number | Unknown semantics; Q4/Q14. `[rungs:none]` |
| `body.motion` | number | Unknown semantics; Q4/Q14. `[rungs:none]` |
| `body.seconds` | number | Unknown semantics; Q4/Q14. `[rungs:none]` |
| `body.sensor` | string | Unknown semantics; Q4/Q14. `[rungs:none]` |
| `body.status` | string | Unknown semantics; Q4/Q14. `[rungs:none]` |
| `body.temperature` | number | Unknown semantics; Q4/Q14. `[rungs:none]` |
| `body.time` | number | Unknown semantics; Q4/Q14. `[rungs:none]` |
| `body.usb` | boolean | Unknown semantics; Q4/Q14. `[rungs:none]` |
| `body.usv` | number | Derived dose rate in µSv/h; see Shared radiation conversion. Tracking-specific missing/zero, window and encoding behavior remain Q14/Q15. `[observed:E3 2026-09-25 schema conf:high]` `[rungs:1 partial]` |
| `body.velocity` | number | Unknown semantics; Q4/Q14. `[rungs:none]` |
| `body.voltage` | number | Unknown semantics; Q4/Q14. `[rungs:none]` |

<a id="notefiles-md--air-qo"></a>

#### _air.qo

For a V3 device configured to use this name, see V3 fixed stream selection; do not select its decoder from the name alone.

The guide identifies this file as device-origin radiation telemetry used by the fixed-survey integration. Scheduling and buffering: see [operations.md](#operations-md), [Configuration controls](#operations-md--configuration-controls). Averaging: see Measurement mechanism. Exact writer and templates have not been traced. `[documented:D1 V2 guide revision 2022-07-06 integration scope conf:high]`

The documented fixed integration uses event-level `when` as its reading timestamp. That is not an arrival-time field; D1 does not establish start/end-of-integration semantics or behavior before clock acquisition. The service arrival field belongs to Event clocks and location; device-clock validity and exact integration boundaries remain Q11. `[documented:D1 V2 guide revision 2022-07-06 integration scope conf:high]`

| Field path | Meaning and limit | Evidence and coverage |
|---|---|---|
| `body.cpm` | Detector count rate in counts per minute; conversion is in Shared radiation conversion and guide-scoped averaging is in Measurement mechanism. Exact deployed integration and the physical causes of zero remain unresolved; analysis inclusion is governed by Fixed-reading inclusion policy. | `[documented:P5 2025-07 example scope conf:medium]` `[rungs:1 partial]` Q4, Q8 |
| `body.usv` | Derived dose rate in µSv/h in the technical article and device-display description; the external fixed-survey guide label omits the rate denominator (A2/Q13). Device conversion is now described in Shared radiation conversion; the downstream transformation remains Q13. | `[documented:P5,D1 technical article and V2 display scope conf:medium]` `[rungs:1 partial]` Q4, Q8, Q13 |
| `body.sensor` | Sensor designation; full mapping and allowed values unresolved. | `[documented:P5 2025-07 example scope conf:medium]` `[rungs:1 partial]` Q4, Q8 |

Wire observations: `cpm` and `usv` are numbers; `sensor` is a string in the schema and bounded sample. This does not establish template width or precision. `[observed:E1,E2 2026-09-25 schemas and n=5 latest events; has_more=true conf:high]`

<a id="notefiles-md--public-fault-example"></a>

##### Public fault example

The published example flags sensor-bearing events with absent or zero `cpm` for hardware investigation. This does not establish a universal detector-failure diagnosis. Analysis inclusion is governed separately by Fixed-reading inclusion policy. `[documented:P5 2025-07 example scope conf:medium]` `[rungs:2 partial]`


<a id="notefiles-md--schema-paths-still-requiring-v2-decoding"></a>

##### Schema paths still requiring V2 decoding

| Path | Observed JSON type | Meaning and gap |
|---|---|---|
| `body.c00_30` | number | Unknown semantics; Q4/Q14. `[rungs:none]` |
| `body.c00_50` | number | Unknown semantics; Q4/Q14. `[rungs:none]` |
| `body.c01_00` | number | Unknown semantics; Q4/Q14. `[rungs:none]` |
| `body.c02_50` | number | Unknown semantics; Q4/Q14. `[rungs:none]` |
| `body.c05_00` | number | Unknown semantics; Q4/Q14. `[rungs:none]` |
| `body.c10_00` | number | Unknown semantics; Q4/Q14. `[rungs:none]` |
| `body.charging` | boolean | Unknown semantics; Q4/Q14. `[rungs:none]` |
| `body.cpm_count` | number, integral meaning | Raw pulse total for the measurement window; see Shared radiation conversion. V2 storage width, reset/overflow and window details remain Q7/Q11/Q15. `[observed:E3 2026-09-25 schema conf:high]` `[rungs:1 partial]` |
| `body.csecs` | number | Duration associated with the radiation count/rate calculation; seconds in the reviewed V3 path. Exact V2 representation, truncation and window boundaries remain Q11/Q15; conversion equivalence alone does not establish these. `[observed:E3 2026-09-25 schema conf:high]` `[rungs:1 partial]` |
| `body.format` | number | Unknown semantics; Q4/Q14. `[rungs:none]` |
| `body.humidity` | number | Unknown semantics; Q4/Q14. `[rungs:none]` |
| `body.motion` | number | Unknown semantics; Q4/Q14. `[rungs:none]` |
| `body.pm01_0` | number | Unknown semantics; Q4/Q14. `[rungs:none]` |
| `body.pm02_5` | number | Unknown semantics; Q4/Q14. `[rungs:none]` |
| `body.pm10_0` | number | Unknown semantics; Q4/Q14. `[rungs:none]` |
| `body.pressure` | number | Unknown semantics; Q4/Q14. `[rungs:none]` |
| `body.storage` | string | Unknown semantics; Q4/Q14. `[rungs:none]` |
| `body.temperature` | number | Unknown semantics; Q4/Q14. `[rungs:none]` |
| `body.template_body` | string | Unknown semantics; Q4/Q14. `[rungs:none]` |
| `body.usb` | boolean | Unknown semantics; Q4/Q14. `[rungs:none]` |
| `body.voltage` | number | Unknown semantics; Q4/Q14. `[rungs:none]` |

<a id="notefiles-md--data-qo"></a>

#### data.qo

Default V3 rich fixed radiation stream, outbound and written by the host after each completed measurement attempt, including unreadable attempts. Name may be overridden; see V3 fixed stream selection. Templated; every declared field is listed under V3 rich fixed fields. No events from this default name were in the schema evidence. Cadence belongs to [operations.md](#operations-md), [V3 configuration and cadence](#operations-md--v3-configuration-and-cadence). Measurement windows and timestamp limitations belong to V3 integration and timestamps. `[documented:F1 V3 firmware 3.2.2 reviewed 2026-09-25 conf:high]` `[scope: v3 firmware, not yet deployed]`

<a id="notefiles-md--data-ntn-qo"></a>

#### data_ntn.qo

Default V3 compact fixed radiation stream, outbound and written alongside the rich representation of the same attempt. It does not represent a second independent measurement. Name may be overridden; see V3 fixed stream selection. Its template declares `csecs`, `usv`, `cpm`, `cpm_count`, `_time` and `_loc5`; field entries follow below. No events from this default name were in the schema evidence. Silence in one transport-specific file alone does not prove measurement stopped. `[documented:F1 V3 firmware 3.2.2 reviewed 2026-09-25 conf:high]` `[scope: v3 firmware, not yet deployed]`

<a id="notefiles-md--vars-db"></a>

#### vars.db

A synchronized settings database named in the V3 firmware; settings can affect operating mode and transport. It was not in the schema evidence. Updates and reads use variable APIs, so the final database-body shape is not independently verified. Module handling, version and sync timing remain Q15. `[documented:F1 V3 firmware 3.2.2 reviewed 2026-09-25 conf:high]` `[scope: v3 firmware, not yet deployed]`

| Note/key | API-visible value | Meaning and limit |
|---|---|---|
| `survey` | Boolean `flag` | Selects survey mode when true; missing entry defaults to false. An error response sets the cached selection false, potentially selecting fixed mode; no response retains the cache. Neither establishes remote contents. `[rungs:1 partial]` |
| `transport` | String `text` | `-` means automatic selection, `wifi`, `cell` and `ntn` choose a transport preference. Missing satellite capability makes the latter resolve to automatic. A preference is not proof of the active path. Read failures preserve the cached preference. `[rungs:1 partial]` |

The executable path re-reads and applies synchronized transport changes; an older local-only comment is not adopted as a rule. There is no periodic radiation cadence inherent to this file.

<a id="notefiles-md--synclog-qi"></a>

#### _synclog.qi

Named in V3 as an incoming module diagnostic queue consumed locally. It was not in the schema evidence and is not a cloud radiation-data source for this lesson. Body schema and remote availability are unverified; no operational silence rule is assigned. `[documented:F1 V3 firmware 3.2.2 reviewed 2026-09-25 conf:high]` `[scope: v3 firmware, not yet deployed]`

<a id="notefiles-md--hub-management"></a>

#### _hub_management

Named only in a V3 source comment about persistent environment defaults. No direct host writer or schema/event evidence establishes its interface here. It is outside the radiation decoder; no fields or arrival expectations are inferred from the comment. `[documented:F1 V3 firmware 3.2.2 comment scope reviewed 2026-09-25 conf:low]`

<a id="notefiles-md--v3-fixed-stream-selection"></a>

### V3 fixed stream selection

`[documented:F1 V3 firmware 3.2.2 reviewed 2026-09-25 conf:high]` `[scope: v3 firmware, not yet deployed]`

`rad_data` selects the rich fixed stream, default `data.qo`; `rad_data_ntn` selects the compact stream, default `data_ntn.qo`. `_air.qo` is a possible configured alias, not this checkout's compiled default. Actual device configuration is required before choosing a decoder. If both names resolve equal, the compact name gains an `ntn_` prefix. These rules describe name families; an actual configured name must be added to the roster when observed.

Each attempt is offered to both templates. Their transport markers request retention of the matching representation and deletion of the other at synchronization. Do not add the two streams as independent measurements. This behavior depends on module/template support, and successful local submissions are not cloud-delivery acknowledgments. `[documented:F1,P13 V3 firmware and public API reviewed 2026-09-25 conf:high]`

A configured compact name can include a numeric port suffix in 2–99. Without a valid suffix the version derives a port; uniqueness is not guaranteed. A malformed suffix may remain part of the name. Port/template compatibility and actual provisioned names remain Q15.

<a id="notefiles-md--shared-radiation-conversion"></a>

### Shared radiation conversion

V2 uses the same radiation conversions and raw `cpm_count` meaning as V3 for valid readings in the sensor configuration being discussed. `[heard:H4 trainer 2026-09-25 conf:high]`

The conversion below is documented in the reviewed V3 firmware and applies to V2 on the trainer's equivalence statement. It does not transfer integration windows, timestamps, template precision, quality enums, alarm logic or transport behavior. It is not a blanket mapping for other sensor configurations.

`cpm_count` represents raw pulses accumulated over the measurement window. `cpm` is a derived count rate in counts/minute; `usv` is a derived dose rate in µSv/h, not accumulated dose. For the active `lnd7317` conversion path, let N be the raw count and T the actual elapsed milliseconds. Raw rate R is 60000 × N / T. When 1 − R × 0.000140 / 60 is positive, reported `cpm` is R divided by that denominator; reported `usv` is `cpm` / 334. These configured conversion rules are not independent calibration evidence. `[documented:F1 V3 firmware 3.2.2 reviewed 2026-09-25 conf:high]` V2 applicability is owned by the H4 statement above.

A field's name or a matching sensor label alone does not establish encoding or which measurement window it covers. Exact V2 duration representation and storage precision remain Q11/Q15. Fixed zero/missing inclusion is separately owned by Fixed-reading inclusion policy. Overflow/failure handling below is only established for V3.

<a id="notefiles-md--v3-integration-and-timestamps"></a>

### V3 integration and timestamps

`[documented:F1 V3 firmware 3.2.2 reviewed 2026-09-25 conf:high]` `[scope: v3 firmware, not yet deployed]`

The normal fixed attempt opens a counter window after 30 seconds stabilization when the detector has been powered up, then integrates for at least 300 seconds. Survey integrations use at least 60 seconds. Every completed integration closes its own window; the next is a fresh window. These are published measurements, not the V2 guide's rolling tracking calculation. Intervals can be longer due to scheduling or counter-read retry; an accumulation of a full counter cycle, or a reset, cannot be reconstructed from endpoints alone.

Conversion is owned by Shared radiation conversion. In this V3 path, a zero or negative correction denominator causes the rates to retain the uncorrected estimate as a lower bound; quality handling identifies it only where that stream carries quality. This exceptional handling has not been established for V2. V3 emits duration truncated to whole seconds, so N × 60 / emitted seconds need not reproduce its reported rate even before storage rounding.

Fixed notes are created after integration and additional device queries, without an explicit integration-start/end timestamp supplied by the host. Creation time is therefore not an exact measurement boundary. Public encoding documentation distinguishes creation metadata from location-fix metadata; compact restoration and actual event-envelope paths must be verified for the module version (Q15). Use neither arrival time nor a track-point timestamp as a substitute for an unverified integration timestamp. Nothing here establishes how long radiation stayed continuously above a threshold.

<a id="notefiles-md--v3-rich-fixed-fields"></a>

### V3 rich fixed fields

All rows apply only to the V3 rich fixed template selected as described above. `[documented:F1 V3 firmware 3.2.2 reviewed 2026-09-25 conf:high]` `[scope: v3 firmware, not yet deployed]` Public encoding widths are documented by P14; these are storage widths, not accuracy claims. An engineering-valid range is not established merely by representability.

| Field path | Type/encoding | Meaning, absence and validity | Rung |
|---|---|---|---|
| `body.sensor` | String | Active radiation source designation is `lnd7317`; it does not identify generation, calibration history or every physical variant. Empty/missing is not a radiation reading. | 1 partial |
| `body.cpm_count` | 4-byte signed integer | Raw pulse total for the integration; not corrected CPM. Nonnegative for completed measurements; zero also occurs in unreadable placeholders. Apply quality and the inclusion policy. | 1,2 partial |
| `body.csecs` | 2-byte signed integer | Completed integration duration in truncated whole seconds. Normally at least the nominal fixed window; zero is supplied for unreadable attempts. Fractional elapsed time is lost. | 1,2 partial |
| `body.cpm` | 4-byte float | Derived count rate in counts/minute; formula in Shared radiation conversion and lower-bound behavior in V3 integration and timestamps. Zero/missing is excluded by the fixed-reading inclusion policy. | 1,2 partial |
| `body.usv` | 4-byte float | Derived dose rate in µSv/h, not accumulated dose; see Shared radiation conversion. Zero/missing is excluded by the fixed-reading inclusion policy. | 1,2 partial |
| `body.quality` | 1-byte signed integer enum | 0: no condition flagged by these measurement checks; 1: full integration with no pulses; 2: correction overflow/lower bound; 3: unreadable counter/no measurement. Missing may encode 0 only for a verified instance of this rich template. No such inference for V2 or compact records. | 1,2 |
| `body.alarm` | 2-byte float | Configured threshold in µSv/h associated with alarm state, not Boolean or radiation magnitude. Presence and retained-state limits belong to V3 fixed alarm meaning. | 1,2 |
| `body.voltage` | 2-byte float | Device-reported supply voltage in volts, queried while assembling the note. Missing/zero may reflect failed acquisition or template default; not proof of a flat battery. Pair with power context and voltage mode. | 1 partial |
| `body.voltage_mode` | String | Module-reported voltage category used as context for power-dependent behavior. Complete enum/boundaries are module-version dependent and unverified (Q15). Not battery percentage. | 1 partial |
| `body.temperature` | 4-byte float | Module onboard temperature, degrees Celsius, queried during note assembly; not a verified ambient-air temperature. A missing value may be a failed query or omitted zero. `[documented:P15 API reviewed 2026-09-25 conf:high]` | 1 partial |
| `body.charging` | Boolean | Written true when the carrier reports active charging. False, absent, and a failed status query are not distinguishable from this field alone. | 1 partial |
| `body.power` | String | Reported selection among `usb`, `primary`, `lipo`. The primary classification uses a different status key than the sampling configuration path; applicability is Q16. Do not use it as sole proof of actual power source. | 1 partial |
| `body.primary_mah` | 4-byte float | Cached cumulative consumed charge in mAh from the power monitor, not remaining battery capacity. Apparent previous-completed-cycle association; resets and missing-query behavior prevent blind differencing. See A5/Q16. | 1 partial |
| `body.primary_mah_this_sample` | 4-byte float | Difference between cached successful start/end charge snapshots, in mAh, attached before this attempt's end accounting refresh. Failed queries can widen the association beyond one prior cycle; do not attribute it to this radiation record without resolving A5. | 1 partial |
| `body.primary_mah_between_samples` | 4-byte float | Difference between cached end-of-demand and subsequent start snapshots, in mAh. Failed queries can make it span more than adjacent cycles. Missing/zero can mean no baseline, no usage or failed acquisition; resets remain unresolved. | 1 partial |

<a id="notefiles-md--empty-values-and-measurement-validity"></a>

#### Empty values and measurement validity

Templated JSON can omit zero, false and empty values; this host does not request full retention. Missing is therefore not a universal fault signal. `[documented:F1,P14 reviewed 2026-09-25 conf:high]` Fixed radiation inclusion remains governed by Fixed-reading inclusion policy, even where firmware recorded a real zero-pulse integration.

Quality 1 does not diagnose the cause of zero pulses: low radiation, detector failure and power failure can look alike. Quality 3 ends an attempt after a 60-second run of counter-read failures; its rates, count and duration are zero placeholders, not observations. Quality 0 means only that these software checks raised no flag; it does not prove calibration or sensor health. Quality 2 is a lower bound and must not enter exact-valued averages or doses as though it were a calibrated measurement. Unknown quality values require investigation. `[documented:F1 V3 firmware 3.2.2 reviewed 2026-09-25 conf:high]` `[scope: v3 firmware, not yet deployed]`

<a id="notefiles-md--v3-compact-fixed-fields"></a>

### V3 compact fixed fields

`[documented:F1 V3 firmware 3.2.2 reviewed 2026-09-25 conf:high]` `[scope: v3 firmware, not yet deployed]`

| Field/template key | Type/encoding | Meaning and limit | Rung |
|---|---|---|---|
| `body.csecs` | 2-byte signed integer | Same duration meaning as the rich field; see V3 rich fixed fields. | 1,2 partial |
| `body.usv` | 4-byte float | Same dose-rate meaning as the rich field; quality does not accompany it. | 1,2 partial |
| `body.cpm` | 4-byte float | Same count-rate meaning as the rich field; quality does not accompany it. | 1,2 partial |
| `body.cpm_count` | 3-byte signed integer | Same raw-count meaning as the rich field, with a narrower storage range. | 1,2 partial |
| `_time` | 4-byte signed integer template metadata | Requests note-creation time restoration. It is not a host-supplied measurement timestamp; actual event path/clock validity is Q15. | 1 partial |
| `_loc5` | 5-byte signed integer template metadata | Requests compact location coding. Exact resolved path, precision and fix freshness remain Q15; do not claim fresh/sub-metre location. | 1 partial |

The compact template contains no `sensor`, `quality`, `alarm` or housekeeping fields. Their absence is expected and does not mean a healthy detector, cleared alarm or failed power acquisition. Zero/missing radiation still follows the trainer's inclusion policy; positive saturation cannot be recognized from a missing quality field alone. Lack of `_ltime` also means this template does not explicitly restore location-fix age.

<a id="notefiles-md--v3-tracking-fields"></a>

### V3 tracking fields

`[documented:F1 V3 firmware 3.2.2 reviewed 2026-09-25 conf:high]` `[scope: v3 firmware, not yet deployed]`

The host supplies the following body to the tracking API only after a completed survey integration that was not unreadable. The module produces the final `_track.qo` event; its implementation/template and actual event shape are Q15. These are supplied field meanings, not a verified V2 decoder or proof each track point carries all five.

| Supplied body path | JSON type | Meaning and limit | Rung |
|---|---|---|---|
| `body.sensor` | String | Same active sensor designation as the rich fixed field. Module retention is unverified. | 1 partial |
| `body.usv` | Number | Dose rate from the completed survey window; formula in Shared radiation conversion. No accompanying quality code. | 1,2 partial |
| `body.cpm` | Number | Count rate from that window; correction overflow can be a lower bound. | 1,2 partial |
| `body.cpm_count` | Number, integral | Raw pulses during that window, not a journey total. | 1,2 partial |
| `body.cpm_secs` | Number, integral | Whole elapsed seconds for that window; analogous to fixed `csecs`, with survey duration and emission semantics. | 1,2 partial |

Unreadable integrations supply no new body; zero-pulse and overflow integrations can be supplied. Source comments describe one-use tracking data and unfilled points becoming zero, but module behavior has not been verified. Do not call a zero/missing track value a fresh zero measurement, copy fixed quality semantics onto it, or treat every five-second point as an independent integration. Journey counter/start identity meanings from D1 remain separately scoped in the `_track.qo` entry. Alarm decisions below are fixed-only.

<a id="notefiles-md--v3-fixed-alarm-meaning"></a>

### V3 fixed alarm meaning

`[documented:F1 V3 firmware 3.2.2 reviewed 2026-09-25 conf:high]` `[scope: v3 firmware, not yet deployed]`

For a completed readable fixed measurement and a nonzero `alarm_usv`, a dose rate at or above the configured threshold enters alarm state. Correction overflow also enters alarm even if the reported lower bound is below the threshold. A subsequent readable, non-overflow value below threshold clears it. Zero threshold disables decisions; an unreadable attempt bypasses recomputation and can preserve the previous state.

On the rich stream, nonzero `body.alarm` carries the threshold associated with retained alarm state. It can coexist with quality 3 and zero/absent radiation placeholders: that is not a newly measured exceedance. The compact stream omits the threshold field. Therefore neither absence of `alarm` across all streams nor direct comparison of every record's `usv` to `alarm` reconstructs the alarm history.

Entering alarm can request expedited synchronization, limited to once per hour after a successfully accepted request. The pending request is consumed only when both local fixed submissions succeed; a failed attempt can leave the request pending for a later measurement. This is neither a delivery SLA nor proof that a person was notified. Persistent alarm does not itself imply a fresh hourly notification. A threshold change, mode change or restart must be included when reconstructing state; no such history is established for deployed V2.

<a id="notefiles-md--coverage-check"></a>

### Coverage check

Six product evidence sets walked: initial schemas, bounded events, public technical example, V2 guide, restricted V3 firmware and refreshed schemas; prior skills were empty. The union contains 20 names and 20 per-name entries. The refreshed schemas contain 83 body paths (including one object parent); each has an individual row. All 15 rich fixed fields, six compact template fields and five tracking-body fields from F1 have entries. The two variable settings have API-value entries; final module/database body paths, system-file meanings, actual V2 encodings and full event envelopes remain incomplete. Shared pulse-count/conversion meanings have trainer support; this does not complete other V2 field semantics. This is field-name coverage, not universal semantic answerability.

---

<a id="recipes-md"></a>

## Recipes

*`recipes.md` | kinds: recipe | 2026-09-25 14:01 | 7143 bytes*

```yaml
kind: recipe
description: Conditional general workflow for rolling-window and geographic analysis
updated: 2026-09-25T18:01:36Z
sources:
  - H5: trainer, query family confirmed by enumerated read-back 2026-09-25
```

<a id="recipes-md--status"></a>

### Status

One general workflow is drafted: Rolling-window and geographic analysis, conditional on resolving the actual query's scope/statistic, usable session evidence when deployment membership is needed, and usable field/location evidence. It has not been exercised against a concrete operational query. No verified example answer or predictive model is claimed.

The trainer requested flexible support rather than a fixed question menu. The following is based on that described family, not a fabricated question attributed to an end user. It includes period and latest-state branches; neither makes every possible question answerable.

<a id="recipes-md--rolling-window-and-geographic-analysis"></a>

### Rolling-window and geographic analysis

**Status: conditional.** Scope and required inputs come from the actual ask. Population and timezone defaults are established in [operations.md](#operations-md); ambiguous geography, missing session evidence or an unsupported field must be resolved before the affected result is computed. This is a reusable semantic workflow, not a fixed report or an access runbook.

**Question family and recipient.** The trainer described a “rolling-window of recent data” from a device or device set, often with a geographic region. No exact operational question or fixed recipient was supplied. Preserve the actual asker's wording when using this workflow. Product context and intended answer limits are in [product.md](#product-md), [Question patterns](#product-md--question-patterns), [Audiences and presentation](#product-md--audiences-and-presentation), and [Public interpretation](#product-md--public-interpretation).

**Resolve the question.** Establish the requested measurement/statistic, device or device set, region, time interval and whether the question is about measurements or arrivals. Apply [operations.md](#operations-md), [Time windows](#operations-md--time-windows), [Population](#operations-md--population) and [Geographic selection](#operations-md--geographic-selection). Resolve names against available identities/mappings; preserve leading zeroes and exact identity tokens, and ask when matches are absent or ambiguous. Read back the resolved selection when normalization could change it. Establish available history for the chosen window; a requested interval is not proof of retention or complete data. Determine the recipient and intended decision when they change presentation or permissible interpretation.

**Meaning in the data.** Use the relevant fixed stream's body fields described in [notefiles.md](#notefiles-md), `_air.qo`, `data.qo` or `data_ntn.qo`, chosen by revision and effective configuration. Shared radiation conversion and Fixed-reading inclusion policy own units, conversion and validity. Use Event clocks and location for the temporal/spatial selection. Tracking is a separate branch through `_track.qo`, Measurement mechanism and V3 tracking fields; do not silently pool its measurement windows with fixed data or assume every track point has a fresh radiation measurement.

Within each note, select the scalar measurement the question asks about; a rate, pulse total, diagnostic or quality state is not interchangeable. Across time, establish whether the asker wants the latest value, a peak, any threshold crossing, a mean, a distribution or another reduction. Do not infer one from another. Across devices, state whether a result weights observations, devices or time; a higher reporting cadence must not silently give one device more weight in a claimed typical-device value. An unspecified anomaly needs a baseline/threshold supplied or justified for the actual question; firmware alarm configuration alone does not establish a health threshold.

Keep the counting unit and denominator explicit. Distinguish eligible devices, devices with valid observations, valid observations and excluded/missing observations. Avoid duplicated retrievals using event identity; transport representations and overlapping windows need their own treatment from the inventory. Do not join replacements or counter resets into one history without an effective mapping. A complete-looking result cannot be based on a silently truncated retrieval; report coverage and unfinished retrieval explicitly.

**Period branch.** Apply the owning validity rule to all observations within the requested period before the chosen aggregate. A later invalid reading does not erase an earlier valid observation or a supported threshold crossing in that period. Include eligible devices with no valid observations in the missing-coverage accounting, not as zero radiation.

**Latest-state branch.** Select the latest relevant record per device first, then apply validity. If invalid or absent, that device has no valid latest measurement; do not replace it silently with an older good result. A last-known-valid value may be reported with its original timestamp and age. Recent session eligibility alone does not establish a fresh radiation value or device health.

**Inference.** Report what the sampled measurements support. A gap is not a negative result. An average integration does not establish uninterrupted time above a threshold; a dose total needs a supported integration/coverage model; a trend does not establish a forecast. Where the actual question requires an unsupported duration, dose or forecast, that part is blocked pending the specific missing mechanism. Keep the supported descriptive part of the answer.

**Answer shape.** Use a concise table/series for comparisons over time and devices, and a map when spatial pattern materially helps. Label the region, effective window, timezone, units and weighting. State which devices/intervals could not be assessed and why; precision should follow the source, not display formatting. Example answer structure, with placeholders only: “For <region/device set>, during <start>–<end> <timezone>, <statistic> of <measurement> was <value> <unit>, using <valid observations> from <contributing devices> of <eligible devices>. <coverage/exclusions> could not be assessed.” For latest-state requests include <capture time> and <age>. This is an authored structure for future adaptation, not a customer quotation or a computed result.

Never describe missing data as safe, healthy, zero, no excursion or complete coverage. Do not claim a local alarm implies notification or acknowledgement. An explicit historical request follows the historical-population distinction in [operations.md](#operations-md); old captured data are not globally discarded.

**Dependencies.** Load [operations.md](#operations-md), [Population](#operations-md--population), [Time windows](#operations-md--time-windows) and [Geographic selection](#operations-md--geographic-selection); the selected Notefile and field entries; Shared radiation conversion, Fixed-reading inclusion policy, Event clocks and location, Session evidence and the appropriate measurement-window section in [notefiles.md](#notefiles-md); [product.md](#product-md), [Question patterns](#product-md--question-patterns) and [Public interpretation](#product-md--public-interpretation); and [questions.md](#questions-md), [Q4](#questions-md--q4-measurement-and-anomalies), [Q6](#questions-md--q6-deployment-lookback-resolved), [Q8](#questions-md--q8-technical-examples-and-variant-applicability), [Q9](#questions-md--q9-screen-semantics-versus-measurement), [Q11](#questions-md--q11-guide-applicability-installed-configuration-and-clocks), [Q14](#questions-md--q14-remaining-radiation-validity-and-representation) and [Q15](#questions-md--q15-module-templates-and-clocks) where relevant. A missing or conflicting dependency blocks only the inference that needs it; identify that gap and continue the supported analysis. New questions may extend this workflow after establishing any new field or inference meanings, rather than being rejected for lacking a predefined recipe.

---

<a id="questions-md"></a>

## Open questions

*`questions.md` | kinds: questions, anomalies | 2026-09-25 14:01 | 13062 bytes*

```yaml
kind: questions,anomalies
description: Open questions and promised materials
updated: 2026-09-25T18:01:36Z
sources:
  - H6: trainer, default session lookback of 90 days confirmed by enumerated read-back 2026-09-25
  - H5: trainer, query patterns, deployment scope and timezone confirmed by enumerated read-back 2026-09-25
  - H4: trainer, V2/V3 radiation-conversion equivalence reported 2026-09-25; read-back pending
  - H3: trainer, fixed-reading inclusion confirmed by enumerated read-back 2026-09-25
  - F1: restricted V3 host firmware, version 3.2.2, reviewed 2026-09-25
  - H1: trainer, interview 2026-09-25, remaining offers tracked below
  - P2: public deployment follow-up, public, published 2025-07, reviewed 2026-09-25
  - P11: public product overview, public, published 2026-01, reviewed 2026-09-25
  - P12: public customer story, public, listed 2026-03, reviewed 2026-09-25
  - D1: public owner guide for the V2 product, public, revised 2022-07-06, reviewed 2026-09-25
  - P5: public technical article on device faults, public, published 2025-07, reviewed 2026-09-25
```

All entries opened 2026-09-25. The interview is collecting offered materials sequentially.

<a id="questions-md--q1-offered-materials"></a>

### Q1 — Offered materials

The public field case study and V2 owner guide were read, and the restricted V3 firmware was supplied and deeply reviewed for data production and field meaning. The trainer authorized learning from it while keeping source private. No offered artifact is awaiting receipt. Derived decoder meanings are drafted locally with V3 scope; no source, paths, excerpts or internal symbols are in the working copy. The general query family, population/timezone defaults and fixed-reading inclusion have been confirmed by read-back. Conversion equivalence remains trainer-reported, with its own read-back pending. No further material is required merely to publish the explicitly conditional training. Remaining field-validity details stay in Q14.

<a id="questions-md--q2-radnote-in-use"></a>

### Q2 — Radnote in use

Public sources now describe installation, power and weather context (product.md, Field deployment). Ask the trainer only what remains: actual maintenance frequency, everyday handling, misuse and exceptions in this deployment. The guide documents fixed/mobile/hybrid modes, shading and condensation; do not re-ask those general meanings. This affects [product.md](#product-md), [Usage](#product-md--usage) and future interpretation of silence. Meanwhile, do not infer usage from field or fleet names.

<a id="questions-md--q3-query-specific-needs"></a>

### Q3 — Query-specific needs

The trainer wants flexible support for arbitrary data questions and expects rolling-window/device-set/geographic requests. Owning purpose: [product.md](#product-md), [Question patterns](#product-md--question-patterns). A conditional workflow is drafted in [recipes.md](#recipes-md). Do not require a fixed example question from the trainer before supporting a real ask. When an actual question arrives, establish only its unresolved statistic, scope, recipient/decision and output needs; use the defaults already recorded. No workflow has yet been validated on a concrete operational result.

<a id="questions-md--q4-measurement-and-anomalies"></a>

### Q4 — Measurement and anomalies

The guide establishes separate scheduling and averaging mechanisms within its revision; see [notefiles.md](#notefiles-md), [Measurement mechanism](#notefiles-md--measurement-mechanism). V3 decoder meanings are now recorded separately; the trainer has supplied conversion equivalence (notefiles.md, Shared radiation conversion). Remaining work is timing/encoding applicability and the operational definition of an anomaly. This affects [notefiles.md](#notefiles-md) and [product.md](#product-md), [Vocabulary](#product-md--vocabulary). Meanwhile, “ongoing monitoring” is a purpose statement, not proof of uninterrupted measurement or continuous threshold exceedance.

<a id="questions-md--q5-timezone-resolved-window-wording-remains-query-specific"></a>

### Q5 — Timezone resolved; window wording remains query-specific

The trainer selected UTC by default with an explicit asker timezone taking precedence; owning rule: [operations.md](#operations-md), [Time windows](#operations-md--time-windows). This was confirmed by read-back. Do not re-ask the default timezone. A duration or boundary still needs resolving when the actual ask uses vague wording; UTC alone does not define overnight or recent.

<a id="questions-md--q6-deployment-lookback-resolved"></a>

### Q6 — Deployment lookback resolved

The trainer set the default recent-session lookback to 90 days; owning rule: [operations.md](#operations-md), [Population](#operations-md--population). This and the recent-session membership rule were confirmed by read-back. Do not re-ask the cutoff or silently tie it to the radiation window. For an actual query, establish only any explicitly different population and whether historical or present-day deployment is intended.

<a id="questions-md--q7-hardware-changes-and-experience"></a>

### Q7 — Hardware changes and experience

Ask the trainer what gets changed in the field, how those changes affect measurements or history, and what was learned from incidents. This affects lifecycle interpretation and constraints. Meanwhile, continuity across a replacement, recalibration or reset is unestablished.

<a id="questions-md--q8-technical-examples-and-variant-applicability"></a>

### Q8 — Technical examples and variant applicability

Ask the trainer which sensor types and variants occur here; settle calibration and mounting applicability. The trainer has supplied a fixed-reading inclusion rule for zero/missing radiation; see [notefiles.md](#notefiles-md), [Fixed-reading inclusion policy](#notefiles-md--fixed-reading-inclusion-policy). The guide identifies a detector model, but does not enumerate all sensor designations in the project or give conversion constants. The shared conversion path now has trainer support (notefiles.md, Shared radiation conversion); this does not establish independent calibration or all sensor-variant mappings. A public example remains insufficient for a universal fault diagnosis. This affects [notefiles.md](#notefiles-md), [_air.qo](#notefiles-md--air-qo) and [product.md](#product-md), [Field deployment](#product-md--field-deployment). Meanwhile, use the public field descriptions only within their stated scope and do not infer a failed tube or zero radiation from one absent/zero field.

<a id="questions-md--q9-screen-semantics-versus-measurement"></a>

### Q9 — Screen semantics versus measurement

Ask the trainer or dashboard owner how a device maps to a public station; the remaining statistic/clock choices, stale/missing/fault display behavior, and radiation notification workflow. The guide documents fixed delivery using `when`, plus mobile location/identity prerequisites; these do not prove the project uses that receiving service. This affects [product.md](#product-md), [Receiving surfaces](#product-md--receiving-surfaces) and future recipes. Meanwhile, keep API-receipt time, measurement time and display refresh distinct. Air-quality subscription documentation does not establish a radiation alert mechanism.

<a id="questions-md--q10-connectivity-scope"></a>

### Q10 — Connectivity scope

Public material describes satellite capability with differing deployment specificity. Ask the trainer or V2 specification which configurations actually support it and how fallback is recognized. This affects any explanation of resilience or silence. The 2022 guide describes cellular and replaceable WiFi alternatives but does not document satellite fallback. Its age means silence on satellite does not settle later configurations. Meanwhile, no claim that every device has working satellite fallback is established. See A1.

<a id="questions-md--q11-guide-applicability-installed-configuration-and-clocks"></a>

### Q11 — Guide applicability, installed configuration and clocks

Ask the trainer or check deployed source/version evidence to establish which 2022-guide mechanisms remain applicable, which power/transport variants exist, effective settings rather than requested settings, exact wire encodings, template behavior and the timestamp location within an integration window. This affects all D1-based descriptions. Meanwhile, keep D1 rules scoped to its revision; do not apply examples as fleet defaults or infer clock validity from a field being present.

<a id="questions-md--q12-tracking-idle-and-heartbeat-behavior"></a>

### Q12 — Tracking idle and heartbeat behavior

The V3 source defines its own journey behavior but cannot resolve historical V2 passages. Ask the trainer to resolve A3: what remains enabled after motion stops, whether uploads continue periodically, when any final upload occurs, and which Notefile/fields carry the tracking heartbeat. This controls silence interpretation. Meanwhile, do not choose an idle-state rule from one passage and treat the other as obsolete.

<a id="questions-md--q13-rate-units-in-the-fixed-receiving-service"></a>

### Q13 — Rate units in the fixed receiving service

Ask the trainer or examine the relevant transformation to settle A2: does the fixed receiving service merely label a dose rate without its per-hour denominator, or is a conversion performed? This affects `_air.qo body.usv` presentation and any dose calculation. Meanwhile, preserve the device-rate description and the downstream-label discrepancy; infer neither accumulated dose nor conversion.

<a id="questions-md--q14-remaining-radiation-validity-and-representation"></a>

### Q14 — Remaining radiation validity and representation

The trainer confirmed fixed zero/missing exclusion by read-back and separately reported V2/V3 conversion equivalence. Their owning rules are [notefiles.md](#notefiles-md), [Fixed-reading inclusion policy](#notefiles-md--fixed-reading-inclusion-policy) and [Shared radiation conversion](#notefiles-md--shared-radiation-conversion); only conversion equivalence remains heard pending its own enumerated read-back. Do not re-ask whether conversions and pulse-count meaning match. Remaining questions: which measurement fields are required in a partly populated tuple, survey zero/missing treatment, V2 duration representation/precision, overflow handling and relevant sensor configurations. The equivalence statement does not establish quality enums, templates, integration windows or alarm behavior in V2. Use the shared conversion for valid readings while preserving those separate limits.

<a id="questions-md--q15-module-templates-and-clocks"></a>

### Q15 — Module, templates and clocks

The reviewed host delegates tracking, transport-specific deletion, timestamp/position restoration, resolved environments and some sync scheduling to module firmware. Establish the applicable module version and inspect representative decoded events/templates before treating those boundaries as verified. In particular, verify tracking-body retention, missing/zero output, compact location and creation-time paths, clock validity and applied configuration. No device commands are authorized by this question; read-only evidence or a trainer explanation is sufficient for the next step.

<a id="questions-md--q16-power-accounting-and-classification"></a>

### Q16 — Power accounting and classification

Resolve A5 before assigning primary-charge use to a particular radiation sample or inferring battery lifetime. Also reconcile the status-key difference used for primary power classification in the rich record versus the cadence path. These uncertainties are V3-specific and do not establish a fault in V2. Until resolved, power fields are diagnostic context, not a reliable remaining-life model.

<a id="questions-md--anomalies"></a>

## Anomalies

<a id="questions-md--a1-connectivity-descriptions-need-version-and-deployment-scope"></a>

### A1 — Connectivity descriptions need version and deployment scope

A public deployment follow-up describes satellite capability, while a later public product overview describes backup satellite connectivity as a future addition; a customer story describes automatic failover. These accounts may refer to different configurations rather than disagreeing about one device. No per-device applicability has been established. `[documented:P2,P11,P12 source dates 2025-07,2026-01,2026-03; reviewed 2026-09-25 conf:medium]` Open: Q10. Do not resolve the scope from marketing language.

<a id="questions-md--a2-fixed-survey-unit-label-omits-a-rate-denominator"></a>

### A2 — Fixed-survey unit label omits a rate denominator

The device-display description in D1 and the public technical article describe µSv/h. D1's fixed-survey integration describes and displays the received value as µSv while routing `body.usv`. A missing label component is not proof of a numerical conversion. `[documented:D1 V2 guide revision 2022-07-06; P5 public technical article 2025-07; reviewed 2026-09-25 conf:high]` Open: Q13. Owning field: [notefiles.md](#notefiles-md), [_air.qo](#notefiles-md--air-qo).

<a id="questions-md--a3-tracking-power-and-idle-upload-descriptions-differ"></a>

### A3 — Tracking power and idle-upload descriptions differ

The guide describes tracking detection circuitry as continuously enabled, but also describes powering down after movement stops. A general passage retains periodic uploads whether moving or stationary; an illustrative mobile configuration describes a final upload after five minutes stillness followed by idle operation. These passages do not settle the distinction between an enabled circuit, a powered detector and the scheduler's idle behavior. `[documented:D1 V2 guide revision 2022-07-06 conf:high]` Open: Q12. Owning scenario: [operations.md](#operations-md), [Location and motion](#operations-md--location-and-motion).

<a id="questions-md--a4-tracking-measurement-windows-differ-by-generation"></a>

### A4 — Tracking measurement windows differ by generation

The confirmed V2 guide describes a one-minute rolling tracking average. The reviewed V3 implementation supplies completed fresh 60-second integrations to a separately scheduled tracker. This is a scoped difference, not a reason to overwrite the confirmed guide statement. Owning descriptions: [notefiles.md](#notefiles-md), [Measurement mechanism](#notefiles-md--measurement-mechanism) and [V3 tracking fields](#notefiles-md--v3-tracking-fields). Open applicability: Q14/Q15.

<a id="questions-md--a5-v3-charge-accounting-may-refer-to-a-prior-cycle"></a>

### A5 — V3 charge accounting may refer to a prior cycle

The rich record includes cached cumulative and per-sample charge before that attempt's end accounting is refreshed. Consequently `primary_mah` and `primary_mah_this_sample` can refer to the previous completed cycle; failed power queries can leave older snapshots cached, making later differences span multiple cycles; resets add uncertainty. The between-sample difference is captured on starting the next cycle. `[documented:F1 V3 firmware 3.2.2 reviewed 2026-09-25 conf:high]` `[scope: v3 firmware, not yet deployed]` Owning fields: [notefiles.md](#notefiles-md), [V3 rich fixed fields](#notefiles-md--v3-rich-fixed-fields). Open: Q16. No automatic source fix or reinterpretation of V2 is implied.
