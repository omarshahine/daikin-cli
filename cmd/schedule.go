package cmd

import (
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"github.com/redgoose/daikin-skyport"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// The Daikin schedule is a flat block of ~252 fields on the device: 7 days
// × 6 parts × 6 sub-fields (Time, Enabled, Label, csp, hsp, Action). We
// use reflection to read/write them by name since enumerating all 252
// statically would be painful.
//
// Time is stored as a 15-minute unit: 00:00 = 0, 06:00 = 24, 12:00 = 48,
// 23:45 = 95.

var scheduleDays = []string{"Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"}

type schedulePart struct {
	Part    int     `json:"part"`
	Time    string  `json:"time"`
	Enabled bool    `json:"enabled"`
	Label   string  `json:"label"`
	Cool    float32 `json:"cool"`
	Heat    float32 `json:"heat"`
	Action  int     `json:"action"`
}

var scheduleCmd = &cobra.Command{
	Use:   "schedule",
	Args:  cobra.NoArgs,
	Short: "Read or edit the weekly schedule (7 days × 6 parts)",
}

var scheduleGetCmd = &cobra.Command{
	Use:   "get",
	Args:  cobra.NoArgs,
	Short: "Print the weekly schedule as JSON",
	RunE: func(cmd *cobra.Command, args []string) error {
		d := daikin.New(viper.GetString("email"), viper.GetString("password"))
		info, err := d.GetDeviceInfo(deviceId)
		if err != nil {
			return fmt.Errorf("GetDeviceInfo failed: %w", err)
		}

		result := make(map[string][]schedulePart, 7)
		v := reflect.ValueOf(info).Elem()
		for _, day := range scheduleDays {
			parts := make([]schedulePart, 0, 6)
			for p := 1; p <= 6; p++ {
				prefix := fmt.Sprintf("Sched%sPart%d", day, p)
				part := schedulePart{
					Part:    p,
					Time:    minutesToHHMM(int(v.FieldByName(prefix + "Time").Int())),
					Enabled: v.FieldByName(prefix + "Enabled").Bool(),
					Label:   v.FieldByName(prefix + "Label").String(),
					Cool:    float32(v.FieldByName(prefix + "Csp").Float()),
					Heat:    float32(v.FieldByName(prefix + "Hsp").Float()),
					Action:  int(v.FieldByName(prefix + "Action").Int()),
				}
				parts = append(parts, part)
			}
			result[day] = parts
		}

		printResult(map[string]interface{}{
			"schedEnabled": info.SchedEnabled,
			"days":         result,
		})
		return nil
	},
}

var (
	schedPartTime    string
	schedPartLabel   string
	schedPartCool    float32
	schedPartHeat    float32
	schedPartEnable  bool
	schedPartDisable bool
)

var scheduleSetPartCmd = &cobra.Command{
	Use:   "set-part [day] [part]",
	Args:  cobra.ExactArgs(2),
	Short: "Update one part of a day's schedule",
	Long: `Update a single part of the weekly schedule.

  daikin-cli device schedule set-part Mon 1 --time 06:00 --label wake --cool 23.3 --heat 21.1 --enable

Arguments:
  day   Three-letter day (Mon, Tue, Wed, Thu, Fri, Sat, Sun)
  part  Part number 1-6

Flags are all optional; only the provided fields are written. Times must be
on a 15-minute boundary. Setpoints are in Celsius.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		day := args[0]
		if !validDay(day) {
			return fmt.Errorf("day must be one of %v, got %q", scheduleDays, day)
		}
		part, err := strconv.Atoi(args[1])
		if err != nil || part < 1 || part > 6 {
			return fmt.Errorf("part must be integer 1-6, got %q", args[1])
		}

		if schedPartEnable && schedPartDisable {
			return errors.New("--enable and --disable are mutually exclusive")
		}

		prefix := fmt.Sprintf("sched%sPart%d", day, part)
		var fields []string

		if schedPartTime != "" {
			mins, err := hhmmToMinutes(schedPartTime)
			if err != nil {
				return err
			}
			fields = append(fields, fmt.Sprintf(`"%sTime":%d`, prefix, mins))
		}
		if schedPartLabel != "" {
			escaped := strings.ReplaceAll(schedPartLabel, `"`, `\"`)
			fields = append(fields, fmt.Sprintf(`"%sLabel":"%s"`, prefix, escaped))
		}
		if schedPartCool > 0 {
			fields = append(fields, fmt.Sprintf(`"%scsp":%g`, prefix, schedPartCool))
		}
		if schedPartHeat > 0 {
			fields = append(fields, fmt.Sprintf(`"%shsp":%g`, prefix, schedPartHeat))
		}
		if schedPartEnable {
			fields = append(fields, fmt.Sprintf(`"%sEnabled":true`, prefix))
		}
		if schedPartDisable {
			fields = append(fields, fmt.Sprintf(`"%sEnabled":false`, prefix))
		}

		if len(fields) == 0 {
			return errors.New("at least one of --time, --label, --cool, --heat, --enable, --disable must be provided")
		}

		payload := "{" + strings.Join(fields, ",") + "}"
		d := daikin.New(viper.GetString("email"), viper.GetString("password"))
		if err := d.UpdateDeviceRaw(deviceId, payload); err != nil {
			return fmt.Errorf("UpdateDeviceRaw(schedule) failed: %w", err)
		}

		printResult(map[string]interface{}{
			"action":  "schedule-set-part",
			"day":     day,
			"part":    part,
			"payload": payload,
		})
		return nil
	},
}

