/*
Copyright © 2020 NAME HERE <EMAIL ADDRESS>

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package cmd

import (
	"log"

	"github.com/abibby/backup/backend"
	"github.com/abibby/backup/backup"
	"github.com/abibby/backup/database"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// backupCmd represents the backup command
var backupCmd = &cobra.Command{
	Use:   "backup",
	Short: "Initiate a backup to the backup server",
	Long:  ``,
	RunE: func(cmd *cobra.Command, args []string) error {
		dir := viper.GetString("dir")
		log.Printf("Backing up directory %s", dir)

		db, err := database.Open(viper.GetString("database"))
		if err != nil {
			return errors.Wrap(err, "failed to initialize database")
		}
		backends, err := getBackends()
		if err != nil {
			return err
		}
		for _, b := range backends {
			if b, ok := b.(backend.Closer); ok {
				defer b.Close()
			}
		}
		return backup.Backup(db, dir, &backup.Options{
			Ignore:   viper.GetStringSlice("ignore"),
			Backends: backends,
			Logger:   logger,
		})
	},
}

func init() {
	rootCmd.AddCommand(backupCmd)

	viper.SetDefault("ignore", []string{})
	viper.SetDefault("backends", []string{})
}
