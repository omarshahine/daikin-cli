/**
 * OpenClaw plugin entry for daikin-cli.
 *
 * Registers tools that shell out to the `daikin-cli` binary to read state
 * and control Daikin One+ thermostats (mode, setpoints, Home/Away).
 */

import { definePluginEntry, type OpenClawPluginDefinition } from 'openclaw/plugin-sdk/plugin-entry';
import { Type } from '@sinclair/typebox';
import { execFileSync, execFile } from 'child_process';
import { promisify } from 'util';
import { existsSync } from 'fs';
import { homedir } from 'os';
import { join } from 'path';

const execFileAsync = promisify(execFile);

interface PluginConfig {
	cliPath?: string;
	deviceId?: string;
}

interface ToolDef {
	name: string;
	description: string;
	parameters: ReturnType<typeof Type.Object>;
	requiresDeviceId: boolean;
	buildArgs: (params: Record<string, unknown>, deviceId: string | null) => string[];
}

const TOOLS: ToolDef[] = [
	{
		name: 'daikin_list_devices',
		description:
			'List Daikin thermostats on the account. Returns id, name, model, firmwareVersion, and location. Use this to find the device ID needed by other tools.',
		parameters: Type.Object({}),
		requiresDeviceId: false,
		buildArgs: () => ['device', 'ls'],
	},
	{
		name: 'daikin_info',
		description:
			'Return full raw device state (2000+ fields: temps, humidity, setpoints, equipment telemetry, schedule, geofence). Use for deep diagnostics; prefer daikin_away_status for everyday Home/Away questions.',
		parameters: Type.Object({
			deviceId: Type.Optional(
				Type.String({ description: 'Override the plugin-configured device ID.' })
			),
		}),
		requiresDeviceId: true,
		buildArgs: (_, id) => ['device', 'info', '-d', id!],
	},
	{
		name: 'daikin_away_status',
		description:
			'Return a curated Home/Away view: active setpoints, scheduled setpoints, configured Away preset, geofence state, and any active schedule override. Read-only and fast. Use this for "is the thermostat in Away mode?" style questions.',
		parameters: Type.Object({
			deviceId: Type.Optional(
				Type.String({ description: 'Override the plugin-configured device ID.' })
			),
		}),
		requiresDeviceId: true,
		buildArgs: (_, id) => ['device', 'away', '-d', id!],
	},
	{
		name: 'daikin_away_on',
		description:
			'Apply a manual schedule override that switches the thermostat to its configured Away setpoints (cspAway/hspAway). Use when you want to force Away mode (e.g., before leaving the house) regardless of geofence. The override persists until the next scheduled event or until daikin_away_off is called. Does NOT mutate the permanent schedule.',
		parameters: Type.Object({
			deviceId: Type.Optional(
				Type.String({ description: 'Override the plugin-configured device ID.' })
			),
		}),
		requiresDeviceId: true,
		buildArgs: (_, id) => ['device', 'away', '--on', '-d', id!],
	},
	{
		name: 'daikin_away_off',
		description:
			'Cancel any active schedule override and return control to the schedule and geofence system. Use when you want to "come back home" (e.g., after an early return from a trip) so the thermostat resumes normal scheduled behavior.',
		parameters: Type.Object({
			deviceId: Type.Optional(
				Type.String({ description: 'Override the plugin-configured device ID.' })
			),
		}),
		requiresDeviceId: true,
		buildArgs: (_, id) => ['device', 'away', '--off', '-d', id!],
	},
	{
		name: 'daikin_set_mode',
		description:
			'Set the operating mode. Valid values: 0=off, 1=heat, 2=cool, 3=auto, 4=emergency heat. Use daikin_info first to check current mode.',
		parameters: Type.Object({
			mode: Type.Integer({
				minimum: 0,
				maximum: 4,
				description: '0=off, 1=heat, 2=cool, 3=auto, 4=emergency heat',
			}),
			deviceId: Type.Optional(
				Type.String({ description: 'Override the plugin-configured device ID.' })
			),
		}),
		requiresDeviceId: true,
		buildArgs: (params, id) => ['device', 'mode', '--mode', String(params.mode), '-d', id!],
	},
	{
		name: 'daikin_set_temp',
		description:
			'Set cooling and/or heating setpoints in Celsius (the Daikin API is Celsius-native). Convert Fahrenheit before calling: C = (F - 32) * 5/9. Cool setpoint must be higher than heat by at least the device tempDeltaMin. Applies a schedule override; call daikin_away_off to return to schedule.',
		parameters: Type.Object({
			cool: Type.Optional(
				Type.Number({ description: 'Cool setpoint in °C (e.g., 22.2 for 72°F)' })
			),
			heat: Type.Optional(
				Type.Number({ description: 'Heat setpoint in °C (e.g., 18.9 for 66°F)' })
			),
			deviceId: Type.Optional(
				Type.String({ description: 'Override the plugin-configured device ID.' })
			),
		}),
		requiresDeviceId: true,
		buildArgs: (params, id) => {
			const args = ['device', 'temp', '-d', id!];
			if (params.cool !== undefined && params.cool !== null) {
				args.push('--cool', String(params.cool));
			}
			if (params.heat !== undefined && params.heat !== null) {
				args.push('--heat', String(params.heat));
			}
			return args;
		},
	},
	{
		name: 'daikin_resume',
		description:
			'Universal cancel: clears any active schedule override (manual hold or temp hold) AND clears Away in one write. Equivalent to the "Resume Program" button in the Daikin One mobile app. Use when you want to return the thermostat to fully-scheduled behavior.',
		parameters: Type.Object({
			deviceId: Type.Optional(
				Type.String({ description: 'Override the plugin-configured device ID.' })
			),
		}),
		requiresDeviceId: true,
		buildArgs: (_, id) => ['device', 'resume', '-d', id!],
	},
	{
		name: 'daikin_hold',
		description:
			'Apply a manual hold on setpoints. Temp hold (schedule resumes after duration) or permanent hold (schedule disabled until daikin_resume). At least one of cool/heat required, plus either duration or permanent. Setpoints are Celsius.',
		parameters: Type.Object({
			cool: Type.Optional(
				Type.Number({ description: 'Cool setpoint in °C (e.g., 22.2 for 72°F)' })
			),
			heat: Type.Optional(
				Type.Number({ description: 'Heat setpoint in °C (e.g., 18.9 for 66°F)' })
			),
			duration: Type.Optional(
				Type.String({
					description:
						'Temp hold duration (Go duration string: "2h", "90m", "1h30m"). Max 24h. Mutually exclusive with permanent.',
				})
			),
			permanent: Type.Optional(
				Type.Boolean({
					description: 'Apply permanent hold (schedEnabled=false until daikin_resume).',
				})
			),
			deviceId: Type.Optional(
				Type.String({ description: 'Override the plugin-configured device ID.' })
			),
		}),
		requiresDeviceId: true,
		buildArgs: (params, id) => {
			const args = ['device', 'hold', '-d', id!];
			if (params.cool !== undefined && params.cool !== null) {
				args.push('--cool', String(params.cool));
			}
			if (params.heat !== undefined && params.heat !== null) {
				args.push('--heat', String(params.heat));
			}
			if (params.duration) {
				args.push('--duration', String(params.duration));
			}
			if (params.permanent === true) {
				args.push('--permanent');
			}
			return args;
		},
	},
	{
		name: 'daikin_schedule_get',
		description:
			'Read the full weekly schedule as JSON. Returns 7 days × 6 parts each (time HH:MM, label, cool/heat setpoints, enabled flag). Also returns schedEnabled at the top level.',
		parameters: Type.Object({
			deviceId: Type.Optional(
				Type.String({ description: 'Override the plugin-configured device ID.' })
			),
		}),
		requiresDeviceId: true,
		buildArgs: (_, id) => ['device', 'schedule', 'get', '-d', id!],
	},
	{
		name: 'daikin_schedule_set_part',
		description:
			'Update one part of the weekly schedule. Only the provided fields are written; unspecified fields remain untouched. Times must be on a 15-minute boundary (HH:00, HH:15, HH:30, HH:45). Setpoints Celsius.',
		parameters: Type.Object({
			day: Type.Union(
				[
					Type.Literal('Mon'),
					Type.Literal('Tue'),
					Type.Literal('Wed'),
					Type.Literal('Thu'),
					Type.Literal('Fri'),
					Type.Literal('Sat'),
					Type.Literal('Sun'),
				],
				{ description: 'Three-letter day name' }
			),
			part: Type.Integer({ minimum: 1, maximum: 6, description: 'Schedule part 1-6' }),
			time: Type.Optional(
				Type.String({ description: 'Start time HH:MM (15-min boundary, 24h format)' })
			),
			label: Type.Optional(
				Type.String({ description: 'Label (e.g., wake, sleep, work, home)' })
			),
			cool: Type.Optional(Type.Number({ description: 'Cool setpoint °C' })),
			heat: Type.Optional(Type.Number({ description: 'Heat setpoint °C' })),
			enable: Type.Optional(Type.Boolean({ description: 'Enable this schedule part' })),
			disable: Type.Optional(Type.Boolean({ description: 'Disable this schedule part' })),
			deviceId: Type.Optional(
				Type.String({ description: 'Override the plugin-configured device ID.' })
			),
		}),
		requiresDeviceId: true,
		buildArgs: (params, id) => {
			const args = ['device', 'schedule', 'set-part', String(params.day), String(params.part), '-d', id!];
			if (params.time) args.push('--time', String(params.time));
			if (params.label) args.push('--label', String(params.label));
			if (params.cool !== undefined && params.cool !== null) {
				args.push('--cool', String(params.cool));
			}
			if (params.heat !== undefined && params.heat !== null) {
				args.push('--heat', String(params.heat));
			}
			if (params.enable === true) args.push('--enable');
			if (params.disable === true) args.push('--disable');
			return args;
		},
	},
	{
		name: 'daikin_humidity',
		description:
			'Set humidifier (humSP) and/or dehumidifier (dehumSP) target percentages. Values are integers 0-100. At least one required.',
		parameters: Type.Object({
			humidify: Type.Optional(
				Type.Integer({ minimum: 0, maximum: 100, description: 'Humidifier target %' })
			),
			dehumidify: Type.Optional(
				Type.Integer({ minimum: 0, maximum: 100, description: 'Dehumidifier target %' })
			),
			deviceId: Type.Optional(
				Type.String({ description: 'Override the plugin-configured device ID.' })
			),
		}),
		requiresDeviceId: true,
		buildArgs: (params, id) => {
			const args = ['device', 'humidity', '-d', id!];
			if (params.humidify !== undefined && params.humidify !== null) {
				args.push('--humidify', String(params.humidify));
			}
			if (params.dehumidify !== undefined && params.dehumidify !== null) {
				args.push('--dehumidify', String(params.dehumidify));
			}
			return args;
		},
	},
];

