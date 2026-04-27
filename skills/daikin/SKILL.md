---
name: daikin
description: |
  Read state and control Daikin One+ thermostats via the daikin-cli Go binary.
  Use when the user asks about their thermostat, HVAC, current indoor/outdoor temp,
  Home/Away state, holds, scheduled wake/sleep times, or wants to change the
  thermostat from Claude (mode, setpoints, humidity, schedule).
---

# Daikin Skill

Read state and control Daikin One+ thermostats by shelling out to the `daikin-cli` Go binary. All commands return JSON. Setpoints are Celsius (API is Celsius-native). Convert from Fahrenheit with `C = (F - 32) * 5/9`.

## Auth + setup

The CLI reads its config from `~/.daikin/daikin.yaml` with `email`, `password`, and `temperatureUnit` fields (Skyport credentials). Not managed by this plugin — set up once via the README. Most commands need a `--device-id <uuid>`; find it via `daikin-cli device ls`.

## Two-axis state model

Two orthogonal state axes drive Daikin's thermostat:

1. **Away flag** (`geofencingAway`, boolean) — when true, the thermostat swaps active setpoints to `cspAway`/`hspAway`. The mobile app shows "Away". Toggled via `daikin-cli away --on/--off`.
2. **Schedule override** (`schedOverride`, 0/1) — when 1, active setpoints are pinned to `cspHome`/`hspHome` regardless of schedule. The app shows "Schedule overridden". Toggled via `daikin-cli hold` and cleared by `daikin-cli device resume`.

`daikin-cli device resume` clears both axes in one write. `away --off` does the same.

## Commands

### Read-only

```bash
daikin-cli device ls                      # list thermostats (ids, models, firmware)
daikin-cli device info -d <id>            # full raw device state (2000+ fields)
daikin-cli away -d <id>                   # curated Home/Away read
daikin-cli schedule get -d <id>           # full weekly schedule (7 days × 6 parts)
```

### Home/Away

```bash
daikin-cli away --on -d <id>              # writes geofencingAway=true; app shows "Away"
daikin-cli away --off -d <id>             # universal resume (same as device resume)
```

### Holds

```bash
daikin-cli hold --cool 22.2 --heat 18.9 --duration 2h -d <id>   # temp hold
daikin-cli hold --cool 22.2 --heat 18.9 --permanent -d <id>     # permanent hold
daikin-cli device resume -d <id>                                # clear all overrides + away
```

### Direct setters

```bash
daikin-cli mode --mode 3 -d <id>                 # 0=off, 1=heat, 2=cool, 3=auto, 4=emergency
daikin-cli temp --cool 22.2 --heat 18.9 -d <id>  # SetTemp with schedule override
daikin-cli humidity --humidify 40 --dehumidify 50 -d <id>
```

### Schedule editing

```bash
daikin-cli schedule set --day Mon --part 1 --time 06:30 --label Wake \
  --cool 22.2 --heat 18.9 --enable -d <id>
```

### Logging + reports

```bash
daikin-cli log -d <id>                            # log a snapshot to local DB
daikin-cli report summary -d <id>                 # HTML summary
daikin-cli report day 2026-04-27 -d <id>          # HTML day view
```

## Usage patterns

**"Am I home according to the thermostat?"** → `daikin-cli away -d <id>` → check `geofencingAway` and `schedOverride`.

**"Go into Away mode before my trip"** → `daikin-cli away --on -d <id>`. App shows "Away" until `away --off`.

**"Hold 72°F/66°F for 2 hours"** → `daikin-cli hold --cool 22.2 --heat 18.9 --duration 2h -d <id>`.

**"Hold until I manually cancel"** → add `--permanent`. Cancel with `daikin-cli device resume -d <id>`.

**"Return to normal schedule"** → `daikin-cli device resume -d <id>` (works regardless of current state).

**"Change wake time to 6:30am on Mondays"** → `daikin-cli schedule set --day Mon --part 1 --time 06:30 -d <id>`.

## Gotchas

- **Times are on 15-minute boundaries.** `06:00`, `06:15`, `06:30`, `06:45` — not `06:22`.
- **`cspHome`/`hspHome` are NOT a permanent Home preset.** They mirror the current active target. The permanent schedule lives in `schedMonPart1csp`, etc. — read via `schedule get`.
- **`geofencingAway` writes stick even when geofencing is disabled on your phone.** That's how the app's manual Home/Away button works.
- **`temp` and `hold` both set `schedOverride=1`.** Same mechanism, different UX. Prefer `hold` for clarity (it accepts a duration).
- **No bulk schedule write.** `schedule set` updates one block at a time (7 days × 6 parts = 42 max).

## Sibling

The same repo also ships an OpenClaw plugin under `openclaw/` that exposes these as named tools (`daikin_away_on`, `daikin_hold`, etc.). Functionally equivalent — different invocation surface for OpenClaw clients.
