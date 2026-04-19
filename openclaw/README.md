# daikin-cli OpenClaw plugin

OpenClaw plugin that wraps the [`daikin-cli`](https://github.com/omarshahine/daikin-cli) Go binary to read state and control Daikin One+ thermostats.

## Install

1. Install Go: https://golang.org/doc/install
2. Install the CLI:
   ```
   go install github.com/omarshahine/daikin-cli@latest
   ```
3. Create `~/.daikin/daikin.yaml` with your Daikin One+ app credentials:
   ```yaml
   email: you@example.com
   password: your-password
   temperatureUnit: F
   ```
4. Confirm the CLI works:
   ```
   daikin-cli device ls
   ```

## Configuration

| Field | Required | Description |
|---|---|---|
| `cliPath` | no | Path or command name for the `daikin-cli` binary. Defaults to PATH lookup. |
| `deviceId` | no | Default device UUID. Leave blank to auto-discover the first device on the account. |

Environment variable `DAIKIN_CLI_PATH` can override `cliPath` at runtime.

## Tools

| Tool | Purpose |
|---|---|
| `daikin_list_devices` | List thermostats on the account. |
| `daikin_info` | Full raw device state (2000+ fields). For diagnostics. |
| `daikin_away_status` | Curated Home/Away read. |
| `daikin_away_on` | Set `geofencingAway: true` (app shows "Away"). |
| `daikin_away_off` | Universal resume (cancel override + clear Away). |
| `daikin_resume` | Same payload as `daikin_away_off`; standalone command. |
| `daikin_hold` | Temp hold (`--duration 2h`) or permanent hold (`--permanent`). |
| `daikin_schedule_get` | Read weekly schedule (7 days × 6 parts). |
| `daikin_schedule_set_part` | Edit one schedule block. |
| `daikin_humidity` | Set humidifier and/or dehumidifier targets. |
| `daikin_set_mode` | Change mode (off / heat / cool / auto / em-heat). |
| `daikin_set_temp` | Set cool and/or heat setpoints (Celsius). |

Full tool reference: [`skills/daikin/SKILL.md`](skills/daikin/SKILL.md).

## Architecture

The plugin shells out to the `daikin-cli` binary via `child_process.execFile`. All tools return parsed JSON.

Device ID resolution order:
1. Explicit `deviceId` param on the tool call
2. Plugin config `deviceId`
3. Auto-discovered via `daikin-cli device ls` (cached for the plugin's lifetime)

Auth is handled by the CLI itself via `~/.daikin/daikin.yaml`. The plugin verifies the file exists before invoking any tool and returns a structured error otherwise.

### State model

Two orthogonal state axes:

- **Away flag** (`geofencingAway`) — toggled by `daikin_away_on`/`off`. Mobile app shows "Away".
- **Schedule override** (`schedOverride`) — engaged by `daikin_hold` or `daikin_set_temp`. Mobile app shows "Schedule overridden".

`daikin_resume` clears both axes in one call.

## License

MIT
