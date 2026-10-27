---
kind: recipe
description: Conditional general workflow for rolling-window and geographic analysis
updated: 2026-09-25T18:01:36Z
sources:
  - H5: trainer, query family confirmed by enumerated read-back 2026-09-25
---

# Recipes

## Status

One general workflow is drafted: Rolling-window and geographic analysis, conditional on resolving the actual query's scope/statistic, usable session evidence when deployment membership is needed, and usable field/location evidence. It has not been exercised against a concrete operational query. No verified example answer or predictive model is claimed.

The trainer requested flexible support rather than a fixed question menu. The following is based on that described family, not a fabricated question attributed to an end user. It includes period and latest-state branches; neither makes every possible question answerable.

## Rolling-window and geographic analysis

**Status: conditional.** Scope and required inputs come from the actual ask. Population and timezone defaults are established in operations.md; ambiguous geography, missing session evidence or an unsupported field must be resolved before the affected result is computed. This is a reusable semantic workflow, not a fixed report or an access runbook.

**Question family and recipient.** The trainer described a “rolling-window of recent data” from a device or device set, often with a geographic region. No exact operational question or fixed recipient was supplied. Preserve the actual asker's wording when using this workflow. Product context and intended answer limits are in product.md, Question patterns, Audiences and presentation, and Public interpretation.

**Resolve the question.** Establish the requested measurement/statistic, device or device set, region, time interval and whether the question is about measurements or arrivals. Apply operations.md, Time windows, Population and Geographic selection. Resolve names against available identities/mappings; preserve leading zeroes and exact identity tokens, and ask when matches are absent or ambiguous. Read back the resolved selection when normalization could change it. Establish available history for the chosen window; a requested interval is not proof of retention or complete data. Determine the recipient and intended decision when they change presentation or permissible interpretation.

**Meaning in the data.** Use the relevant fixed stream's body fields described in notefiles.md, `_air.qo`, `data.qo` or `data_ntn.qo`, chosen by revision and effective configuration. Shared radiation conversion and Fixed-reading inclusion policy own units, conversion and validity. Use Event clocks and location for the temporal/spatial selection. Tracking is a separate branch through `_track.qo`, Measurement mechanism and V3 tracking fields; do not silently pool its measurement windows with fixed data or assume every track point has a fresh radiation measurement.

Within each note, select the scalar measurement the question asks about; a rate, pulse total, diagnostic or quality state is not interchangeable. Across time, establish whether the asker wants the latest value, a peak, any threshold crossing, a mean, a distribution or another reduction. Do not infer one from another. Across devices, state whether a result weights observations, devices or time; a higher reporting cadence must not silently give one device more weight in a claimed typical-device value. An unspecified anomaly needs a baseline/threshold supplied or justified for the actual question; firmware alarm configuration alone does not establish a health threshold.

Keep the counting unit and denominator explicit. Distinguish eligible devices, devices with valid observations, valid observations and excluded/missing observations. Avoid duplicated retrievals using event identity; transport representations and overlapping windows need their own treatment from the inventory. Do not join replacements or counter resets into one history without an effective mapping. A complete-looking result cannot be based on a silently truncated retrieval; report coverage and unfinished retrieval explicitly.

**Period branch.** Apply the owning validity rule to all observations within the requested period before the chosen aggregate. A later invalid reading does not erase an earlier valid observation or a supported threshold crossing in that period. Include eligible devices with no valid observations in the missing-coverage accounting, not as zero radiation.

**Latest-state branch.** Select the latest relevant record per device first, then apply validity. If invalid or absent, that device has no valid latest measurement; do not replace it silently with an older good result. A last-known-valid value may be reported with its original timestamp and age. Recent session eligibility alone does not establish a fresh radiation value or device health.

**Inference.** Report what the sampled measurements support. A gap is not a negative result. An average integration does not establish uninterrupted time above a threshold; a dose total needs a supported integration/coverage model; a trend does not establish a forecast. Where the actual question requires an unsupported duration, dose or forecast, that part is blocked pending the specific missing mechanism. Keep the supported descriptive part of the answer.

**Answer shape.** Use a concise table/series for comparisons over time and devices, and a map when spatial pattern materially helps. Label the region, effective window, timezone, units and weighting. State which devices/intervals could not be assessed and why; precision should follow the source, not display formatting. Example answer structure, with placeholders only: “For <region/device set>, during <start>–<end> <timezone>, <statistic> of <measurement> was <value> <unit>, using <valid observations> from <contributing devices> of <eligible devices>. <coverage/exclusions> could not be assessed.” For latest-state requests include <capture time> and <age>. This is an authored structure for future adaptation, not a customer quotation or a computed result.

Never describe missing data as safe, healthy, zero, no excursion or complete coverage. Do not claim a local alarm implies notification or acknowledgement. An explicit historical request follows the historical-population distinction in operations.md; old captured data are not globally discarded.

**Dependencies.** Load operations.md, Population, Time windows and Geographic selection; the selected Notefile and field entries; Shared radiation conversion, Fixed-reading inclusion policy, Event clocks and location, Session evidence and the appropriate measurement-window section in notefiles.md; product.md, Question patterns and Public interpretation; and questions.md, Q4, Q6, Q8, Q9, Q11, Q14 and Q15 where relevant. A missing or conflicting dependency blocks only the inference that needs it; identify that gap and continue the supported analysis. New questions may extend this workflow after establishing any new field or inference meanings, rather than being rejected for lacking a predefined recipe.
