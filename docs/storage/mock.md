# Mock Storage

The `mock` storage does not really store the synced resources anywhere, instead it just logs what it would have stored instead. It can be used for testing the sync configuration before 'activating' it. `mockConfig` can be set to configure the mock storage, but opposed to other storage options, this is optional. A filesystem storage is used internally, so `filesystemConfig` can be specified to. The mock storage always uses an in-memory filesystem, independt of the value of `filesystemConfig.inMemory`.

The `subPath` field in the storage reference is expected to be a directory path without leading or trailing `/`, e.g. `a/b/c`. If empty, it will use the configured filesystem's root path (`filesystemConfig.rootPath`).

## Configuration

```yaml
- name: myStorage
  type: filesystem
  filesystemConfig: ... # optional
  mockConfig:
    logPersisterCallsOnInfoLevel: false
```

- `logPersisterCallsOnInfoLevel` - If set to `true`, the persister calls are logged on `INFO` verbosity (instead of `DEBUG`). This allows inspecting the persister calls of a specific sync configuration without having to switch the logging verbosity to `DEBUG` for the whole controller.


