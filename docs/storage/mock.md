# Mock Storage

The `mock` storage does not really store the synced resources anywhere, instead it just logs what it would have stored instead. It can be used for testing the sync configuration before 'activating' it. `mockConfig` can be set to configure the mock storage, but opposed to other storage options, this is optional. Internally, the mock persister just dumps the stored resources into a map, using a combination of name, namespace, GroupVersionKind, and subPath as key.

The `subPath` field in the storage reference can be an arbitrary string.

## Configuration

```yaml
- name: myStorage
  type: mock
  mockConfig:
    logPersisterCallsOnInfoLevel: false
```

- `logPersisterCallsOnInfoLevel` - If set to `true`, the persister calls are logged on `INFO` verbosity (instead of `DEBUG`). This allows inspecting the persister calls of a specific sync configuration without having to switch the logging verbosity to `DEBUG` for the whole controller.


