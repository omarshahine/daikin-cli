package cmd

import (
	"errors"
	"fmt"
	"time"

	"github.com/redgoose/daikin-skyport"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var holdCool float32
var holdHeat float32
var holdDuration string
var holdPermanent bool

var holdCmd = &cobra.Command{
	Use:   "hold",
	Args:  cobra.NoArgs,
	Short: "Apply a temp or permanent hold (manual setpoint override)",
	Long: `Apply a manual hold on the thermostat's setpoints.

Temp hold (holds for a fixed duration, then schedule resumes):
  daikin-cli device hold --cool 22.2 --heat 18.9 --duration 2h

Permanent hold (holds until explicitly resumed):
  daikin-cli device hold --cool 22.2 --heat 18.9 --permanent

One of --cool or --heat may be omitted; the omitted value falls back to the
current active target. Setpoints are in Celsius.

--duration accepts Go duration strings: "90m", "2h", "1h30m". The Daikin
API stores the value in whole minutes. Maximum is 24h.

--permanent disables the schedule entirely (schedEnabled=false). Re-enable
via 'daikin-cli device resume' or the mobile app's Resume Program button.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if holdDuration == "" && !holdPermanent {
			return errors.New("one of --duration or --permanent is required")
		}
		if holdDuration != "" && holdPermanent {
			return errors.New("--duration and --permanent are mutually exclusive")
		}
		if holdCool == 0 && holdHeat == 0 {
			return errors.New("at least one of --cool or --heat is required")
		}

		d := daikin.New(viper.GetString("email"), viper.GetString("password"))
		info, err := d.GetDeviceInfo(deviceId)
		if err != nil {
			return fmt.Errorf("GetDeviceInfo failed: %w", err)
		}

		cool := holdCool
		heat := holdHeat
		if cool == 0 {
			cool = info.CspActive
		}
		if heat == 0 {
			heat = info.HspActive
		}

		if cool <= heat {
			return fmt.Errorf("cool setpoint (%g) must be higher than heat (%g)", cool, heat)
		}
		if cool < info.TempSPMin || cool > info.TempSPMax || heat < info.TempSPMin || heat > info.TempSPMax {
			return fmt.Errorf("setpoint out of range [%g, %g]", info.TempSPMin, info.TempSPMax)
		}

		var payload string
		note := ""

		if holdPermanent {
			payload = fmt.Sprintf(
				`{"cspHome":%g,"hspHome":%g,"schedOverride":0,"schedEnabled":false}`,
				cool, heat,
			)
			note = "permanent hold; schedule disabled (resume with 'device resume')"
		} else {
			dur, err := time.ParseDuration(holdDuration)
			if err != nil {
				return fmt.Errorf("invalid --duration %q: %w", holdDuration, err)
			}
			minutes := int(dur.Minutes())
			if minutes <= 0 {
				return errors.New("--duration must be at least 1 minute")
			}
			if minutes > 24*60 {
				return errors.New("--duration cannot exceed 24h")
			}
			payload = fmt.Sprintf(
				`{"cspHome":%g,"hspHome":%g,"schedOverride":1,"schedOverrideDuration":%d}`,
				cool, heat, minutes,
			)
			note = fmt.Sprintf("temp hold for %d minutes (%s); schedule will resume automatically", minutes, dur)
		}

		if err := d.UpdateDeviceRaw(deviceId, payload); err != nil {
			return fmt.Errorf("UpdateDeviceRaw(hold) failed: %w", err)
		}
		printResult(map[string]interface{}{
			"action":       "hold",
			"coolSetpoint": cool,
			"heatSetpoint": heat,
			"permanent":    holdPermanent,
			"note":         note,
		})
		return nil
	},
}

func init() {
	deviceCmd.AddCommand(holdCmd)
	holdCmd.Flags().StringVarP(&deviceId, "device-id", "d", "", "Daikin device ID")
	holdCmd.Flags().Float32Var(&holdCool, "cool", 0, "Cool setpoint in °C")
	holdCmd.Flags().Float32Var(&holdHeat, "heat", 0, "Heat setpoint in °C")
	holdCmd.Flags().StringVar(&holdDuration, "duration", "", "Duration for temp hold (e.g., 2h, 90m)")
	holdCmd.Flags().BoolVar(&holdPermanent, "permanent", false, "Apply permanent hold (disables schedule until resume)")
	holdCmd.MarkFlagsMutuallyExclusive("duration", "permanent")
	holdCmd.MarkFlagRequired("device-id")
}
