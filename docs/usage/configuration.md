# Configuration

K8Syncer requires a configuration which specifies which resources should be persisted in which storage.

**Example:**
```yaml
syncConfigs:
- id: fooDummyWatcher
  resource:
    kind: Dummy
    version: v1
    group: k8syncer.gardener.cloud
    namespace: foo
  state:
    type: status
    verbosity: detail
    statusConfig:
      generationPath: syncStatus.lastSyncedGeneration
      phasePath: syncStatus.phase
      detailPath: syncStatus.detail
  storageRefs:
  - name: myStorage
    subPath: "foo/foo_data/dummies"

storageDefinitions:
- name: myStorage
  type: git
  filesystemConfig:
    rootPath: "/tmp/k8syncer"
  gitConfig:
    url: "https://github.com/myuser/mygit.git"
    branch: master
    exclusive: true
    auth:
      type: username_password
      username: <git username>
      password: <git password or access token>
```

This part of the documenation covers mainly the `syncConfigs` field of the config file. There is further documentation for the different [storage types](../storage/README.md) and [state options](../state/README.md).


## Sync Configuration

The main functionality of K8Syncer is to sync k8s resources from a cluster into some kind of storage. The `syncConfigs` field of the configuration therefore contains a list of sync configurations, each of which specifies a single resource type that should be watched as well as a list of references to storage definitions. If a matching resource is changed, it will be synced to all storages referenced here.

```yaml
syncConfigs:
...
- id: fooDummyWatcher
  resource:
    kind: Dummy
    version: v1
    group: k8syncer.gardener.cloud
    namespace: foo # optional
  state: # optional
    type: status
    verbosity: detail
    statusConfig:
      generationPath: syncStatus.lastSyncedGeneration
      phasePath: syncStatus.phase
      detailPath: syncStatus.detail
  storageRefs:
  - name: myStorage
    subPath: "foo/foo_data/dummies"
  finalize: true # optional
```

- `id` - Some freely chosen identifier for the sync config. Must be unique. This is only used for logging. Must only consist of letters, digits, `-`, and `_`.
- `resource`
  - `kind` - The kind of the resource to be watched.
  - `group` - The group of the resource to be watched. Might be empty for core resources, e.g. namespaces.
  - `version` - The version of the resource to be watched.
  - `namespace` - If the resource is namespaced and only resources from a specific namespace should be watched, the namespace can be specified here. An empty string or leaving out this field completely will result in the resource being watched across all namespaces.
  - Note that multiple sync configurations for the same resource must have disjunct sets of storage references to avoid problems with concurrency.
- `state` - If configured, K8Syncer will attach the state of the latest sync to the synced resource. See the [state documentation](../state/README.md) for further information. State won't be updated for resources in deletion if `finalize` is set to `false`.
  - `type` - In which way the state should be shown on the resource. Set to `none` or leave out `state` completely to disable state display.
  - `verbosity` - How verbose the state should be.
- `storageRefs` - A list of references to storages defined in `storageDefinitions`. The specified resource type will be synced to all of these storages.
  - `name` - The name of the referenced storage definition. There has to be an entry in `storageDefinitions` with the same `name` as specified here.
  - `subPath` - Usually, digital storage options have some kind of tree-like architecture, for example directories in filesystems. This field allows to specify the path along the tree, from the root of the referenced storage, which should be used as root for storing the resources.
    - The format of the path depends on the type of the storage. For filesystem-like storage, it will usually look like `a/b/c`, but for different storage architectures, it might look differently or be ignored completely.
- `finalize` - If true, K8Syncer will add a finalizer to resources of this type. This has two major advantages: It is then possible to display the state also for resources which are in deletion and K8Syncer will not miss deletion of resources, even if the resource is deleted while the controller is not running (deletion of the resource will be blocked, though). Defaults to `true`.

⚠️ Currently, K8Syncer only notices changes to the `labels`, `generation`, and `ownerReferences` metadata fields. For some native and all custom resources, the apiserver usually increases the generation whenever the resource's `spec` changes. However, there are some resources for which this is not the case, for example secrets don't have their generation increased when their content changes. As a result, K8Syncer can currently not sync secrets and similar resources which don't make use of the `metadata.generation` field. A possible workaround would be to modify a label on the resource whenever its content changes to ensure that the change is picked up by K8Syncer. This might be improved in the future.


## Storage Definitions

A storage definition defines access to a specific storage. There are a few common fields, but most of the configuration is specific to the type of storage used. These specific configurations are described [here](../storage/README.md).

```yaml
storageDefinitions:
- name: myStorage
  type: git
  <type-specific configuration>
```

- `name` - A unique identifier for this storage. This is used to reference storage definitions in the sync configurations. It must only consist of letters, digits, `-`, and `_`.
- `type` - The type of the storage. It determines which of the type-specific fields are expected to be set. See the mentioned storage documentation for details on the supported types and their required configurations.

