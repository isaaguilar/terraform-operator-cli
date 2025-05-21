# Infrakube CLI [ik]

A CLI tool to aid in discovering Infrakube managed pods and running debug sessions



## Installation

**Binary files will be created when the cli is released**

Requires golang 1.24

You can manually build the project by:


1. Download the repo https://github.com/isaaguilar/infrakube-cli
2. Run the `go build` command

```bash
git clone https://github.com/isaaguilar/infrakube-cli.git
cd infrakube-cli
go build -o ik main.go
mv ik /usr/local/bin
```


## Usage

Run `ik help` for all options.



### `ik local debug`

Opens a **debug** session.

```bash
ik local debug <tf-resource-name>
```

**Example:**

```bash
kubectl apply --namespace default -f - << EOF
apiVersion: tf.isaaguilar.com/v1alpha2
kind: Terraform
metadata:
  name: stable
spec:
  terraformModule:
    source: https://github.com/isaaguilar/simple-aws-tf-modules.git//create_file
  backend: |-
    terraform {
      backend "kubernetes" {
        secret_suffix    = "stable"
        in_cluster_config  = true
      }
    }
  ignoreDelete: false
EOF
```

Output should look like:

```
terraform.tf.isaaguilar.com/stable configured
```

Then run a debug pod:

```bash
ik local debug --namespace default stable
```

This command will create a pod on the cluster using the tf resource for configuration. The pod puts the user in the terraform module.
```
Connecting to stable-2huxns3o-v3-debug-xhhtg.....

Try running 'terraform init'

/home/i3-runner/generations/3/main$
```

Finally, debug and exit.

```bash
/home/i3-runner/generations/3/main$ terraform plan
null_resource.write_file: Refreshing state... [id=2066474016391370391]

No changes. Your infrastructure matches the configuration.

Terraform has compared your real infrastructure against your configuration and found no differences, so no changes are needed.

/home/i3-runner/generations/3/main$ exit
exit
```

Notice the debug pod terminates as soon as the user exits.

```bash
kubectl get po | grep stable-2huxns3o-v3-debug-xhhtg
stable-2huxns3o-v3-debug-xhhtg                 1/1     Terminating   0          4m20s
```

