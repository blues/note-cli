# Teaching a Notehub project

You are an AI agent running in a developer's harness. The person you are working with is
a developer or administrator of a Notehub project, and they have asked you to teach that
project about their product.

Read this whole document before doing anything. Then start at Step 1.

## What you are doing, and why

Someone is going to ask an agent a question about their product. Not about their data -
about their **product**. *Did anything thaw in transit? Which units need a service visit?
Is it safe where I live? Are we going to miss the SLA this month?*

They will not know, and should not have to know, that the answer lives in a Notefile
called `env.qo`, in a field called `t`, which is an array of three readings taken twenty
seconds apart in tenths of a degree, where a fourth element appears only on units with a
door sensor fitted, and where a value of exactly `0` means the probe is disconnected
rather than freezing.

**Closing that gap is the whole job**, and it is the one thing an agent cannot do by
looking harder at the data. More data does not help. Better statistics do not help.
Nothing inside `t` will ever reveal that `t` is a temperature, that the product is a
refrigerated shipping container, that this customer's contractual limit is eight degrees
for fifteen consecutive minutes, or that the person asking is a logistics manager who
needs a yes or no and a container number, not a distribution.

So you are not writing a data dictionary. You are writing down what an agent needs in
order to take a question posed in the product's own language, turn it into the right
query against the right interface over the right devices, and then answer in the language
the asker actually uses - hedged where the data is weak, and refusing where the data
cannot support the claim.

A good test of every sentence you write: **would this help an agent answer a question
asked by someone who has never seen the data?** If it only helps a reader who already
understands the schema, it is documentation, and the API already has documentation.

You are describing the **shape** of the project, never its data. A skill says "each
`env.qo` note carries `t`, three interior-temperature samples in tenths of a degree
Celsius taken twenty seconds apart, and a sustained reading above 8 is an excursion." It
never says "on Tuesday container 4021 reported 9.1." Shape is durable and small. Data is
neither.

### The shapes that defeat an agent

These are what make a project unreadable from the data alone. Hunt each of them
deliberately, because every one of them silently produces a confident wrong answer:

- **Field names that carry no meaning.** `t`, `lvl`, `st`, `v2`, `pm`. Abbreviations that
  made sense to whoever wrote the firmware and to nobody since.
- **Arrays where position is meaning.** A field holding `[12, 14, 11]` may be three
  samples in time, or three different sensors, or a value with its minimum and maximum.
  No amount of staring at the numbers will tell you which, and getting it wrong makes
  every answer wrong.
- **Integers that are really enums.** `status: 3` means something. Nothing in the data
  says what, and an agent that averages it has produced nonsense.
- **Units that are implied.** Tenths, thousandths, Fahrenheit, kilopascals, counts per
  minute. A number with the wrong unit is worse than no number at all.
- **Compact and templated encodings**, where a field is present but zero because the
  sender omitted it - so absence and a real zero are indistinguishable on the wire.
- **Names that lie.** A Notefile or field whose name describes what it carried two product
  generations ago. The most dangerous kind, because it invites a confident answer from an
  agent that never suspected anything was wrong.
- **The same quantity under two names**, in two Notefiles, because two people implemented
  it. A query that joins on one name silently drops half the fleet.
- **Fields whose absence is the signal.** The reading missing because the sensor failed is
  often the most important thing the product will ever report.

For each one you find, the skill must say what it means - in the words the person asking
would use, not the words the firmware used.

## Step 1 - Confirm you can reach Notehub

```
notehub -token
```

If that prints a token, continue. If it fails, the person must sign in themselves - it
opens a browser and you cannot do it for them. Ask them to run `notehub -signin` in their
own terminal and tell you when it is done. If the `notehub` command is not found, they
have not installed the CLI: point them at
https://dev.blues.io/tools-and-sdks/notehub-cli and wait.

## Step 2 - Choose the project

```
notehub -projects -pretty
```

Ask which project you are teaching. Teaching requires developer or administrator access;
if a later step is refused, say so plainly rather than working around it. Every command
from here on takes `-project <projectUID>` or `-product <productUID>`.

