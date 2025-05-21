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
	"time"

	"github.com/galleybytes/infrakube-stella/pkg/api"
	"github.com/gin-gonic/gin"
	"github.com/skratchdot/open-golang/open"
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
	},
}

func init() {
	connectCmd.Flags().StringVarP(&host, "host", "H", "", "Terraform-Operator API URL")
	connectCmd.Flags().StringVarP(&username, "username", "U", "", "Username of the API")
	viper.BindPFlag("username", connectCmd.Flags().Lookup("username"))
	rootCmd.AddCommand(connectCmd)
}

func connect(host string) {
	var token string

	connecter, err := getConnecter(host)
	if err != nil {
		log.Fatal(err)
	}
	if connecter == "sso" {
		token, err = ssoConnecter(host)
		if err != nil {
			log.Fatal(err)
		}
	} else {
		token, err = loginConnecter(host)
		if err != nil {
			log.Fatal(err)
		}
	}
	fmt.Println("Login succeeded")
	viper.Set("host", host)
	viper.Set("username", username)
	viper.Set("token", token)
	viper.WriteConfig()

}

func getConnecter(host string) (string, error) {
	url := host + "/connect"
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	httpClient := http.Client{Transport: tr}

	resp, err := httpClient.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var respData api.Response
	err = json.Unmarshal(body, &respData)
	if err != nil {
		return "", fmt.Errorf("error parsing %s: %s", url, string(body))
	}

	if respData.StatusInfo.StatusCode != 200 {
		return "", fmt.Errorf(respData.StatusInfo.Message)
	}

	return respData.Data.([]interface{})[0].(string), nil
}

func loginConnecter(host string) (string, error) {
	url := host + "/login"
	username = viper.GetString("username")
	if username == "" {
		fmt.Print("Login username: ")
		fmt.Scanln(&username)
	} else {
		fmt.Printf("(Username %s)\n", username)
	}
	var password []byte
	password = []byte(viper.GetString("password")) // Not a very smart place to put a password...
	if len(password) == 0 {
		fmt.Print("Login password: ")

		p, err := xterm.ReadPassword(int(os.Stdin.Fd()))
		if err != nil {
			return "", err
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
		return "", err
	}
	data := bytes.NewBuffer(b)

	resp, err := httpClient.Post(url, "application/json", data)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var respData api.Response
	err = json.Unmarshal(body, &respData)
	if err != nil {
		return "", err
	}

	if respData.StatusInfo.StatusCode != 200 {
		return "", fmt.Errorf(respData.StatusInfo.Message)

	}

	token := respData.Data.([]interface{})[0].(string)
	return token, nil
}

func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Set CORS headers
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "*")

		// Check if the request method is OPTIONS
		if c.Request.Method == "OPTIONS" {
			// If so, abort with a 204 status code
			c.AbortWithStatus(204)
			return
		}

		// Continue processing the request
		c.Next()
	}
}

func ssoConnecter(host string) (string, error) {

	gin.SetMode(gin.ReleaseMode)
	gin.DefaultWriter = io.Discard // Shut up!
	router := gin.Default()

	router.Use(corsMiddleware())

	stopCh := make(chan bool)
	tokenCh := make(chan string)
	errorCh := make(chan error)

	router.Any("/connecter", func(c *gin.Context) {
		token := c.Query("token")
		defer func() {
			stopCh <- true
			tokenCh <- token
		}()
		if token == "" {
			c.AbortWithError(http.StatusNotFound, fmt.Errorf("failed to get token"))
			return
		}
		fmt.Fprintln(c.Writer, " success!") // Response to send to caller
	})

	go func() {
		errorCh <- router.Run(":18080")
	}()

	// Once the server has started, connect to the SSO Identity Provider (IDP)
	err := open.Start(host + "/sso")
	if err != nil {
		return "", err
	}

	select {
	case err := <-errorCh:
		return "", err
	case <-stopCh:
		token := <-tokenCh
		time.Sleep(100 * time.Millisecond) // Time for response to send
		if token == "" {
			return "", fmt.Errorf("the connecter did not receive a token")
		}
		return token, nil
	}
}
