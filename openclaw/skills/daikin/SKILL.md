---
name: daikin
description: |
  Read state and control Daikin One+ thermostats: check current state; toggle Home/Away;
  apply temp or permanent holds; edit the weekly schedule; set humidifier/dehumidifier targets;
  change operating mode; set cooling/heating setpoints; return to schedule via universal resume.
  Use when the user asks about their thermostat, HVAC, current indoor/outdoor temperature,
  Home/Away state, scheduled wake/sleep times, or wants to change the thermostat from Claude/OpenClaw.
license: MIT
metadata:
  author: Omar Shahine
  version: 0.2.0
  openclaw:
    requires:
      bins: [daikin-cli]
---

# Daikin Skill

Read state and control Daikin One+ thermostats via the `daikin-cli` Go binary.

All tools return JSON. Setpoints are always Celsius (API is Celsius-native). Convert from Fahrenheit with `C = (F - 32) * 5/9`.

## Two-axis state model

Daikin's thermostat has two orthogonal state axes you need to understand:

1. **Away flag** (`geofencingAway`, boolean) — when true, the thermostat swaps active setpoints to `cspAway`/`hspAway`. The mobile app shows "Away". Controlled by `daikin_away_on` / `daikin_away_off`.
2. **Schedule override** (`schedOverride`, 0/1) — when 1, active setpoints are pinned to `cspHome`/`hspHome` regardless of schedule. The mobile app shows "Schedule overridden". Controlled by `daikin_hold`.

`daikin_resume` clears both axes in one call. `daikin_away_off` does the same (shared payload).

## Tools

### Read-only

| Tool | Purpose |
|---|---|
| `daikin_list_devices` | List thermostats on the account (ids, models, firmware). |
| `daikin_info` | Full raw device state (2000+ fields). Use for diagnostics. |
| `daikin_away_status` | Curated Home/Away read (active setpoints, schedule, geofence, override). |
| `daikin_schedule_get` | Full weekly schedule: 7 days × 6 parts, with times/labels/setpoints. |

### Home/Away

| Tool | Payload |
|---|---|
| `daikin_away_on` | `{"geofencingAway": true}` → app label: "Away", active → cspAway/hspAway |
| `daikin_away_off` | Universal resume (same as `daikin_resume`) |

### Overrides & holds

| Tool | Payload |
|---|---|
| `daikin_hold { duration: "2h" }` | Temp hold: `{cspHome, hspHome, schedOverride:1, schedOverrideDuration:120}` |
| `daikin_hold { permanent: true }` | Permanent hold: `{cspHome, hspHome, schedOverride:0, schedEnabled:false}` |
| `daikin_resume` | `{cspHome:cspSched, hspHome:hspSched, schedOverride:0, schedEnabled:true, geofencingAway:false}` |

### Schedule editing

| Tool | Purpose |
|---|---|
| `daikin_schedule_set_part { day, part, time, label, cool, heat, enable }` | Update one block of the 7×6 schedule grid. |

### Direct setters

| Tool | Payload |
|---|---|
| `daikin_set_mode { mode: 3 }` | `mode` enum: 0=off, 1=heat, 2=cool, 3=auto, 4=emergency heat |
| `daikin_set_temp { cool, heat }` | `SetTemp` — applies schedule override with new targets |
| `daikin_humidity { humidify, dehumidify }` | `{humSP, dehumSP}` — 0-100 integers |

## Usage patterns

**"Am I home according to the thermostat?"** → `daikin_away_status` → check `geofencingAway` and `schedOverride`.

**"Go into Away mode before my trip"** → `daikin_away_on`. App will show "Away" until `daikin_away_off`.

**"Hold 72°F/66°F for 2 hours"** → `daikin_hold { cool: 22.2, heat: 18.9, duration: "2h" }`.

**"Hold until I manually cancel"** → `daikin_hold { cool, heat, permanent: true }`. Cancel with `daikin_resume`.

**"Return to normal schedule"** → `daikin_resume` (works regardless of current override or away state).

**"Change wake time to 6:30am on Mondays"** → `daikin_schedule_set_part { day: "Mon", part: 1, time: "06:30" }`.

## Gotchas

- **Times are on 15-minute boundaries.** `06:00`, `06:15`, `06:30`, `06:45` — not `06:22`.
- **cspHome/hspHome are NOT a permanent Home preset.** They're a mirror of the current active target. The permanent schedule lives in `schedMonPart1csp`, etc. (`daikin_schedule_get` reads these).
- **geofencingAway writes stick even when geofencing is disabled on your phone.** This is how the mobile app's manual Home/Away button works.
- **`SetTemp` and `daikin_hold` both set schedOverride=1.** They're the same mechanism with different UX. Prefer `daikin_hold` for clarity.
- **Auth**: `~/.daikin/daikin.yaml` with email, password, temperatureUnit. Not managed by this plugin.

## Changelog

- **v0.2.0** — Add `daikin_resume`, `daikin_hold`, `daikin_schedule_get`, `daikin_schedule_set_part`, `daikin_humidity`. Rewrite `daikin_away_on`/`off` with correct Skyport semantics (geofencingAway field, not schedOverride).
- **v0.1.0** — Initial plugin: list/info/away_status/on/off, set_mode, set_temp. Away used SetTemp (wrong semantics; the app labeled result as "Schedule overridden" not "Away").