## Step 3 - Pull, and find out which kind of run this is

```
notehub skills pull -project <projectUID>
notehub skills status -project <projectUID>
```

Teaching happens in a **working copy**, a folder on this machine that belongs to this
project. `pull` fills it from the project and `status` says where it is. Read every file
in it before proposing anything.

**If the project holds nothing, this is an initial run.** Say so, and offer to start with
the lesson "Learn from the live project", which needs nothing from the person.

**If the project already holds skills, this is an update run.** Do not start over. Open
by telling the person where things stand:

- what is known, by kind
- when each file was last updated, and from which sources
- what has gone stale: firmware that has moved on since the commit recorded in a file,
  Notefiles now in the data that no skill mentions, sources older than a few months
- what was never learned at all

Then ask what they want to work on. An update run is mostly Step 6.

## Step 4 - The kinds of knowledge

Every skill declares its kind in its front matter, and the kind is stored as the upload's
tags, so that a later agent can load only the knowledge a question needs instead of
reading everything. A file may carry several kinds; keep the file set small and let the
kinds be rich.

**What the product is** - from people and documents, and slow to change:

| Kind | What it holds |
|---|---|
| `product` | The physical thing, its variants, what is inside it, who operates it |
| `mission` | What the organization is trying to achieve; the measure of success that is not a sensor reading |
| `audiences` | Who asks questions, in what words and units, and what they may be told |
| `constraints` | What an answer must never claim, and who must be escalated to |

**What the data means** - from firmware and observation:

| Kind | What it holds |
|---|---|
| `vocabulary` | The customer's words mapped to fields, in both directions, tagged by who uses them |
| `mapping` | Which Notefile and field carries a concept, with direction and cadence |
| `derivation` | Units, formulas, and what is computed rather than measured |
| `sentinels` | What absence, zero, and placeholder values mean - absent is not zero |
| `population` | Which devices are real, which are test rigs, gateways, or long dead |
| `scenarios` | What situation produces what data - which conditions make a note appear, at what cadence, carrying which fields, and what its absence means |
| `config` | Project and fleet configuration - environment variables, smart fleet rules, routes - what each controls, and which of them are dead letters |

**How to answer a question** - about this project specifically:

| Kind | What it holds |
|---|---|
| `recipe` | A question shape, and the exact call sequence that answers it |
| `cost` | This project's cardinality, so a plan can be sized before it is run |
| `verification` | How to prove an answer is right before reporting it |
| `drift` | Where the declared shape and the observed shape disagree |
| `gaps` | What is still unknown, and who has to answer it |

And one kind that is about the skills rather than about the project:

| Kind | What it holds |
|---|---|
| `index` | Start here: what this project is in a paragraph, and what the other skills cover |

Start with `index`, `product`, `vocabulary`, `mapping`, `scenarios`, `derivation`,
`sentinels`, `population`, and `recipe`. Add the others when you have something real to put in them.
Do not create a file you cannot fill.

`gaps` deserves particular discipline, because it is what a later run reads to know what
to ask. Every claim you had to mark `assumed`, and every question you could not answer
because the person was not there, belongs in it - with the name of whoever could settle
it. A teaching run that ends with an empty `gaps` file has almost certainly been
guessing.

Do not use `publish` as a kind - Notehub reserves it.

## Step 4a - Where a finding goes

You will discover things that fit several kinds at once. Route them by asking one
question: **which question would be answered wrongly if nobody knew this?** Put the
finding in the kind that question would load. Not where you found it - where it will be
read.

Then four rules that settle the rest:

1. If the finding is a **disagreement between two sources**, record the current truth in
   the kind that answers questions, and record the disagreement itself in `drift`. Those
   serve different readers: the first serves an agent answering a question, the second
   serves the developer who should go fix something. Write both; neither substitutes for
   the other.
2. If it changes **whether a device's data can be trusted**, it belongs in `population`,
   however you came across it.
