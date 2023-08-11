package cmd

import (
	"context"
	"fmt"
	"log"
	"os"

	tfv1beta1 "github.com/galleybytes/terraform-operator/pkg/apis/tf/v1beta1"
	"golang.org/x/sys/unix"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/cli-runtime/pkg/genericclioptions"

	// "k8s.io/cli-runtime/pkg/genericclioptions"

	"k8s.io/client-go/tools/remotecommand"

	// "k8s.io/kubectl/pkg/cmd/exec"
	"k8s.io/kubectl/pkg/cmd/exec"
	"k8s.io/kubectl/pkg/scheme"
)

func generatePod(tf *tfv1beta1.Terraform) *corev1.Pod {
	terraformVersion := tf.Spec.TerraformVersion
	if terraformVersion == "" {
		terraformVersion = "1.1.5"
	}
	generation := fmt.Sprint(tf.Generation)
	versionedName := tf.Status.PodNamePrefix + "-v" + generation
	generateName := versionedName + "-debug-"
	generationPath := "/home/tfo-runner/generations/" + generation
	env := []corev1.EnvVar{}
	envFrom := []corev1.EnvFromSource{}
	annotations := make(map[string]string)
	labels := make(map[string]string)
	for _, taskOption := range tf.Spec.TaskOptions {
		if tfv1beta1.ListContainsTask(taskOption.For, "*") {
			env = append(env, taskOption.Env...)
			envFrom = append(envFrom, taskOption.EnvFrom...)
			for key, value := range taskOption.Annotations {
				annotations[key] = value
			}
			for key, value := range taskOption.Labels {
				labels[key] = value
			}
		}
	}
	env = append(env, []corev1.EnvVar{
		{
			Name:  "TFO_TASK",
			Value: "debug",
		},
		{
			Name:  "TFO_RESOURCE",
			Value: tf.Name,
		},
		{
			Name:  "TFO_NAMESPACE",
			Value: tf.Namespace,
		},
		{
			Name:  "TFO_GENERATION",
			Value: generation,
		},
		{
			Name:  "TFO_GENERATION_PATH",
			Value: generationPath,
		},
		{
			Name:  "TFO_MAIN_MODULE",
			Value: generationPath + "/main",
		},
		{
			Name:  "TFO_TERRAFORM_VERSION",
			Value: tf.Spec.TerraformVersion,
		},
	}...)

	volumes := []corev1.Volume{
		{
			Name: "tfohome",
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: tf.Status.PodNamePrefix,
					ReadOnly:  false,
				},
			},
		},
	}
	volumeMounts := []corev1.VolumeMount{
		{
			Name:      "tfohome",
			MountPath: "/home/tfo-runner",
			ReadOnly:  false,
		},
	}
	env = append(env, corev1.EnvVar{
		Name:  "TFO_ROOT_PATH",
		Value: "/home/tfo-runner",
	})

	optional := true
	xmode := int32(0775)
	volumes = append(volumes, corev1.Volume{
		Name: "gitaskpass",
		VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{
				SecretName: versionedName,
				Optional:   &optional,
				Items: []corev1.KeyToPath{
					{
						Key:  "gitAskpass",
						Path: "GIT_ASKPASS",
						Mode: &xmode,
					},
				},
			},
		},
	})
	volumeMounts = append(volumeMounts, []corev1.VolumeMount{
		{
			Name:      "gitaskpass",
			MountPath: "/git/askpass",
		},
	}...)
	env = append(env, []corev1.EnvVar{
		{
			Name:  "GIT_ASKPASS",
			Value: "/git/askpass/GIT_ASKPASS",
		},
	}...)

	for _, c := range tf.Spec.Credentials {
		if c.AWSCredentials.KIAM != "" {
			annotations["iam.amazonaws.com/role"] = c.AWSCredentials.KIAM
		}
	}

	for _, c := range tf.Spec.Credentials {
		if (tfv1beta1.SecretNameRef{}) != c.SecretNameRef {
			envFrom = append(envFrom, []corev1.EnvFromSource{
				{
					SecretRef: &corev1.SecretEnvSource{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: c.SecretNameRef.Name,
						},
					},
				},
			}...)
		}
	}

	labels["terraforms.tf.isaaguilar.com/generation"] = generation
	labels["terraforms.tf.isaaguilar.com/resourceName"] = tf.Name
	labels["terraforms.tf.isaaguilar.com/podPrefix"] = tf.Status.PodNamePrefix
	labels["terraforms.tf.isaaguilar.com/terraformVersion"] = tf.Spec.TerraformVersion
	labels["app.kubernetes.io/name"] = "terraform-operator"
	labels["app.kubernetes.io/component"] = "terraform-operator-cli"
	labels["app.kubernetes.io/instance"] = "debug"
	labels["app.kubernetes.io/created-by"] = "cli"

	initContainers := []corev1.Container{}
	containers := []corev1.Container{}

	// Make sure to use the same uid for containers so the dir in the
	// PersistentVolume have the correct permissions for the user
	user := int64(0)
	group := int64(2000)
	runAsNonRoot := false
	privileged := true
	allowPrivilegeEscalation := true
	seLinuxOptions := corev1.SELinuxOptions{}
	securityContext := &corev1.SecurityContext{
		RunAsUser:                &user,
		RunAsGroup:               &group,
		RunAsNonRoot:             &runAsNonRoot,
		Privileged:               &privileged,
		AllowPrivilegeEscalation: &allowPrivilegeEscalation,
		SELinuxOptions:           &seLinuxOptions,
	}
	restartPolicy := corev1.RestartPolicyNever

	containers = append(containers, corev1.Container{
		SecurityContext: securityContext,
		Name:            "debug",
		Image:           "ghcr.io/galleybytes/terraform-operator-tftaskv1.1.0:" + terraformVersion,
		Command: []string{
			"/bin/sleep", "86400",
		},
		ImagePullPolicy: corev1.PullIfNotPresent,
		EnvFrom:         envFrom,
		Env:             env,
		VolumeMounts:    volumeMounts,
	})

	podSecurityContext := corev1.PodSecurityContext{
		FSGroup: &group,
	}
	serviceAccount := tf.Spec.ServiceAccount
	if serviceAccount == "" {
		// By prefixing the service account with "tf-", IRSA roles can use wildcard
		// "tf-*" service account for AWS credentials.
		serviceAccount = "tf-" + versionedName
	}

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: generateName,
			Namespace:    tf.Namespace,
			Labels:       labels,
			Annotations:  annotations,
		},
		Spec: corev1.PodSpec{
			SecurityContext:    &podSecurityContext,
			ServiceAccountName: serviceAccount,
			RestartPolicy:      restartPolicy,
			InitContainers:     initContainers,
			Containers:         containers,
			Volumes:            volumes,
		},
	}

	return pod
}