func validDay(day string) bool {
	for _, d := range scheduleDays {
		if d == day {
			return true
		}
	}
	return false
}

// Daikin stores times in 15-minute units; e.g., 06:00 = 24, 23:45 = 95.
func minutesToHHMM(units int) string {
	totalMins := units * 15
	return fmt.Sprintf("%02d:%02d", totalMins/60, totalMins%60)
}

func hhmmToMinutes(s string) (int, error) {
	parts := strings.Split(s, ":")
	if len(parts) != 2 {
		return 0, fmt.Errorf("invalid time %q: expected HH:MM", s)
	}
	h, err := strconv.Atoi(parts[0])
	if err != nil || h < 0 || h > 23 {
		return 0, fmt.Errorf("invalid hours in %q", s)
	}
	m, err := strconv.Atoi(parts[1])
	if err != nil || m < 0 || m > 59 {
		return 0, fmt.Errorf("invalid minutes in %q", s)
	}
	if m%15 != 0 {
		return 0, fmt.Errorf("minutes must be a 15-minute boundary (00, 15, 30, 45), got %d", m)
	}
	return h*4 + m/15, nil
}

func init() {
	deviceCmd.AddCommand(scheduleCmd)
	scheduleCmd.AddCommand(scheduleGetCmd)
	scheduleCmd.AddCommand(scheduleSetPartCmd)

	scheduleGetCmd.Flags().StringVarP(&deviceId, "device-id", "d", "", "Daikin device ID")
	scheduleGetCmd.MarkFlagRequired("device-id")

	scheduleSetPartCmd.Flags().StringVarP(&deviceId, "device-id", "d", "", "Daikin device ID")
	scheduleSetPartCmd.Flags().StringVar(&schedPartTime, "time", "", "Start time HH:MM (15-min boundary)")
	scheduleSetPartCmd.Flags().StringVar(&schedPartLabel, "label", "", "Label (e.g., wake, sleep, work)")
	scheduleSetPartCmd.Flags().Float32Var(&schedPartCool, "cool", 0, "Cool setpoint °C")
	scheduleSetPartCmd.Flags().Float32Var(&schedPartHeat, "heat", 0, "Heat setpoint °C")
	scheduleSetPartCmd.Flags().BoolVar(&schedPartEnable, "enable", false, "Enable this part")
	scheduleSetPartCmd.Flags().BoolVar(&schedPartDisable, "disable", false, "Disable this part")
	scheduleSetPartCmd.MarkFlagRequired("device-id")
}
