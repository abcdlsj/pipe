package server

import (
	"github.com/spf13/cobra"
)

func Command() *cobra.Command {
	var (
		cfgFile string
		flagCfg Config
	)

	cmd := &cobra.Command{
		Use: "server",
		Run: func(cmd *cobra.Command, args []string) {
			if cfgFile != "" {
				newServer(parseConfig(cfgFile)).Run()
			} else {
				newServer(flagCfg).Run()
			}
		},
	}
	cmd.PersistentFlags().IntVarP(&flagCfg.Port, "port", "p", 8910, "server port")
	cmd.PersistentFlags().IntVarP(&flagCfg.AdminPort, "admin-port", "a", 0, "admin server port")
	cmd.PersistentFlags().StringVarP(&cfgFile, "config", "c", "", "config file")

	return cmd
}