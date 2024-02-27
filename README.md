# K8Syncer
[![REUSE status](https://api.reuse.software/badge/github.com/gardener/k8syncer)](https://api.reuse.software/info/github.com/gardener/k8syncer)

K8Syncer is a tool to backup k8s resources into some kind of storage. While it was designed to be easily extensible with further storage options, the primary goal was to be able to backup resources from a k8s cluster into a git repository, and the tool does not support much more than this use-case at the moment.

See [here](docs/storage/README.md) for a documentation of the supported storage types and [here](docs/usage/configuration.md) for how to configure the tool.

## Limitations

At the moment, K8Syncer reacts only to changes on the `metadata` fields `generation`, `labels`, and `ownerReferences` of k8s resources. This means that resources kinds for which the k8s apiserver does not increase the generation are not synced on a spec update. This applies to namespaces and secrets, for example. A workaround would be to trigger a sync by changing a label on the resource.


## How to use K8Syncer

Although it is possible to run K8Syncer locally - one simply has to provide its [configuration](docs/usage/configuration.md) via `--config` and a kubeconfig for the target cluster, either via `KUBECONFIG` env var or `--kubeconfig` - it was designed to run as a controller inside a kubernetes cluster. The easiest way to install it is by using the provided helm chart.

```yaml
image: # can usually be left out
  repository: eu.gcr.io/gardener-project/k8syncer
  tag: "0.1.0"

config:
  # kubeconfig has to be provided if k8syncer should watch another cluster than the one it is running in
  # kubeconfig: |
  #   apiVersion: v1
  #   clusters:
  #   - cluster: ...

  syncConfigs: # goes directly into the k8syncer configuration, see docs/usage/configuration.md
  - id: dummyWatcher
    resource:
      group: k8syncer.gardener.cloud
      version: v1
      kind: Dummy
    storageRefs:
    - name: mockStorage
    state:
      type: annotation
      verbosity: detail

  storageDefinitions: # goes directly into the k8syncer configuration, see docs/usage/configuration.md
  - name: mockStorage
    type: mock

# resources:
#   requests:
#     cpu: 100m
#     memory: 256Mi
#   limits:
#     cpu: 500m
#     memory: 2Gi

logging:
  verbosity: info # error, info, or debug (defaults to 'info' if omitted)
```

## Learn more!

Have a look at the documentation:
- [documentation index](docs/README.md)
- [K8Syncer configuration](docs/usage/configuration.md)
- [storage types](docs/storage/README.md)
- [state display](docs/state/README.md)