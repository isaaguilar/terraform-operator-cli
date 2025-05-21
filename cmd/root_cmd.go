package cmd

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"

	infrakube "github.com/galleybytes/infrakube/pkg/client/clientset/versioned"
	"github.com/ghodss/yaml"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/plugin/pkg/client/auth/oidc"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

var (
	rootCmd = &cobra.Command{
		Use:     "ik",
		Aliases: []string{"\"kubectl tf(o)\""},
		Short:   "Terraform Operator (ik) CLI -- Manage TFO deployments",
		Args:    cobra.MaximumNArgs(0),
	}
)

var (
	// vars used for flags
	host                string
	kubeconfig          string
	namespace           string
	infrakubeConfigFile string
	clientName          string
	token               string
	username            string
	command             []string

	session Session

	// show only flags
	allNamespaces bool
)

func init() {
	cobra.OnInitialize(newSession)
	getKubeconfig()
	rootCmd.PersistentFlags().StringVar(&infrakubeConfigFile, "config", "", "absolute path to config file (default is $HOME/.ik/config)")
	viper.BindPFlag("config", rootCmd.PersistentFlags().Lookup("config"))
	rootCmd.PersistentFlags().StringVarP(&namespace, "namespace", "n", "", "namespace to add Kubernetes creds secret")

	rootCmd.CompletionOptions.DisableDefaultCmd = true
}

type Session struct {
	config             *rest.Config
	infrakubeclientset infrakube.Interface
	clientset          kubernetes.Interface
	namespace          string
}

func newSession() {

	viper.SetEnvPrefix("TFO")
	viper.AutomaticEnv()

	// Load from global config first, will be ignored if does not exist
	viper.SetConfigName("global")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("$HOME/.ik")
	if err := viper.ReadInConfig(); err == nil {
		if host == "" {
			host = viper.GetString("host")
		}
	}

	viper.SetConfigType("yaml")
	infrakubeConfigFile = viper.GetString("config")

	if infrakubeConfigFile != "" {
		viper.SetConfigFile(infrakubeConfigFile)
		infrakubeConfigFileType := filepath.Ext(infrakubeConfigFile)
		log.Println(infrakubeConfigFile, infrakubeConfigFileType)
		if infrakubeConfigFileType == "" {
			viper.SetConfigFile(infrakubeConfigFileType)
		}
	} else {
		// From config, get any values to use as a default or fallback
		viper.SetConfigName("config")
		viper.SetConfigType("yaml")
		viper.AddConfigPath("$HOME/.ik")
		if home := homedir.HomeDir(); home != "" {
			infrakubeConfigFile = filepath.Join(home, ".ik", "config")
		}
	}

	if readErr := viper.ReadInConfig(); readErr != nil {
		err := createConfigFile(infrakubeConfigFile)
		if err != nil {
			log.Print(err)
			os.Exit(0)
		}
		if readErr := viper.ReadInConfig(); readErr != nil {
			log.Fatal(err)
		}
	}

	// Config file found and successfully parsed

	// Get the namespace from the user's contexts when not passed in via flag
	kubeConfig, _ := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if kubeConfig != nil && namespace == "" {
		// Define the schema of the kubeconfig that is meaningful to extract
		// the current-context's namespace (if defined)
		type kubecfgContext struct {
			Namespace string `json:"namespace"`
		}
		type kubecfgContexts struct {
			Name    string         `json:"name"`
			Context kubecfgContext `json:"context"`
		}
		type kubecfg struct {
			CurrentContext string            `json:"current-context"`
			Contexts       []kubecfgContexts `json:"contexts"`
		}
		// As a kubectl plugin, it's nearly guaranteed a kubeconfig is used
		b, err := ioutil.ReadFile(kubeconfig)
		if err != nil {
			panic(err)
		}
		kubecfgCtx := kubecfg{}
		yaml.Unmarshal(b, &kubecfgCtx)
		for _, item := range kubecfgCtx.Contexts {
			if item.Name == kubecfgCtx.CurrentContext {
				namespace = item.Context.Namespace
				break
			}
		}
		if namespace == "" {
			namespace = "default"
		}
	}

	if kubeConfig != nil {

		clientset, err := kubernetes.NewForConfig(kubeConfig)
		if err != nil {
			panic(err)
		}

		infrakubeclientset, err := infrakube.NewForConfig(kubeConfig)
		if err != nil {
			panic(err)
		}
		session.clientset = clientset
		session.infrakubeclientset = infrakubeclientset
		session.namespace = namespace
		session.config = kubeConfig
	}

}

func getKubeconfig() {
	if home := homedir.HomeDir(); home != "" {
		rootCmd.PersistentFlags().StringVarP(&kubeconfig, "kubecfg", "", "", "(optional) absolute path to the kubeconfig file")
		if kubeconfig == "" {
			kubeconfig = os.Getenv("KUBECONFIG")
			if kubeconfig == "" {
				kubeconfig = filepath.Join(home, ".kube", "config")
			}
		}
	} else {
		rootCmd.PersistentFlags().StringVarP(&kubeconfig, "kubecfg", "", "", "absolute path to the kubeconfig file")
		if kubeconfig == "" {
			kubeconfig = os.Getenv("KUBECONFIG")
		}
	}

}

func createConfigFile(name string) error {
	if name == "" {
		return fmt.Errorf("no config file defined")
	}
	fileInfo, err := os.Stat(name)

	if err != nil {
		createCh := make(chan bool)
		go func() {
			for {
				var stringbool string
				fmt.Printf("Do you want to create '%s' (Y/n): ", name)
				fmt.Scanln(&stringbool)

				if strings.HasPrefix(strings.ToLower(stringbool), "y") {
					createCh <- true
					return
				}
				if strings.HasPrefix(strings.ToLower(stringbool), "n") {
					createCh <- false
					return
				}

			}
		}()

		create := <-createCh
		if !create {
			return fmt.Errorf("select a config file with `--config`")
		}
		err = os.MkdirAll(filepath.Dir(name), 0755)
		if err != nil {
			return fmt.Errorf("failed to create dir: %s", err)
		}
		_, err = os.Create(name)
		return err

	}

	if fileInfo.IsDir() {
		return fmt.Errorf("config file %s expected a file but is a dir", name)
	}
	return nil

}

func Execute(v string) error {
	version = v
	return rootCmd.Execute()
}
