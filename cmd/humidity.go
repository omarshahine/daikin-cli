package cmd

import (
	"errors"
	"fmt"
	"strings"

	"github.com/redgoose/daikin-skyport"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var humHumidify int
var humDehumidify int

var humidityCmd = &cobra.Command{
	Use:   "humidity",
	Args:  cobra.NoArgs,
	Short: "Set humidifier and/or dehumidifier target percentages",
	Long: `Set the humidifier target (humSP) and/or dehumidifier target (dehumSP)
as integer percentages.

  daikin-cli device humidity --humidify 35 --dehumidify 55

Either flag may be omitted. At least one is required.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if humHumidify == 0 && humDehumidify == 0 {
			return errors.New("at least one of --humidify or --dehumidify is required")
		}

		if humHumidify < 0 || humHumidify > 100 || humDehumidify < 0 || humDehumidify > 100 {
			return errors.New("humidity values must be 0-100")
		}

		var parts []string
		if humHumidify > 0 {
			parts = append(parts, fmt.Sprintf(`"humSP":%d`, humHumidify))
		}
		if humDehumidify > 0 {
			parts = append(parts, fmt.Sprintf(`"dehumSP":%d`, humDehumidify))
		}
		payload := "{" + strings.Join(parts, ",") + "}"

		d := daikin.New(viper.GetString("email"), viper.GetString("password"))
		if err := d.UpdateDeviceRaw(deviceId, payload); err != nil {
			return fmt.Errorf("UpdateDeviceRaw(humidity) failed: %w", err)
		}

		result := map[string]interface{}{"action": "humidity"}
		if humHumidify > 0 {
			result["humSP"] = humHumidify
		}
		if humDehumidify > 0 {
			result["dehumSP"] = humDehumidify
		}
		printResult(result)
		return nil
	},
}

func init() {
	deviceCmd.AddCommand(humidityCmd)
	humidityCmd.Flags().StringVarP(&deviceId, "device-id", "d", "", "Daikin device ID")
	humidityCmd.Flags().IntVar(&humHumidify, "humidify", 0, "Humidifier target % (0-100)")
	humidityCmd.Flags().IntVar(&humDehumidify, "dehumidify", 0, "Dehumidifier target % (0-100)")
	humidityCmd.MarkFlagRequired("device-id")
}
