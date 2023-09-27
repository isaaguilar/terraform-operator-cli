package cmd

import (
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/atotto/clipboard"
	"github.com/eiannone/keyboard"
	"github.com/galleybytes/terraform-operator-api/pkg/api"
	"github.com/gorilla/websocket"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	xterm "golang.org/x/term"
)

var execCmd = &cobra.Command{
	Use:   "exec [-c client] <tf-resource-name>",
	Short: "Launch a debug session",
	Long:  "Create a debug pod via the API and interact via webtty",
	PreRunE: func(cmd *cobra.Command, args []string) error {
		viper.BindPFlag("host", cmd.Flags().Lookup("host"))
		host = viper.GetString("host")
		if host == "" {
			return fmt.Errorf("`--host` is required")
		}
		if clientName == "" {
			return fmt.Errorf("`--client` is required")
		}
		token = viper.GetString("token")
		if token == "" {
			return fmt.Errorf("No token was found. Try running `tfo connect`")
		}
		return nil
	},
	Args: cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) > 1 {
			command = args[1:]
		}
		TerminalWebsocket(args[0])
	},
}

func init() {
	execCmd.Flags().StringVarP(&host, "host", "", "", "Terraform-Operator API URL")
	execCmd.Flags().StringVarP(&clientName, "client", "c", "", "The client identifier")
	rootCmd.AddCommand(execCmd)
}

