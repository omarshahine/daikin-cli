package cmd

import (
	"fmt"

	"github.com/redgoose/daikin-skyport"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var resumeCmd = &cobra.Command{
	Use:   "resume",
	Args:  cobra.NoArgs,
	Short: "Universal cancel: clear any override AND clear Away",
	Long: `Writes the universal "Resume Program" payload:

  cspHome        = current scheduled cool setpoint
  hspHome        = current scheduled heat setpoint
  schedOverride  = 0
  schedEnabled   = true
  geofencingAway = false

This cancels any active schedule override (manual hold or temp hold) AND
clears the Away flag in one PUT. Equivalent to the "Resume Program" button
in the Daikin One mobile app.

Shares the same payload as 'daikin-cli device away --off'.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		d := daikin.New(viper.GetString("email"), viper.GetString("password"))
		info, err := d.GetDeviceInfo(deviceId)
		if err != nil {
			return fmt.Errorf("GetDeviceInfo failed: %w", err)
		}
		payload := buildResumePayload(info.CspSched, info.HspSched)
		if err := d.UpdateDeviceRaw(deviceId, payload); err != nil {
			return fmt.Errorf("UpdateDeviceRaw(resume) failed: %w", err)
		}
		printResult(map[string]interface{}{
			"action":       "resume",
			"coolSetpoint": info.CspSched,
			"heatSetpoint": info.HspSched,
			"note":         "override cancelled, schedule re-enabled, geofencingAway cleared",
		})
		return nil
	},
}

func init() {
	deviceCmd.AddCommand(resumeCmd)
	resumeCmd.Flags().StringVarP(&deviceId, "device-id", "d", "", "Daikin device ID")
	resumeCmd.MarkFlagRequired("device-id")
}
