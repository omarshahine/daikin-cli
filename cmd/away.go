package cmd

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/redgoose/daikin-skyport"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var awayOn bool
var awayOff bool

// buildResumePayload returns the universal "cancel everything" write. It
// clears any schedule override (manual or temp hold) AND clears geofence
// Away. Mirrors the mobile app's "Resume Program" button.
//
// Empirically, writing just {"schedOverride":0, ...} is ignored by the
// Skyport server. The override only clears when cspHome/hspHome are also
// written in the same PUT (same pattern SetTemp uses). We use the current
// scheduled setpoints as the targets so the thermostat lands on what the
// schedule says it should be.
func buildResumePayload(cspSched, hspSched float32) string {
	return fmt.Sprintf(
		`{"cspHome":%g,"hspHome":%g,"schedOverride":0,"schedEnabled":true,"geofencingAway":false}`,
		cspSched, hspSched,
	)
}

var awayCmd = &cobra.Command{
	Use:   "away",
	Args:  cobra.NoArgs,
	Short: "Toggle or inspect Home/Away state",
	Long: `Toggle or inspect the thermostat's Home/Away state.

Away state is controlled by the single boolean field geofencingAway. This
is the same mechanism the Daikin One mobile app's manual Home/Away button
uses, so the app's "Away" UI label specifically reflects this field. When
true, the thermostat swaps active setpoints to cspAway/hspAway.

Without flags, prints the current state snapshot.

--on writes {"geofencingAway": true}. Mobile app label: "Away".

--off writes the universal resume payload:
  {"schedEnabled": true, "schedOverride": 0, "geofencingAway": false}
This cancels any active schedule override (manual hold or temp hold) AND
clears Away in one write. Equivalent to "Resume Program" in the mobile app.
See also the standalone 'daikin-cli device resume' command.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if awayOn && awayOff {
			return errors.New("cannot set both --on and --off")
		}

		d := daikin.New(viper.GetString("email"), viper.GetString("password"))

		switch {
		case awayOn:
			if err := d.UpdateDeviceRaw(deviceId, `{"geofencingAway":true}`); err != nil {
				return fmt.Errorf("UpdateDeviceRaw(geofencingAway=true) failed: %w", err)
			}
			printResult(map[string]interface{}{
				"action": "away-on",
				"note":   "geofencingAway=true written; mobile app should show Away and swap to cspAway/hspAway",
			})

		case awayOff:
			info, err := d.GetDeviceInfo(deviceId)
			if err != nil {
				return fmt.Errorf("GetDeviceInfo failed: %w", err)
			}
			payload := buildResumePayload(info.CspSched, info.HspSched)
			if err := d.UpdateDeviceRaw(deviceId, payload); err != nil {
				return fmt.Errorf("UpdateDeviceRaw(resume) failed: %w", err)
			}
			printResult(map[string]interface{}{
				"action":       "away-off",
				"coolSetpoint": info.CspSched,
				"heatSetpoint": info.HspSched,
				"note":         "universal resume: override cancelled, schedule re-enabled, geofencingAway cleared",
			})

		default:
			info, err := d.GetDeviceInfo(deviceId)
			if err != nil {
				return fmt.Errorf("GetDeviceInfo failed: %w", err)
			}
			printResult(map[string]interface{}{
				"cspActive":             info.CspActive,
				"hspActive":             info.HspActive,
				"cspSched":              info.CspSched,
				"hspSched":              info.HspSched,
				"cspAway":               info.CspAway,
				"hspAway":               info.HspAway,
				"cspHome":               info.CspHome,
				"hspHome":               info.HspHome,
				"geofencingEnabled":     info.GeofencingEnabled,
				"geofencingAway":        info.GeofencingAway,
				"schedOverride":         info.SchedOverride,
				"schedOverrideDuration": info.SchedOverrideDuration,
				"schedEnabled":          info.SchedEnabled,
			})
		}

		return nil
	},
}

func printResult(v interface{}) {
	s, _ := json.MarshalIndent(v, "", "\t")
	fmt.Println(string(s))
}

func init() {
	deviceCmd.AddCommand(awayCmd)
	awayCmd.Flags().StringVarP(&deviceId, "device-id", "d", "", "Daikin device ID")
	awayCmd.Flags().BoolVar(&awayOn, "on", false, "Set geofencingAway=true (mobile app shows Away)")
	awayCmd.Flags().BoolVar(&awayOff, "off", false, "Universal resume: cancel override AND clear Away")
	awayCmd.MarkFlagsMutuallyExclusive("on", "off")
	awayCmd.MarkFlagRequired("device-id")
}