/** Build a tool result with the required content + details shape. */
function toolResult(text: string) {
	return {
		content: [{ type: 'text' as const, text }],
		details: undefined,
	};
}

/** Look up a binary on PATH, cross-platform. */
function whichBinary(name: string): string | null {
	const cmd = process.platform === 'win32' ? 'where.exe' : 'which';
	try {
		const result = execFileSync(cmd, [name], { encoding: 'utf8' }).trim();
		const first = result.split('\n')[0]?.trim();
		return first || null;
	} catch {
		return null;
	}
}

/**
 * Resolve the CLI binary path:
 * 1. Plugin config cliPath
 * 2. Env var DAIKIN_CLI_PATH
 * 3. PATH lookup
 */
function resolveCliPath(config?: PluginConfig): string {
	if (config?.cliPath && existsSync(config.cliPath)) {
		return config.cliPath;
	}

	const envPath = process.env.DAIKIN_CLI_PATH;
	if (envPath && existsSync(envPath)) {
		return envPath;
	}

	const found = whichBinary('daikin-cli');
	if (found) return found;

	throw new Error(
		'daikin-cli not found. Install with: go install github.com/omarshahine/daikin-cli@latest\n' +
			'Or set DAIKIN_CLI_PATH or configure cliPath in plugin settings.'
	);
}

