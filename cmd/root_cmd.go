package cmd

import (
	"io/ioutil"
	"os"
	"path/filepath"

	tfo "github.com/galleybytes/terraform-operator/pkg/client/clientset/versioned"
	"github.com/ghodss/yaml"
	"github.com/spf13/cobra"
	"k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/plugin/pkg/client/auth/oidc"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

func Execute(v string) error {
	version = v
	return rootCmd.Execute()
}

type Session struct {
	config       *rest.Config
	tfoclientset tfo.Interface
	clientset    kubernetes.Interface
	namespace    string
}

func newSession() {
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		panic(err)
	}

	// Get the namespace from the user's contexts when not passed in via flag
	if namespace == "" {
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

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err)
	}

	tfoclientset, err := tfo.NewForConfig(config)
	if err != nil {
		panic(err)
	}

	session.clientset = clientset
	session.tfoclientset = tfoclientset
	session.namespace = namespace
	session.config = config
}

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
	session    Session
	kubeconfig string
	namespace  string

	// show only flags
	allNamespaces bool
)

func init() {
	cobra.OnInitialize(newSession)
	getKubeconfig()
	rootCmd.PersistentFlags().StringVarP(&namespace, "namespace", "n", "", "namespace to add Kubernetes creds secret")
	rootCmd.CompletionOptions.DisableDefaultCmd = true
}

func getKubeconfig() {
	if home := homedir.HomeDir(); home != "" {
		rootCmd.PersistentFlags().StringVarP(&kubeconfig, "kubecfg", "c", "", "(optional) absolute path to the kubeconfig file")
		if kubeconfig == "" {
			kubeconfig = os.Getenv("KUBECONFIG")
			if kubeconfig == "" {
				kubeconfig = filepath.Join(home, ".kube", "config")
			}
		}
	} else {
		rootCmd.PersistentFlags().StringVarP(&kubeconfig, "kubecfg", "c", "", "absolute path to the kubeconfig file")
		if kubeconfig == "" {
			kubeconfig = os.Getenv("KUBECONFIG")
		}
	}

}
