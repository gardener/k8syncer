# State via Status

The `status` state type allows writing the sync state to specified fields in the resource's `status` subresource. This requires the synced resource to have a status subresource and also have designated fields for the sync state in there.

## Configuration

The `status` state type requires a `statusConfig`.

```yaml
state:
  type: status
  verbosity: detail
  statusConfig:
    generationPath: syncStatus.lastSyncedGeneration
    phasePath: syncStatus.phase
    detailPath: syncStatus.detail
```

The fields `generationPath`, `phasePath`, and `detailPath` all work in the same way: They contain the path to the field within the status where the corresponding state value should be written. The path allows only for simple map field access, accessing list elements or more complex jsonPath logic is not supported.

The configured verbosity defines which of the fields need to be provided, e.g. for verbosity `phase` no detail will be written into the state, so the field `detailPath` is not required in that case.

The above example could result in the following status:
```yaml
status:
  syncStatus:
    lastSyncedGeneration: 1
    phase: Finished
    detail: ""
```

Note that the generation is an integer, while phase and detail will result in strings. In a k8s CRD, you could use the following snippet to add the sync state to the status:
```yaml
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata: ...
spec:
  ...
  versions:
  - name: v1
    ...
    schema:
      openAPIV3Schema:
        type: object
        properties:
          ...
          status:
            type: object
            properties:
              syncStatus:
                type: object
                properties:
                  lastSyncedGeneration:
                    type: integer
                  phase:
                    type: string
                  detail:
                    type: string
    subresources:
      status: {}
```

A small caveat: If the specified path does not exist (e.g. due to a typo in the configuration), there won't be any error. The state will simply not appear in the status.