3. If it changes **what a value means**, it belongs in `sentinels` or `derivation`. Never
   leave that kind of fact in prose somewhere, because the agent that needs it will be
   reading a number, not a narrative.
4. If it is a fact about **this project's configuration** rather than about its data, it
   belongs in `config` - including a rule or variable that is configured but inert.

Three real findings, routed:

| Finding | Goes to |
|---|---|
| A Notefile named `air.qo` carries cargo temperature, not air quality - the name is left over from an earlier product that shared the firmware. | `mapping` - it is what someone asking "where is the temperature data" must read. Also `vocabulary`, because the name is a false friend that will mislead every future reader. |
| Its schema still declares `pm02_5`, `humidity` and `pressure` from an older build, and no event has carried them in years. | `mapping` states what it actually carries. `drift` records the disagreement, since the schema is a candidate for cleanup. A query for `pm02_5` returns silence, not an error, so this one is load-bearing. |
| A smart fleet rule is inoperative, so its fleet is always empty. | `config`, because a rule that never fires is configuration that is a dead letter. Also `population`, because membership of that fleet carries no signal and must not be used to include or exclude devices. |

When a finding genuinely belongs in two kinds, write it in both rather than picking. The
files are small and an agent loads by kind; a fact that exists only in the file nobody
loaded may as well not have been learned.

But **one file owns each fact**. Write the full statement once, in the kind whose reader
needs it most, and have the others refer to it in a sentence rather than restate it. A
fact restated in five files will, sooner or later, be restated differently in five files
- and a reader who meets two versions cannot tell which to believe. A set that
contradicts itself is worse than a set with a gap, because a gap is visible.

If you are ever working alongside other agents on the same skill set, this stops being
advice and becomes the rule that keeps the set usable: agree who owns a fact before
anybody writes it down, and reconcile before you push.

And when a fact turns out to be contested, **separate settling it from applying it**.
One party goes and settles the question against primary sources and writes the exact
wording; everybody else applies that wording without re-deriving it. If each writer
re-checks for themselves, they will disagree again, then disagree differently after the
next repair, and the set will oscillate rather than converge - each round looking like
progress because every individual file got more rigorous. Re-derivation is how you find
the truth once. Applying an agreed wording is how fifteen files come to carry it.

A writer who thinks the agreed wording is wrong should say so loudly, and apply it
anyway. Being overruled and recorded is recoverable. A set where every file quietly
believes something different is not.

## Step 5 - How a claim records where it came from

This is the most important convention in this document, because it is what makes a
project safe to re-teach.

Every claim carries a marker saying where it came from and how sure you are:

```markdown
- An excursion means above 8 degrees for fifteen consecutive minutes, not a single
  sample. `[stated:dana, operations 2026-09-17 conf:high]`
- `t` is in tenths of a degree Celsius and is not converted before sending.
  `[documented:host firmware, reviewed 2026-09-17 conf:high]`
- The median interval between notes is 15 minutes across 240 devices.
  `[observed:events 2026-08-18..2026-09-17 n=31,000 conf:high]`
- The 8-degree limit may be contractual rather than physical.
  `[assumed conf:low]`
```

Four origins, and they are not interchangeable:

- **`stated`** - a human told you, in conversation with you, in this session. Record
  **who**; a field engineer and a CTO are not the same witness. A brief, a ticket, or a
  document that quotes a human is **not** `stated` - it is `documented`. This is the tag
  that gets abused, and it matters because `stated` is the one origin a later run may
  never overwrite.
- **`documented`** - you read it in firmware, a datasheet, a manual, a website. Record
  the file and commit, or the URL and the date.
- **`observed`** - you inferred it from data. Record the window and how many events.
- **`assumed`** - you guessed. Say so, and never let an assumed claim reach an answer
  unhedged.

**The precedence rule, which governs what an update run may do:**

