# K8S-SYNC
k8s-sync is a simple CLI to reconcile a yaml file or directory against the cluster. When working with manifests designed for GitOps deployment, it can be quite time consuming to manually apply manifests in a specific order using `kubectl`. `k8s-sync` allows you to reconcile objects defined in a YAML or
JSON file (or a directory containing them) against a Kubernetes cluster following a GitOps pattern.

`k8s-sync will` try to create target resources against your current cluster context, and, if some of the object dependencies are not found on the cluster,
it will reapply failed objects against the cluster until success. If the state of your YAML/JSON resources is eventually
consistant, you should be good to go!

---

## Install

To install the project using go:

    go get -v github.com/SamChinellato/k8s-sync

---

## Reconcile a file or directory

To reconcile a resource against your current cluster context:

    k8s-sync reconcile -f <path-to-file-or-dir>

To set the reapply interval to 10 seconds:

    k8s-sync reconcile -f <path-to-file-or-dir> -i 10

`k8s-sync` will then reapply the selected resources until eventual state consistency is reached. If there are any subdirectories in the target directory, `k8s-sync` will automatically parse down them and apply YAML/JSON resources.

---

## Delete a file or directory

To delete resources in a file or directory:

	k8s-sync cleanup -f <path-to-file-or-dir>

If a directory is specified, k8s-sync will try to delete all resources in the directory and any subdirectories from the current cluster context Kubernetes cluster.	

---

