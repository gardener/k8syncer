# State via Annotations

The `annotation` state type writes the sync state into the resource's annotations.

## Configuration

The `annotation` state type does not require any specific configuration.

```yaml
state:
  type: annotation
  verbosity: detail
```

The state will then look like this:
```yaml
metadata:
  annotations:
    state.k8syncer.gardener.cloud/detail: ""
    state.k8syncer.gardener.cloud/lastSyncedGeneration: "1"
    state.k8syncer.gardener.cloud/phase: Finished
```