func RemoteDebug(name string, tty *os.File) {
	log.Println("NO REQ wooohooo1")

	newSession()
	// config       *rest.Config
	// tfoclientset tfo.Interface
	// clientset    kubernetes.Interface
	// namespace    string

	// tfClient := tfo.New()

	tfClient := session.tfoclientset.TfV1beta1().Terraforms(session.namespace)
	podClient := session.clientset.CoreV1().Pods(session.namespace)

	tf, err := tfClient.Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil {
		log.Fatal(err)
	}

	pod := generatePod(tf)
	pod, err = podClient.Create(context.TODO(), pod, metav1.CreateOptions{})
	if err != nil {
		log.Fatal(err)
	}
	// defer podClient.Delete(context.TODO(), pod.Name, metav1.DeleteOptions{})

	fmt.Printf("Connecting to %s ", pod.Name)

	watcher, err := podClient.Watch(context.TODO(), metav1.ListOptions{
		FieldSelector: "metadata.name=" + pod.Name,
	})
	if err != nil {
		log.Fatal(err)
	}

	for event := range watcher.ResultChan() {
		fmt.Printf(".")
		switch event.Type {
		case watch.Modified:
			pod = event.Object.(*corev1.Pod)
			// If the Pod contains a status condition Ready == True, stop
			// watching.
			for _, cond := range pod.Status.Conditions {
				if cond.Type == corev1.PodReady &&
					cond.Status == corev1.ConditionTrue &&
					pod.Status.Phase == corev1.PodRunning {
					watcher.Stop()
				}
			}
		default:
			// fmt.Fprintln(os.Stderr, event.Type)
		}
	}
	log.Println("")

	log.Printf("tty isTerminal? %t", isTerminal(tty))

	ioStreams := genericclioptions.IOStreams{In: tty, Out: tty, ErrOut: tty}
	log.Println("Setting up steamOptions")
	streamOptions := exec.StreamOptions{
		IOStreams: ioStreams,
		Stdin:     true,
		TTY:       true,
	}
	log.Println("Checking TTY setup")
	t := streamOptions.SetupTTY()

	var sizeQueue remotecommand.TerminalSizeQueue
	if t.Raw {
		// this call spawns a goroutine to monitor/update the terminal size
		sizeQueue = t.MonitorSize(t.GetSize())

		// unset p.Err if it was previously set because both stdout and stderr go over p.Out when tty is
		// true
		streamOptions.ErrOut = nil
	}
	// log.Println(file.Name())
	log.Println("Setting up request")
	// go func() {

	/*







		req := session.clientset.CoreV1().RESTClient().
			Post().
			Namespace(pod.Namespace).
			Resource("pods").
			Name(pod.Name).
			SubResource("exec").
			VersionedParams(&corev1.PodExecOptions{

				Container: pod.Spec.Containers[0].Name,
				Command: []string{
					"/bin/bash",
					"-c",
					"cd $TFO_MAIN_MODULE && export PS1=\"\\w\\$ \" && " +
						"if [[ -n \"$AWS_WEB_IDENTITY_TOKEN_FILE\" ]];then " +
						"export $(irsa-tokengen); " +
						"echo printf \"\nAWS creds set from token file\n\";fi &&" +
						"printf \"\nTry running 'terraform init'\n\n\" && bash",
				},
				// Stdin:  streamOptions.Stdin,
				// Stdout: streamOptions.Out != nil,
				// Stderr: streamOptions.ErrOut != nil,
				Stdin:  true,
				Stdout: true,
				Stderr: true,
				// TTY:    t.Raw,
				TTY: true,
			},
				scheme.ParameterCodec,
			)

		if false {

			log.Println("I SHOULD NOT SEE THIS")

			exec, err := remotecommand.NewSPDYExecutor(session.config, "POST", req.URL())
			if err != nil {
				panic(err)
			}

			exec.StreamWithContext(context.TODO(), remotecommand.StreamOptions{
				// Stdin:  pr,
				// Stdout: pw,
				// Stderr: pw,
				Stdin:             streamOptions.In,
				Stdout:            streamOptions.Out,
				Stderr:            streamOptions.ErrOut,
				Tty:               true,
				TerminalSizeQueue: sizeQueue,
			})
		}
		// }()
	*/
	fn := func() error {
		req := session.clientset.CoreV1().RESTClient().
			Post().
			Namespace(pod.Namespace).
			Resource("pods").
			Name(pod.Name).
			SubResource("exec").
			VersionedParams(&corev1.PodExecOptions{
				Container: pod.Spec.Containers[0].Name,
				Command: []string{
					"/bin/bash",
					"-c",
					"cd $TFO_MAIN_MODULE && export PS1=\"\\w\\$ \" && " +
						"if [[ -n \"$AWS_WEB_IDENTITY_TOKEN_FILE\" ]];then " +
						"export $(irsa-tokengen); " +
						"echo printf \"\nAWS creds set from token file\n\";fi &&" +
						"printf \"\nTry running 'terraform init'\n\n\" && bash",
				},
				Stdin:  streamOptions.Stdin,
				Stdout: streamOptions.Out != nil,
				Stderr: streamOptions.ErrOut != nil,
				TTY:    t.Raw,
			}, scheme.ParameterCodec)

		return func() error {

			exec, err := remotecommand.NewSPDYExecutor(session.config, "POST", req.URL())
			if err != nil {
				panic(err)
			}

			return exec.Stream(remotecommand.StreamOptions{
				Stdin:             streamOptions.In,
				Stdout:            streamOptions.Out,
				Stderr:            streamOptions.ErrOut,
				Tty:               t.Raw,
				TerminalSizeQueue: sizeQueue,
			})
		}()

	}

	if err := t.Safe(fn); err != nil {
		panic(err)
	}

}

func isTerminal(file *os.File) bool {

	inFd := file.Fd()
	log.Println(file.Name(), inFd)
	_, err := unix.IoctlGetTermios(int(inFd), unix.TIOCGETA)
	return err == nil

}
