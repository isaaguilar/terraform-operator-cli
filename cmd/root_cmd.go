package cmd

import (
	"io/ioutil"
	"log"
	"os"
	"path/filepath"

	tfo "github.com/galleybytes/terraform-operator/pkg/client/clientset/versioned"
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
		Use:     "tfo",
		Aliases: []string{"\"kubectl tf(o)\""},
		Short:   "Terraform Operator (tfo) CLI -- Manage TFO deployments",
		Args:    cobra.MaximumNArgs(0),
	}
)

var (
	// vars used for flags
	host          string
	kubeconfig    string
	namespace     string
	tfoConfigFile string
	clientName    string
	token         string

	session Session

	// show only flags
	allNamespaces bool
)

func init() {
	cobra.OnInitialize(newSession)
	getKubeconfig()
	rootCmd.PersistentFlags().StringVar(&tfoConfigFile, "config", "", "absolute path to config file (default is $HOME/.tfo/config)")
	viper.BindPFlag("config", rootCmd.PersistentFlags().Lookup("config"))
	rootCmd.PersistentFlags().StringVarP(&namespace, "namespace", "n", "", "namespace to add Kubernetes creds secret")

	rootCmd.CompletionOptions.DisableDefaultCmd = true
}

type Session struct {
	config       *rest.Config
	tfoclientset tfo.Interface
	clientset    kubernetes.Interface
	namespace    string
}

func newSession() {
	kubeConfig, _ := clientcmd.BuildConfigFromFlags("", kubeconfig)
	// if err != nil {
	// 	panic(err)
	// }

	viper.SetEnvPrefix("TFO")
	viper.AutomaticEnv()

	// Load from global config first, will be ignored if does not exist
	viper.SetConfigName("global")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("$HOME/.tfo")
	if err := viper.ReadInConfig(); err == nil {
		host = viper.GetString("host")
	}

	viper.SetConfigType("yaml")
	tfoConfigFile = viper.GetString("config")

	if tfoConfigFile != "" {
		viper.SetConfigFile(tfoConfigFile)
		tfoConfigFileType := filepath.Ext(tfoConfigFile)
		log.Println(tfoConfigFile, tfoConfigFileType)
		if tfoConfigFileType == "" {
			viper.SetConfigFile(tfoConfigFileType)
		}
	} else {
		// From config, get any values to use as a default or fallback
		viper.SetConfigName("config")
		viper.SetConfigType("yaml")
		viper.AddConfigPath("$HOME/.tfo")
	}

	if err := viper.ReadInConfig(); err != nil {
		log.Println(err)
	}

	// Config file found and successfully parsed

	// Get the namespace from the user's contexts when not passed in via flag
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

		tfoclientset, err := tfo.NewForConfig(kubeConfig)
		if err != nil {
			panic(err)
		}
		session.clientset = clientset
		session.tfoclientset = tfoclientset
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

func Execute(v string) error {
	version = v
	return rootCmd.Execute()
}