1. `observed` and `assumed` may be overwritten freely by anything newer.
2. `documented` may be replaced by newer `documented`, or by `stated`.
3. **`stated` may never be silently rewritten or deleted.** If something now contradicts
   it, append the contradiction and raise it with the person. A human's claim can be
   wrong, but only a human may retract it.

Confidence is a separate axis from origin. A person can be confidently wrong; two
thousand observed events can be high confidence and three cannot.

**A default page is not a sample.** The single most common way to earn a false
`conf:high` is to read the first page of a paginated endpoint as though it were the
whole set: `/events` returns 50 rows unless you ask for more, and "50" then gets written
down as a total. Before you attach `conf:high` to any number, ask what would have
happened if there were more.

**Verify a load-bearing claim against a primary source before you write it, not after.**
If a sentence would change what somebody does, go and check it in the firmware, the
OpenAPI spec, or a read-only query. A research summary is a pointer, not a source.

**Know what your test could not have detected.** A measurement that comes back empty
proves nothing until you know it had the power to find what you were looking for. If you
test whether a value has been adjusted, and the adjustment would be smaller than the
precision the value is stored at, then a null result tells you about the storage and
nothing about the adjustment. State what your test could and could not have distinguished,
in the file, next to the finding. An unpowered null reported as a fact is the most
convincing wrong answer there is, because it survives review: everybody can reproduce it.

Front matter carries the roll-up, so that staleness can be judged without reading the
body:

```
---
kind: mapping,derivation
description: Every Notefile in this project, and what each field means
updated: 2026-09-17T14:02:00Z
sources:
  - documented: src/radtask.cpp, src/rad.cpp at commit 3f2a1b9
  - observed: 2000 events, 2026-08-18 to 2026-09-17
  - stated: andriy, 2026-09-17
---
```

## Step 5a - What you are actually reconstructing

The lessons below look like four separate sources to be cross-checked against each other.
They are not. They are four views of **one causal chain**, and reconstructing that chain
is the job:

> **the mission** says what the product is for, which is why the firmware senses what it
> senses - **the firmware** says how and above all *when* it senses, and what it emits in
> which situation - **the data** says what actually arrived, and therefore which of those
> situations really occur in the field - **the schema** says what Notehub inferred from
> all of it, which is neither the firmware's intent nor the data's reality.

Read in that direction and each source explains the next. The firmware is where you learn
that a device samples every fifteen minutes on solar and every hour on a low battery, that
a survey unit emits a track point every few seconds instead, that a field is written only
when a condition holds, that two notefiles are written and one is later discarded. None of
that is visible in the data - what you see there is a cadence that changes for no apparent
reason, a field that is sometimes absent, a device that looks silent. The firmware turns
those from anomalies into **scenarios**, and a scenario is the most valuable thing you can
write down, because it is what lets an agent tell a fault from normal operation.

So do not treat a disagreement between firmware and data as the point of the exercise. It
is a by-product. Where they agree, you have learned a mechanism, and that goes in
`scenarios` and `mapping`. Only where they genuinely conflict does anything belong in
`drift`.

**A caution learned the hard way.** A project's data may have been produced by an older
generation of firmware than the source you are handed. When that happens the temptation is
to discount the firmware and trust only the data - which throws away every mechanism and
leaves you with correlations. Do the opposite: use the firmware to generate candidate
explanations, then check each against the data, and say plainly which generation each
claim describes. A mechanism that is confirmed still explains, even if it was written for
a successor; a mechanism that is contradicted tells you exactly what changed between
generations, which is itself worth knowing.

## Step 6 - The lessons

Ask which the person wants, and say what each will cost them in effort. On an update run,
say which ones would refresh something stale.

**A. Learn from the host firmware.** Ask for it. Say why, and say what happens to it -
see "The promise you make about their firmware" below, and make that promise before you
ask, not after. Then ask for the path to the firmware that drives the Notecard, and the
commit. Read it for every `note.add`, `note.template`, `note.get`,
environment variable read, and Notefile name.

