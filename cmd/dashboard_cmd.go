package cmd

import (
	"fmt"
	"log"

	"github.com/skratchdot/open-golang/open"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var dashboardCmd = &cobra.Command{
	Use:     "dashboard",
	Aliases: []string{"co"},
	Short:   "Login to the dashboard with token",
	Long:    "Using 'ik dashboard' automatically authenticates the cli by first calling 'connect'",
	PreRunE: func(cmd *cobra.Command, args []string) error {
		viper.BindPFlag("host", cmd.Flags().Lookup("host"))
		host = viper.GetString("host")
		if host == "" {
			return fmt.Errorf("`--host` is required")
		}
		if viper.ConfigFileUsed() == "" {
			return fmt.Errorf("config file not defined")
		}
		return nil
	},
	Run: func(cmd *cobra.Command, args []string) {
		log.Println("Connecting to", host)
		connect(host)
		log.Println("Loading dashboard")
		token = viper.GetString("token")
		dashboard(token)
	},
}

func init() {
	dashboardCmd.Flags().StringVarP(&host, "host", "H", "", "Terraform-Operator API URL")
	dashboardCmd.Flags().StringVarP(&username, "username", "U", "", "Username of the API")
	viper.BindPFlag("username", dashboardCmd.Flags().Lookup("username"))
	rootCmd.AddCommand(dashboardCmd)
}

func dashboard(token string) {
	err := open.Start(host + "/dashboard?token=" + token)
	if err != nil {
		log.Fatal(err)
	}
}
