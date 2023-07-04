# Filesystem Storage

With the `filesystem` storage type, it is possible to sync k8s resources to the local filesystem.
This requires the `filesystemConfig` field to be set.

The `subPath` field in the storage reference is expected to be a directory path without leading or trailing `/`, e.g. `a/b/c`. If empty, it will use the configured filesystem's root path (`filesystemConfig.rootPath`).

## Configuration

```yaml
- name: myStorage
  type: filesystem
  filesystemConfig:
    rootPath: "/tmp/k8syncer"
    namespacePrefix: "ns_" # optional
    gvrNameSeparator: "_" # optional
    fileExtension: yaml # optional
    inMemory: false # optional
```

- `rootPath` - The path on the local filesystem which should be used as root. This directory has to exist, unless an in-memory filesystem is used.
- `namespacePrefix` - Namespace directories will be prefixed with this prefix. Defaults to `ns_`
- `gvrNameSeparator` - This will be used as separator between the resource's `GroupVersionResource` string and its name. Defaults to `_`.
- `fileExtension` - Will be used as file extension for the resource files. May be specified with or without a leading `.`. Defaults to `yaml`.
- `inMemory` - If true, an virtual in-memory filesystem will be used. Defaults to `false`.


## Effect

The filesystem persister stores the resources on the local filesystem. For each configured sync, an own root folder is used, which is determined by joining the storage definition's `rootPath` with the storage reference's `subPath` fields. Within in this resource-specific root folder, cluster-scoped resources are put at top-level, while namespace-scoped resources are grouped in directories which correspond to the namespaces. The names of these namespace directories are determined by adding the specified `namespacePrefix` to the name of the namespace. The names of the resource files are determined by joining the `GroupVersionResource` value for the resource with its name, separated by the specified `gvrNameSeparator`. Note that the trailing `.` for resources without group is omitted in this case.

The content of the resource files corresponds to the YAML manifest of the resource. To avoid syncing volatile fields, the `status` is omitted and from the `metadata`, only `name`, `generateName`, `namespace`, `uid`, `labels`, and `ownerReferences` are persisted.


## Limitations

Base paths - `rootPath` from the filesystem configuration joined with `subPath` from the storage reference of the sync configuration - must not be nested for shared filesystems. The reason for this is that nested base paths could cause conflicts with the created folder structure. Multiple sync configurations may use the same base path, though.
Note that filesystem storage definitions with `inMemory` set to `true` usually use their own virtual filesystem each, so this limitation does not apply to them.


## Examples

All examples refer to the storage definition from the example above, unless specified otherwise.

### #1

Disclaimer: Namespaces don't have a generation and therefore not all features of K8Syncer will work properly for them (see [limitations](../../README.md#limitations)). This example uses them nonetheless, because they are a well-known, easy-to-understand representative for cluster-scoped resources.

Sync configuration:
```yaml
- id: namespaceWatcher
  resource:
    resource: "namespaces.v1."
  storageRefs:
  - name: myStorage
    subPath: "namespaces"
```

Resource:
```yaml
apiVersion: v1
kind: Namespace
metadata:
  creationTimestamp: "2023-06-13T05:55:26Z"
  labels:
    kubernetes.io/metadata.name: foo
  name: foo
  resourceVersion: "277344"
  uid: d6ef6dd8-0b85-425c-9a64-3238a7d366e0
spec:
  finalizers:
  - kubernetes
status:
  phase: Active
```

Persisted result:
```
/tmp/k8syncer/namespaces/namespaces.v1_foo.yaml
```
```yaml
apiVersion: v1
kind: Namespace
metadata:
  labels:
    kubernetes.io/metadata.name: foo
  name: foo
  uid: d6ef6dd8-0b85-425c-9a64-3238a7d366e0
spec:
  finalizers:
  - kubernetes
```

### #2

Sync configuration:
```yaml
- id: deploymentWatcher
  resource:
    resource: "deployments.v1.apps"
  storageRefs:
  - name: myStorage
    subPath: "mydeploys"
```

Resource:
```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  creationTimestamp: "2023-05-31T06:29:46Z"
  generation: 1
  name: foo
  namespace: default
  resourceVersion: "228187"
  uid: af230f52-6c4f-4c75-90a4-c826a62c03c0
  annotations:
    deployment.kubernetes.io/revision: "1"
spec: <deployment spec>
status: <deployment status>
```

Persisted result:
```
/tmp/k8syncer/mydeploys/ns_default/deployments.v1.apps_foo.yaml
```
```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: foo
  namespace: default
  uid: af230f52-6c4f-4c75-90a4-c826a62c03c0
spec: <deployment spec>
```