func TerminalWebsocket(name string) (isDone bool) {
	// Get the file descriptor of the terminal
	fd := int(os.Stdout.Fd())

	// Create a channel to receive terminal size changes
	sizeCh := make(chan ([2]int))
	columns, rows, _ := xterm.GetSize(fd)

	// Create a signal handler for SIGWINCH
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGWINCH)

	// Run a goroutine to listen for signals and send the new size to the channel
	go func() {
		sizeCh <- [2]int{columns, rows}
		for range sigCh {
			columns, rows, err := xterm.GetSize(fd)
			if err == nil {
				sizeCh <- [2]int{columns, rows}
			}
		}
	}()
	// isKeyboardSet := false

	isDone = true

	URL, err := url.Parse(host)
	if err != nil {
		log.Fatal("Invalid URL", err)
	}

	scheme := "ws"
	if URL.Scheme == "https" {
		scheme = "wss"
	}

	wsURL := fmt.Sprintf("%s://%s/api/v1/cluster/%s/debug/%s/%s", scheme, URL.Host, clientName, namespace, name)
	if len(command) > 0 {
		query := ""
		for _, i := range command {
			if query != "" {
				query += "&"
			}
			query += url.PathEscape(fmt.Sprintf("command=%s", i))
		}
		wsURL += fmt.Sprintf("?%s", query)
	}
	headers := http.Header{
		"Token": {token},
	}

	log.Printf("-Connection Info-\n")
	log.Printf("Host: %s\n", URL.Host)
	log.Printf("Client: %s\n", clientName)
	log.Printf("Namespace: %s\n", namespace)
	log.Printf("Name: %s\n", name)

	dialer := websocket.DefaultDialer
	dialer.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	conn, resp, err := dialer.Dial(wsURL, headers)
	if err != nil && resp != nil {

		defer resp.Body.Close()
		data, _ := io.ReadAll(resp.Body)
		var apiResponse api.Response
		err = json.Unmarshal(data, &apiResponse)
		if err != nil {
			log.Fatal(err)
		}
		log.Fatal(apiResponse.StatusInfo.Message)

	} else if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	// Create a channel for interrupt signal
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt, syscall.SIGTERM)

	// Create a channel for closer signal
	closer := make(chan error)

	// Start a goroutine to read messages from the WebSocket connection
	go func() {
		defer close(closer)
		for {
			// Read a message
			mt, bmsg, err := conn.ReadMessage()
			if err != nil {
				if strings.Contains(err.Error(), "close 1000") {
					closer <- nil
				}
				closer <- err
				return
			}
			switch mt {
			case websocket.TextMessage:
				msg := string(bmsg)
				if strings.HasPrefix(msg, "2") {
					// log.Println("PONG!")
					continue
				}
				if strings.HasPrefix(msg, "3") {
					continue
				}
				if strings.HasPrefix(msg, "6") {
					continue
				}
				dec, err := base64.StdEncoding.DecodeString(string(msg[1:]))
				if err != nil {
					log.Println(err)
					continue
				}
				fmt.Printf("%s", string(dec))
			case websocket.CloseMessage:
				return
			default:
				fmt.Printf("The MessageType: %+v\n", mt)
				// Print the message to the standard output
				fmt.Printf("Received: %s\n", bmsg)
				return
			}

		}
	}()

	keyEvent, err := keyboard.GetKeys(128)
	if err != nil {
		log.Panic(err)
	}
	defer func() {
		_ = keyboard.Close()
	}()

	var ctrlCExit bool
	for {

		select {
		case size := <-sizeCh:

			sizeJSON := fmt.Sprintf(`{"Columns": %d, "Rows": %d}`, size[0], size[1])
			encodedSizeJSON := base64.StdEncoding.EncodeToString([]byte(sizeJSON))
			// log.Println("Will set size", sizeJSON)
			err := conn.WriteMessage(websocket.TextMessage, append([]byte{byte('3')}, []byte(encodedSizeJSON)...))
			// err := conn.WriteMessage(websocket.TextMessage, []byte{byte('2')})
			if err != nil {
				log.Println("there was some error:", err)
			}
			continue

		case err := <-closer:
			if err != nil {
				log.Fatal(err)
			}
			return
		case <-interrupt:
			// Send a close message to the WebSocket server
			log.Println("interrupt")
			err := conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
			if err != nil {
				log.Println("write close:", err)
				return
			}
			// Wait for the server to close the connection
			select {
			case <-closer:
			case <-time.After(time.Second):
			}
			return

		case ev := <-keyEvent:

			// fmt.Printf("`")

			if ctrlCExit {
				fmt.Fprintf(os.Stderr, "\nPress ctrl-c again to exit session\n")
			}

			char, key, err := ev.Rune, ev.Key, ev.Err
			if err != nil {
				panic(err)
			}

			if key == keyboard.KeyCtrlC {
				if ctrlCExit {
					err := conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
					if err != nil {
						log.Println("write close:", err)
						return
					}
				}
				ctrlCExit = false // Set to true - useful for debugging
			} else {
				ctrlCExit = false
			}

			// fmt.Printf("%c", char)

			var byteArr []byte
			if key != 0 {
				// TODO Arrow keys and special keys might depend on the "term" type. For example,
				// arrows do not work properly in a "screen" term. Fix special characters based
				// on the term type.
				switch key {
				case keyboard.KeyArrowUp:
					byteArr = []byte{27, 91, 65}
				case keyboard.KeyArrowDown:
					byteArr = []byte{27, 91, 66}
				case keyboard.KeyArrowLeft:
					byteArr = []byte{27, 91, 68}
				case keyboard.KeyArrowRight:
					byteArr = []byte{27, 91, 67}
				case keyboard.KeyEsc:
					if char > 0 {
						data, err := clipboard.ReadAll()
						if err == nil {
							byteArr = []byte(data)
						} else {
							byteArr = []byte{byte(key)}
						}

					} else {
						byteArr = []byte{byte(key)}
					}
				default:
					byteArr = []byte{byte(key)}
				}
				// log.Println("I pressed", char, "key", key, "byte", byteArr, "string", string(byteArr))

			} else {
				byteArr = []byte{byte(char)}
			}

			// log.Println("I pressed", char, "key", key, "byte", byteArr, "string", string(byteArr))
			// fmt.Printf("%s", string(byteArr))
			encodedText := base64.StdEncoding.EncodeToString(byteArr)

			if key == keyboard.KeyHome {
				log.Println("I pressed the home key")
				err := conn.WriteMessage(websocket.TextMessage, []byte{byte('2')})
				if err != nil {
					log.Println(err)
				}
				continue
			}

			wstext := "1" + encodedText
			_ = wstext
			// Write a text message to the WebSocket connection
			erre := conn.WriteMessage(websocket.TextMessage, []byte(wstext))
			if erre != nil {
				log.Println("write:", erre)
				return
			}

		}
	}

}
