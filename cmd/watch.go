/*
Copyright © 2024 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"log/slog"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// watchCmd represents the watch command
var watchCmd = &cobra.Command{
	Use:   "watch",
	Short: "",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		for {
			frequency := viper.GetDuration("watch.frequency")
			if frequency == 0 {
				frequency = time.Hour
			}
			err := runBackup()
			if err != nil {
				slog.Error("Backup failed", "err", err)
			}

			timeToNextRun := frequency - time.Duration(time.Now().Unix()%int64(frequency/time.Second))*time.Second
			if timeToNextRun == 0 {
				timeToNextRun += frequency
			}
			slog.Info("Next backup", "next", time.Now().Add(timeToNextRun).Truncate(time.Second))
			time.Sleep(timeToNextRun)
		}
	},
}

func init() {
	rootCmd.AddCommand(watchCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// watchCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// watchCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