/** Daikin credentials live in ~/.daikin/daikin.yaml (email + password). */
function isAuthConfigured(): boolean {
	return existsSync(join(homedir(), '.daikin', 'daikin.yaml'));
}

/**
 * Resolve the device ID for a tool call:
 * 1. Explicit `deviceId` in the tool params (per-call override)
 * 2. Plugin config `deviceId`
 * 3. First device returned by `daikin-cli device ls` (auto-discover, cached)
 */
async function resolveDeviceId(
	cliPath: string,
	config: PluginConfig | undefined,
	params: Record<string, unknown>,
	cache: { value: string | null }
): Promise<string> {
	if (typeof params.deviceId === 'string' && params.deviceId.length > 0) {
		return params.deviceId;
	}
	if (config?.deviceId) return config.deviceId;
	if (cache.value) return cache.value;

	const { stdout } = await execFileAsync(cliPath, ['device', 'ls'], {
		encoding: 'utf8',
		timeout: 15_000,
	});
	const devices = JSON.parse(stdout) as Array<{ id: string }>;
	if (!Array.isArray(devices) || devices.length === 0 || !devices[0]?.id) {
		throw new Error('No Daikin devices found on account.');
	}
	cache.value = devices[0].id;
	return cache.value;
}

const pluginEntry: OpenClawPluginDefinition = definePluginEntry({
	id: 'daikin-cli',
	name: 'Daikin',
	description: 'Read state and control Daikin One+ thermostats',

	register(api) {
		const config = api.pluginConfig as PluginConfig | undefined;

		let cliPath: string;
		try {
			cliPath = resolveCliPath(config);
		} catch (error) {
			const errorMessage = error instanceof Error ? error.message : String(error);
			for (const tool of TOOLS) {
				api.registerTool({
					name: tool.name,
					label: tool.name,
					description: tool.description,
					parameters: tool.parameters,
					async execute() {
						return toolResult(
							JSON.stringify({ success: false, error: errorMessage }, null, 2)
						);
					},
				});
			}
			return;
		}

		// Auto-discovered device ID is cached across calls within one plugin
		// lifetime — avoids a `device ls` round-trip on every tool invocation.
		const deviceIdCache = { value: null as string | null };

		for (const tool of TOOLS) {
			api.registerTool({
				name: tool.name,
				label: tool.name,
				description: tool.description,
				parameters: tool.parameters,

				async execute(_id: string, params: Record<string, unknown>) {
					if (!isAuthConfigured()) {
						return toolResult(
							JSON.stringify(
								{
									success: false,
									error:
										'Daikin credentials not configured. Create ~/.daikin/daikin.yaml with email and password fields.',
								},
								null,
								2
							)
						);
					}

					try {
						let deviceId: string | null = null;
						if (tool.requiresDeviceId) {
							deviceId = await resolveDeviceId(cliPath, config, params, deviceIdCache);
						}

						const args = tool.buildArgs(params, deviceId);
						const { stdout } = await execFileAsync(cliPath, args, {
							encoding: 'utf8',
							timeout: 30_000,
							maxBuffer: 4 * 1024 * 1024,
						});

						if (stdout.trim().length === 0) {
							return toolResult(JSON.stringify({ success: true }, null, 2));
						}

						let result: unknown;
						try {
							result = JSON.parse(stdout);
						} catch {
							result = { output: stdout.trim() };
						}
						return toolResult(JSON.stringify(result, null, 2));
					} catch (error: unknown) {
						const message = error instanceof Error ? error.message : String(error);
						const stderr =
							error && typeof error === 'object' && 'stderr' in error
								? String((error as { stderr: unknown }).stderr).trim()
								: '';
						const errorOutput = stderr ? `${message}\n\nstderr: ${stderr}` : message;
						return toolResult(
							JSON.stringify({ success: false, error: errorOutput }, null, 2)
						);
					}
				},
			});
		}
	},
});

export default pluginEntry;
