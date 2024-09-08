package cmd

import (
	"os"

	"github.com/abibby/backup/backend"
	"github.com/spf13/viper"
)

func getBackends() ([]backend.Backend, error) {
	backendUris := viper.GetStringSlice("backends")
	backends := []backend.Backend{}
	for _, uri := range backendUris {
		b, err := backend.Load(os.ExpandEnv(uri))
		if err != nil {
			return nil, err
		}
		backends = append(backends, b)
	}
	return backends, nil
}
