# K8Syncer

K8Syncer is a tool to backup k8s resources into some kind of storage.

See [here](docs/storage/README.md) for a documentation of the supported storage types and [here](docs/usage/configuration.md) for how to configure the tool.

## Limitations

At the moment, K8Syncer reacts only to changes on the `metadata` fields `generation`, `labels`, and `ownerReferences` of k8s resources. This means that resources kinds for which the k8s apiserver does not increase the generation are not synced on a spec update. This applies to namespaces and secrets, for example. A workaround would be to trigger a sync by changing a label on the resource.