But an inventory of calls is the least of what firmware gives you. What you are really
after is **when each one fires, and under what conditions** - the power tiers, the
transport choice, the modes, the thresholds, the state machine. That is where `scenarios`
comes from, and it is the knowledge that turns an unexplained gap in the data into "this
device is on its winter battery tier, and it is behaving exactly as designed".

Firmware is also the only place that carries **`derivation`** - the constants and formulas
behind a field - and much of **`sentinels`**, because templates encode fields positionally
and an omitted field arrives as a zero rather than as nothing. A template declaration also
tells you the *wire type* of every field, which decides how much precision a value can
carry and therefore what a test on it can and cannot detect. Code comments are frequently
the only written record of what a field means, and a comment explaining why something was
*changed* is worth more than the code it explains.

Feeds `scenarios`, `mapping`, `derivation`, `sentinels`, `config`.

**B. Learn from the product materials.** Ask for a website, datasheets, manuals, support
articles, case studies - and ask for the internal ones too: requirements documents, design
docs, specifications, test plans. Those are usually the only written record of *why* the
product does what it does, and people will hand them over once they know what happens to
them. Make the promise below first. This is where the product, the
mission, the audiences, and above all the **thresholds** come from: the numbers dividing
normal from notable from alarming exist nowhere in the data. Feeds `product`, `mission`,
`audiences`, `constraints`, `vocabulary`.

**C. Learn from the live project.** You can do this alone, right now, with nothing from
them. Read the Notefile schemas, a sample of recent events, the fleets and how devices
are distributed among them, the routes, and the environment variables at project, fleet,
and device level. Then go back to the firmware with what you found and ask what
explains it: a cadence you observed should correspond to a tier you read, and if it does
not, you have either missed a scenario or found a real difference. Fleet names and smart
rules are worth particular attention: they are where operational meaning is written down, and a fleet called "Bad Tube" defined by
`$exists(body.t) = false or body.t = 0` teaches you more about that field than the
schema ever will. Feeds `mapping`, `population`, `config`, `cost`.

**D. Reconcile what you have learned.** Once you have both A and C, compare them and
report, without resolving anything yourself:

- fields the firmware sends that never arrive
- fields arriving that no current firmware produces
- fields declared in the schema that appear in no recent event
- notefiles whose names no longer describe their contents
- environment variables configured on fleets that the firmware never reads
- devices whose behavior is unlike their siblings

This is the single most valuable thing you can offer a developer, and it routinely
surfaces genuine bugs. Show them the disagreement and ask which side is correct. Feeds
`drift`.

**E. Learn the questions they actually ask.** Ask for the ten questions they or their
customers most want answered. For each, work out the exact call sequence and record it.
A taught project that cannot answer the questions its owner cares about has not been
taught. Feeds `recipe`, `verification`, `audiences`.

**F. Quiz me.** You ask the questions. Go through what you have written and find the
gaps: a field whose units you never determined, a notefile whose purpose you guessed, a
fleet whose name you do not understand, a device that behaves unlike its siblings.
Anything marked `assumed` is a question waiting to be asked. Ask one at a time and write
the answers down as `stated`.

**G. Check what is already known.** On an update run, take the new material and test it
against what is stored. See Step 7.

**H. Done for now.**

## Step 6a - The promise you make about their firmware

The best sources are the ones people are most reluctant to hand over. Firmware,
requirements documents and design specifications are where the how and the why live -
what the product senses, when, under what conditions, and what the fields were meant to
mean. Nothing else will tell you. So ask for them, plainly, and make this promise first,
in your own words, because it is the reason they can say yes:

> Give me the firmware and the internal documents and I will read them, and **nothing
> from them will appear in what I write.** No source, no comments, no file names, no
> internal names, no excerpts from your specifications. What goes into a skill is only
> what I *understood* from reading them - the meaning of a field, when a reading is taken,
> why a gap appears - and it is written so that it stands on its own, as a description of
> the data your project already contains.

Then keep it. What is stored is a **residual**: a learning that outlives the source and
does not carry it. This is what a skill is for anyway - an agent answering a question
needs to know that a missing reading means a failed sensor, and is not helped by knowing
which line of which file says so.

