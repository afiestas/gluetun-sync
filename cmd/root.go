/* SPDX-License-Identifier: MIT */
package cmd

import (
	"fmt"
	"os"

	"github.com/afiestas/gluetun-sync/lib"
	"github.com/fatih/color"
	"github.com/go-playground/validator/v10"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	cfgFile   string
	config    lib.Configuration
	requester lib.Requester = lib.NewRequester()
)

var rootCmd = &cobra.Command{
	Use:   "gluetun-sync",
	Short: "Syncs via http requests port changes from gluetun",
	Long: `Monitors the gluetun port file and issues a serie of
configured http requests whenever it changes to keep other
applications in sync.

For example you can change the listening port (or the mapping) for
any self-hosted software such as video game servers, nextcloud, etc.`,
	Run: func(cmd *cobra.Command, args []string) {
		if config.Once {
			once()
			return
		}

		watchAndSync()
	},
}

func updatePort(port uint16) {
	fmt.Printf("\n[INFO] Detected port %d\n", port)
	updateCh, quitCh := lib.PrintRequester()
	requester.SendRequests(port, config.Requests, updateCh)
	<-quitCh
}

func watchAndSync() {
	lib.Info(fmt.Sprintf("Monitoring: %s", config.PortFile))
	portCh, quit, err := lib.PortChangeNotifier(config.PortFile, 1000)
	if err != nil {
		lib.PrintError(fmt.Errorf("watchg file err %w", err))
		return
	}

	for {
		select {
		case port, ok := <-portCh:
			if !ok {
				continue
			}
			updatePort(port)
		case <-quit:
			return
		}
	}
}

func once() {
	lib.Info("Synchronizing port once")

	port, err := lib.GetPortFromFile(config.PortFile)
	if err != nil {
		lib.PrintError(fmt.Errorf("error while reading port from file %w", err))
		return
	}
	updatePort(port)
}

func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	pFlags := rootCmd.PersistentFlags()
	pFlags.StringVar(&cfgFile, "config", "", "config file (default is /etc/gluetun-sync/config.toml).")
	pFlags.String("port-file", "/tmp/portfile", "The path to where the gluetun port file is")
	pFlags.BoolP("force-color", "f", false, "Forces color output")
	pFlags.BoolP("once", "1", false, "Tries to synchronize just once")

	viper.BindPFlags(pFlags)
}

func initConfig() {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		viper.SetConfigName("config")
		viper.AddConfigPath("/etc/gluetun-sync")
		viper.AddConfigPath(".")
	}
	viper.SetEnvPrefix("GLUESYNC")
	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err != nil {
		lib.PrintStepError(fmt.Errorf("couldn't parse config file %w", err))
		os.Exit(1)
	}

	fmt.Println("⚙️ Using configuration")
	fmt.Printf("  └─ %s: ", viper.ConfigFileUsed())
	err := viper.UnmarshalExact(&config)
	if err != nil {
		lib.PrintError(fmt.Errorf("error marshaling config %w", err))
		os.Exit(1)
	}

	if config.ForceColor {
		color.NoColor = !config.ForceColor
	}

	validate := validator.New(validator.WithRequiredStructEnabled())
	err = validate.Struct(config)

	if err != nil {
		fmt.Println("❌")
		if _, ok := err.(*validator.InvalidValidationError); ok {
			lib.PrintStepError(err)
			os.Exit(1)
		}

		errs := err.(validator.ValidationErrors)
		for _, e := range errs {
			lib.PrintStepError(e)
		}
		os.Exit(1)
	}
	fmt.Println("✅")
}
