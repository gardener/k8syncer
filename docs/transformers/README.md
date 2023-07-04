# Transformers

A `Transformer` is a piece of code which transforms the k8s resource into the format in which it is then stored in the configured storage.

Currently, only a basic transformer is implemented and nothing can be configured for it. Might be expanded in the future.


## Basic

The basic transformer removes highly volatile fields from the k8s resource and marshals it to YAML:
- If the resource has a `status`, it is removed.
- From the resource's `metadata`, only the following fields are preserved:
  - `name`
  - `generateName`
  - `namespace`
  - `uid`
  - `labels`
  - `ownerReferences`
- All other fields of the resource are preserved.


### Example

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

Result after transformation:
```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: foo
  namespace: default
  uid: af230f52-6c4f-4c75-90a4-c826a62c03c0
spec: <deployment spec>
```