**What may never appear in a skill:**

- Source code, in any quantity. Not a function, not a line, not a fragment.
- Comments from the source, quoted or closely paraphrased.
- File names, paths, line numbers, commit hashes, branch names, repository names.
- Internal symbol names - functions, macros, constants, structs, enum members, variables.
- Anything quoted from a requirements or design document, and anything in it about
  unreleased plans, costs, suppliers, schedules or customers.
- Architecture that is not visible in the data: module structure, call flow, build layout.
- A description so specific that it would let a reader reconstruct the source.

**What may, and should:**

- What a field means, its units, its range, and what its absence means.
- When data is produced and what governs the cadence, as behavior.
- What situation produces what output, as a scenario a reader could confirm.
- A relationship between values, as a relationship - not as an implementation.
- Anything already public: a datasheet figure, a published specification, a marketing page.

**Provenance for a confidential source names the source, not the place in it.** Write
`[documented:host firmware, reviewed 2026-09-18 conf:high]`, never
`[documented:src/sensor.c@3f2a1b9:214 conf:high]`. The team that owns the firmware can
re-read their own firmware; everybody else must not be handed a map of it. The same goes
for a requirements document: name the document and the date, quote nothing from it.

**The test to apply to every sentence you write.** Would this still be true, and still
useful, to a reader who has never seen the firmware and never will? If it only makes
sense as a pointer into the source, it is a leak. If it describes the data and the
behavior, it is a residual, and it is what you were asked for.

One consequence worth stating: these files live in the customer's Notehub project and are
readable by anyone with viewer access to it, which is a wider audience than the people who
can see the firmware. Write for that audience. When in doubt, describe the behavior and
drop the citation.

**Ask, though, rather than assuming.** Some products are open: the firmware is public, the
hardware is documented, the whole thing is on the web. For those, a precise citation -
file, commit and line - is a gift rather than a leak, because it lets anyone reading a
skill go and check it. So when you ask for the sources, ask the same question about each
of them: *is this confidential, or is it public?* Treat everything as confidential until
they say otherwise, honor the answer, and **write the answer down in the skill set**, so
that a later run neither strips citations it was welcome to keep nor adds citations it
should not. A project whose sources are public should say so in its `index`.

## Step 7 - The contradiction pass

Whenever new material arrives for a project that has already been taught, before writing
anything, check it against what is stored.

For every claim you are about to write that disagrees with one already there:

1. If the stored claim is `observed` or `assumed`, replace it and mention that you did.
2. If it is `documented` and your source is newer, replace it and say what changed.
3. If it is `stated`, **stop and ask.** Show the person their own words, what now
   contradicts them, and where each came from. Let them decide. If they are not
   available, append the contradiction beside the claim rather than resolving it:

```markdown
- Devices report every 15 minutes. `[stated:andriy 2026-09-17 conf:high]`
  - Contradicted: 60-minute gaps on 31 of 343 devices over the last 30 days.
    `[observed:events 2026-08-18..2026-09-17 conf:high]`
```

Contradictions are findings, not errors. A human who said something six months ago may
simply be describing a fleet that has since been reconfigured - and that is exactly the
sort of thing the person will want to know.

## Step 8 - Write into the working copy, and upload only when they say so

**Nothing you write reaches the project until it is pushed.** Write after each lesson -
never accumulate a whole session in memory, because a session that is interrupted should
lose nothing - then show what is pending and ask before uploading.

```
notehub skills status -project <projectUID>      # what would change, and of what kind
notehub skills push -project <projectUID>        # upload the new and changed skills
notehub skills delete <name> -project <projectUID>
```

The working copy is theirs to edit. Encourage them to open it, argue with what you wrote,
fix it by hand, and sleep on it. `status` will still be true tomorrow.

Deleting a file from the working copy does **not** remove it from the project; `status`
reports it as deleted locally, and removing it takes the explicit `delete` command. That
asymmetry is deliberate, so a skill is never lost because a file went missing from
somebody's folder.

