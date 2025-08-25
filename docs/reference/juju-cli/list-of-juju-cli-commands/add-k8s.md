(command-juju-add-k8s)=
# `juju add-k8s`
> See also: [remove-k8s](#remove-k8s)

## Summary
Adds a Kubernetes endpoint and credential to Juju.

## Usage
```juju add-k8s [options] <k8s name>```

### Options
| Flag | Default | Usage |
| --- | --- | --- |
| `-B`, `--no-browser-login` | false | Do not use web browser for authentication |
| `-c`, `--controller` |  | Controller to operate in |
| `--client` | false | Client operation |
| `--cloud` |  | Specify the Kubernetes cluster cloud |
| `--cluster-name` |  | Specify the Kubernetes cluster to import |
| `--context-name` |  | Specify the Kubernetes context to import |
| `--credential` |  | Specify the credential to use when accessing the cluster |
| `--region` |  | Specify the Kubernetes cluster region or cloud/region |
| `--skip-storage` | false | Skip storage provisioning for clusters that don't have storage |
| `--storage` |  | Specify the Kubernetes storage class for workload storage |

## Examples

When your kubeconfig file is in the default location:

    juju add-k8s myk8scloud
    juju add-k8s myk8scloud --client
    juju add-k8s myk8scloud --controller mycontroller
    juju add-k8s --context-name mycontext myk8scloud
    juju add-k8s myk8scloud --region cloudNameOrCloudType/someregion
    juju add-k8s myk8scloud --cloud cloudNameOrCloudType
    juju add-k8s myk8scloud --cloud cloudNameOrCloudType --region=someregion
    juju add-k8s myk8scloud --cloud cloudNameOrCloudType --storage mystorageclass

To add a Kubernetes cloud using data from your kubeconfig file, when this file is not in the default location:

    KUBECONFIG=path-to-kubeconfig-file juju add-k8s myk8scloud --cluster-name=my_cluster_name

To add a Kubernetes cloud using data from kubectl, when your kubeconfig file is not in the default location:

    kubectl config view --raw | juju add-k8s myk8scloud --cluster-name=my_cluster_name



## Details

Creates a user-defined cloud based on a Kubernetes cluster.

The new Kubernetes cloud can then be used to bootstrap into, or it
can be added to an existing controller.

Specify a non default kubeconfig file location using `$KUBECONFIG` environment variable or pipe in file content from stdin.

The config file can contain definitions for different Kubernetes clusters,
use `--cluster-name` to pick which one to use.
It's also possible to select a context by name using `--context-name`.

When running `add-k8s` the underlying cloud/region hosting the cluster needs to be
detected to enable storage to be correctly configured. If the cloud/region cannot
be detected automatically, use either
  `--cloud <cloudType|cloudName> `to specify the host cloud
or
  `--region <cloudType|cloudName>/<someregion>` to specify the host
  cloud type and region.

Region is strictly necessary only when adding a Kubernetes cluster to a JAAS controller.
When using a standalone Juju controller, usually just `--cloud` is required.

Once Juju is aware of the underlying cloud type, it looks for a suitably configured
storage class to provide operator and workload storage. If none is found, use
of the `--storage` option is required so that Juju will create a storage class
with the specified name.

If the cluster does not have a storage provisioning capability, use the
`--skip-storage` option to add the cluster without any workload storage configured.