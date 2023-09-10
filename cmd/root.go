/* SPDX-License-Identifier: MIT */
package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/afiestas/gluetun-sync/lib"
	"github.com/fatih/color"
	"github.com/fsnotify/fsnotify"
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

func updatePort(port uint16) error {
	updateCh, quitCh := lib.PrintRequester()
	err := requester.SendRequests(port, config.Requests, updateCh)
	<-quitCh

	return err
}

func watchAndSync() {
	lib.Info(fmt.Sprintf("Monitoring: %s ", config.PortFile))
	portCh, quit, err := lib.PortChangeNotifier(config.PortFile, 1000)
	if err != nil {
		fmt.Println("❌")
		lib.PrintError(fmt.Errorf("watchg file err %w", err))
		return
	}

	fmt.Println("✅")
	var timer *time.Timer
	for {
		select {
		case port, ok := <-portCh:
			if !ok {
				continue
			}
			if timer != nil {
				timer.Stop()
				timer = nil
			}
			fmt.Printf("Detected port %d\n", port)
			err := updatePort(port)
			if err != nil {
				lib.Info("Retrying every 10 seconds\n")
				timer = time.AfterFunc(time.Second*2, func() {
					err := updatePort(port)
					if err != nil {
						fmt.Println("Error happened, reseting timer")
						timer.Reset(time.Second * 2)
						return
					}
					timer.Stop()
					timer = nil
				})
			}
		case <-quit:
			return
		}
	}
}

func once() {
	lib.Info("Synchronizing port once")
	port, err := lib.GetPortFromFile(config.PortFile)
	fmt.Printf("Detected port %d\n", port)
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

func setupViper() {
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
}

func unmarshalAndValidate() (lib.Configuration, error) {
	c := lib.Configuration{}
	err := viper.UnmarshalExact(&c)
	if err != nil {
		return c, err
	}

	validate := validator.New(validator.WithRequiredStructEnabled())
	err = validate.Struct(c)
	if err != nil {
		return c, err
	}

	return c, nil
}

func printConfigError(err error) {
	if err == nil {
		return
	}

	if err != nil {
		if _, ok := err.(*validator.InvalidValidationError); ok {
			lib.PrintStepError(err)
		}

		errs := err.(validator.ValidationErrors)
		for _, e := range errs {
			lib.PrintStepError(e)
		}
	}
}

func initConfig() {
	setupViper()
	lib.Info(fmt.Sprintf("Configuration: %s ", viper.ConfigFileUsed()))

	var err error
	config, err = unmarshalAndValidate()
	if err != nil {
		fmt.Println("❌")
		printConfigError(err)
		os.Exit(1)
	}
	fmt.Println("✅")

	var timer *time.Timer
	viper.OnConfigChange(func(e fsnotify.Event) {
		if timer != nil {
			timer.Reset(time.Second * 1)
			return
		}

		timer = time.AfterFunc(time.Second*1, func() {
			timer.Stop()
			timer = nil

			lib.Info(fmt.Sprintf("Config file changed: %s ", e.Name))
			newConfig, err := unmarshalAndValidate()
			if err != nil {
				fmt.Println("❌")
				fmt.Println("can't parse new configuration")
				printConfigError(err)
				return
			}
			fmt.Println("✅")
			lib.Info("Updating configuration\n")
			config = newConfig
		})
	})
	viper.WatchConfig()

	if config.ForceColor {
		color.NoColor = !config.ForceColor
	}
}
