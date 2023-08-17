package cmd

import (
	"context"
	"fmt"
	"log"
	"os"

	tfv1beta1 "github.com/galleybytes/terraform-operator/pkg/apis/tf/v1beta1"
	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var showCmd = &cobra.Command{
	Use:   "show",
	Short: "Show a comprehensive list of tfo related resources",
	Args:  cobra.MaximumNArgs(0),
	Run: func(cmd *cobra.Command, args []string) {
		show("name", allNamespaces, false)
	},
}

func init() {
	showCmd.Flags().BoolVarP(&allNamespaces, "all-namespaces", "A", false, "Show tfo resources for all namespaces")
	rootCmd.AddCommand(showCmd)
}

func show(name string, allNamespaces, showPrevious bool) {
	var data [][]string
	var header []string
	var namespaces []string
	var tfs []tfv1beta1.Terraform
	var pods []corev1.Pod

	if allNamespaces {
		header = []string{"Namespace", "Name", "Generation", "Pods"}
		namespaceClient := session.clientset.CoreV1().Namespaces()
		namespaceList, err := namespaceClient.List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			log.Fatal(err)
		}
		for _, namespace := range namespaceList.Items {
			namespaces = append(namespaces, namespace.Name)
		}

		tfClient := session.tfoclientset.TfV1beta1().Terraforms("")
		tfList, err := tfClient.List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			log.Fatal(err)
		}
		tfs = tfList.Items

		podClient := session.clientset.CoreV1().Pods("")
		podList, err := podClient.List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			log.Fatal(err)
		}
		pods = podList.Items
	} else {
		header = []string{"Name", "Generation", "Pods"}
		namespaces = []string{session.namespace}

		tfClient := session.tfoclientset.TfV1beta1().Terraforms(session.namespace)
		tfList, err := tfClient.List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			log.Fatal(err)
		}
		tfs = tfList.Items

		podClient := session.clientset.CoreV1().Pods(session.namespace)
		podList, err := podClient.List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			log.Fatal(err)
		}
		pods = podList.Items
	}

	for _, namespace := range namespaces {

		var namespacedTfs []tfv1beta1.Terraform
		for _, tf := range tfs {
			if tf.Namespace == namespace {
				namespacedTfs = append(namespacedTfs, tf)
			}
		}

		var namespacedPods []corev1.Pod
		for _, pod := range pods {
			if pod.Namespace == namespace {
				namespacedPods = append(namespacedPods, pod)
			}
		}

		for _, tf := range namespacedTfs {
			data_index := len(data)
			generation := fmt.Sprintf("%d", tf.Generation)

			podsIndex := 2
			previousPodsIndex := 3

			if allNamespaces {
				podsIndex = podsIndex + 1
				previousPodsIndex = previousPodsIndex + 1
				data = append(data, []string{namespace, tf.Name, generation, "", ""})
			} else {
				data = append(data, []string{tf.Name, generation, "", ""})
			}

			var currentRunnerEntryIndex int
			var previousRunnersEntryIndex int
			for _, pod := range namespacedPods {
				if pod.Labels["terraforms.tf.isaaguilar.com/generation"] == generation {
					if len(data) == data_index+currentRunnerEntryIndex {
						if allNamespaces {
							data = append(data, []string{"", "", "", pod.Name, ""})
						} else {
							data = append(data, []string{"", "", pod.Name, ""})
						}

					} else {
						data[data_index+currentRunnerEntryIndex][podsIndex] = pod.Name
					}

					currentRunnerEntryIndex++
				} else if showPrevious {
					if len(data) == data_index+previousRunnersEntryIndex {
						if allNamespaces {
							data = append(data, []string{"", "", "", "", pod.Name})
						} else {
							data = append(data, []string{"", "", "", pod.Name})
						}
					} else {
						data[data_index+previousRunnersEntryIndex][previousPodsIndex] = pod.Name
					}

					previousRunnersEntryIndex++
				}
			}
		}
	}

	if showPrevious {
		header = append(header, "PreviousPods")
	}
	table := tablewriter.NewWriter(os.Stdout)
	table.SetAutoWrapText(false)
	table.SetAutoFormatHeaders(true)
	table.SetHeaderAlignment(tablewriter.ALIGN_LEFT)
	table.SetAlignment(tablewriter.ALIGN_LEFT)
	table.SetCenterSeparator("")
	table.SetColumnSeparator("")
	table.SetRowSeparator("")
	table.SetHeaderLine(false)
	table.SetBorder(false)
	table.SetTablePadding("\t") // pad with tabs
	table.SetNoWhiteSpace(true)
	table.SetHeader(header)
	table.AppendBulk(data)
	table.Render()

}