To see exactly what changed in a file, diff it against the copy of the project's version
in the working copy's `.baseline` folder.

Before anything large - a first push, a restructuring, a re-teach from new firmware -
offer a backup. It costs nothing and it makes the person free to experiment:

```
notehub skills backup -project <projectUID>          # onto their desktop, named for the
                                                     # project and the moment
notehub skills restore <file> -project <projectUID>  # put one back
```

A restore replaces the working copy and touches nothing in the project, so afterwards
`status` shows exactly what uploading it would do.

## Reaching the data

You have the person's credentials through the CLI:

```
curl -s -H "Authorization: Bearer $(notehub -token)" \
  "https://api.notefile.net/v1/projects/<projectUID>/schemas"
```

The API is fully specified; do not guess at an endpoint:

- Reference: https://dev.blues.io/api-reference/notehub-api
- OpenAPI: https://raw.githubusercontent.com/blues/notehub-js/main/openapi.yaml

The endpoints that matter for teaching are `/schemas` for what Notehub has inferred about
each Notefile, `/events` and `/events-cursor` for what is actually arriving, `/fleets`
and `/devices` for how the project is organized, `/routes` for where data goes, and
`/environment_variables` at project, fleet, and device level.

Things that are true of this API and cost people hours:

- **`/events` has no `limit` parameter.** It takes `pageSize`, which defaults to 50 and
  caps at 10000. Passing `limit` is silently ignored, so you get 50 rows and think that
  is all there is.
- **`dateType` defaults to `captured`**, the device's own clock, not `uploaded`, the
  server's. A Notecard that was offline for a week, or whose clock never set, lands
  outside the window you meant. "Nothing reported" is more often a clock artifact than a
  silence.
- **There is no total.** `/events` returns `has_more`, never a count. To count, use
  `/usage/events` with `period` and `aggregate` - it is the only server-side counter.
- **There is no filter on body fields.** Narrow server-side with `files`, `fleetUID`,
  `startDate` and `endDate`, project with `selectFields` and `format=csv`, then finish
  the job locally in DuckDB.
- **`/events-cursor` takes no date window.** It is for draining everything in order, not
  for a time range. Asking it for last week means downloading all history.
- **Do not loop over devices.** `/devices` already returns voltage, temperature, last
  activity, firmware versions, and location for every device in one call. Reach for the
  per-device endpoints only after you have narrowed to a few.

Some projects are additionally enabled for **Notehub IQ**, which mirrors events into
queryable tables and is far more efficient than paging the Events API for anything
aggregate. If the Notehub MCP is available, check whether this project is onboarded and
prefer it for questions spanning many events or a long period. A Notefile with no dataset
is invisible to it. Notehub IQ is optional and most projects do not have it, so never
assume it.

## Rules

1. **Shape, not data.** If a sentence would be wrong tomorrow, it does not belong in a
   skill.
2. **Never invent.** Mark a guess `assumed` and ask about it in lesson F. A confident
   wrong answer is worse than a gap.
3. **Mark every claim.** Origin governs what a later run may overwrite; confidence
   governs how an answer should be phrased.
4. **Never overwrite a human.** See Step 7.
5. **Sample, do not hoard.** Read enough events to establish shape, then stop.
6. **Small files, plain language, rich kinds.** Another agent will read these with no
   context but the project itself.
7. **Say what you changed.** Every time you write, say what you wrote and why.
8. **Never let their firmware or their documents leak into a skill.** You may read
   anything they give you; you may write down only what you learned from it. No code, no
   comments, no file names, no line numbers, no internal names, no excerpts. See Step 6a -
   this is a promise the tool makes, and it is the reason people hand over the sources
   that make the teaching worth doing.
9. **Never put a secret in a skill, and do not go looking for one.** A project's
   configuration carries route credentials, database passwords, and access tokens. You do
   not need any of them to describe the shape of a project. `hub.app.get` in particular
   returns the whole project configuration, secrets included - stay away from it.
