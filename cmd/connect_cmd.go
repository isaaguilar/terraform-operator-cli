package cmd

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"

	"github.com/galleybytes/terraform-operator-api/pkg/api"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	xterm "golang.org/x/term"
)

var connectCmd = &cobra.Command{
	Use:     "connect",
	Aliases: []string{"co"},
	Short:   "Authenticate with TFO API",
	Long:    "Adds or updates the token in the TFOCONFIG",
	// Args: cobra.MaximumNArgs(1),
	// Args: cobra.ExactArgs(1),
	PreRunE: func(cmd *cobra.Command, args []string) error {
		host = viper.GetString("host")
		if host == "" {
			return fmt.Errorf("`--host` is required")
		}
		return nil
	},
	Run: func(cmd *cobra.Command, args []string) {
		url := host + "/login"
		log.Println("Connecting to", url)
		connect(url)
	},
}

func init() {
	connectCmd.Flags().StringVarP(&host, "host", "", "", "what is foo?")
	viper.BindPFlag("host", connectCmd.Flags().Lookup("host"))
	rootCmd.AddCommand(connectCmd)
}

func connect(url string) {
	username := viper.GetString("username")
	if username == "" {
		fmt.Print("Login username: ")
		fmt.Scanln(&username)
	}
	var password []byte
	password = []byte(viper.GetString("password"))
	if len(password) == 0 {
		fmt.Print("Login password: ")
		p, err := xterm.ReadPassword(int(os.Stdin.Fd()))
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println()
		password = p
	}

	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	httpClient := http.Client{Transport: tr}

	d := struct {
		Username string `json:"user"`
		Password string `json:"password"`
	}{
		Username: username,
		Password: string(password),
	}
	b, err := json.Marshal(d)
	if err != nil {
		log.Fatal(err)
	}
	data := bytes.NewBuffer(b)

	resp, err := httpClient.Post(url, "application/json", data)
	if err != nil {
		log.Panic(err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		// Handle error
	}

	var respData api.Response
	err = json.Unmarshal(body, &respData)
	if err != nil {
		log.Fatal(err)
	}

	if respData.StatusInfo.StatusCode != 200 {
		fmt.Println(respData.StatusInfo.Message)
		os.Exit(1)
	}

	token := respData.Data.([]interface{})[0].(string)
	fmt.Println("Login succeeded")
	viper.Set("host", host)
	viper.Set("username", username)
	viper.Set("token", token)
	viper.WriteConfig()

}
