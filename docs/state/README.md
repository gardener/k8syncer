# State

For some scenarios, the information whether or not a resource has already been synced might be relevant. To communicate this information, K8Syncer can be configured to attach state to synced resources.


## Configuration

```yaml
state:
  type: <state type>
  verbosity: <generation|phase|detail>
  <further configuration depending on type>
```

State can be displayed in three different verbosity levels:
- `generation` - Only the `metadata.generation` of the latest synced resource version is displayed in the state.
- `phase` - Includes `generation`. Additionally, the current phase of the sync is written to the state:
  - `Progressing` means a change has been picked up and the sync is ongoing.
  - `Finished` means the resource has successfully been synced.
  - `Error` means there was a problem during the last sync. Syncing will be retried.
  - `Deleting` is the same as `Progressing`, but is used if the resource is being deleted.
  - `ErrorDeleting` is the same as `Error`, but is used if the resource is being deleted.
- `detail` - Includes `phase`. In case of an error (phase `Error` or `ErrorDeleting`), the error details are written to the state.

There are different types of states which have their own documentation each:
- `none` - No state should be attached to the resource.
- [`status`](status.md) - Write the state to specified fields in the `status` subresource of the synced resource.
- [`annotation`](annotation.md) - Write the state as annotations on the resource.


## Working with State

If another k8s controller is expected to react on the K8Syncer state, there are a few useful structs and methods which will be explained here shortly.

### IsSynced

The `IsSynced` function from `github.com/gardener/k8syncer/pkg/state` can be used to check whether the latest version of the resource has been successfully synced.

Currently, the function just compares the sync state's last synced generation field to the resource's current generation to determine whether it has been synced, but this could be enhanced in the future.

It takes a `client.Object` - the k8s resource with the state - and either a `StateDisplay` or a `*SyncState`. If the state was already read from the resource and stored in a `SyncState` object by any means, this state object can simply be passed as third argument, with the second one being nil. If the state has not been read yet, the second argument is required as it contains the information how the state is stored in the object, simply pass `nil` as third argument then.

To get the `StateDisplay` object, call the corresponding constructor function (same package, name starts with `New`) and instantiate it with the same values that are configured in the K8Syncer state configuration for this resource.

#### Example
K8Syncer state configuration:
```yaml
state:
  type: status
  verbosity: phase
  statusConfig:
    generationPath: syncStatus.lastSyncedGeneration
    phasePath: syncStatus.phase
```

StateDisplay instantiation:
```golang
sd := state.NewStatusStateDisplay("syncStatus.lastSyncedGeneration", "syncStatus.phase", "", state.STATE_VERBOSITY_PHASE)
```

Calling `IsSynced` on `obj`:
```golang
synced, err := state.IsSynced(obj, sd, nil)
```


### StateDisplay

`StateDisplay` is an interface which represents a way of storing a state on a k8s resource. Of its methods, these two might be interesting for usage outside of K8Syncer:
```golang
Read(obj client.Object) (*SyncState, StateError)
```
`Read` reads the state from a k8s resource. The types of the returned objects are described below.
```golang
Verbosity() StateVerbosity
```
`Verbosity` returns the verbosity configured for the state display. The available verbosities are defined as constants in the `state` package.


### SyncState

`SyncState` is a struct which stores the state. It contains fields for generation, phase, and detail, as well as a verbosity that defines which of the other three fields are actually set. The fields are exported, but there are also some helper methods which allow accessing the fields dynamically. Whenever any of these methods asks for a `*StateField` argument, one of the constants of the `state` package starting with `STATE_FIELD_` should be used.


### StateError

All errors returned by functions in the `state` package are usually `StateError`s. The following errors are defined:
- `MissingStateError`
  - Means that reading state from an object failed because either the complete state or parts of it are missing on the object. Which state fields are expected to exist on the object can be controlled with the verbosity that is present in many structs and functions.
- `InvalidStateError`
  - Means that a state field contained an invalid value, e.g. if the generation read from the state cannot be parsed into an integer.
- `ReadStateError`
  - Something went wrong while reading the state from an object. Is only returned if neither `MissingStateError` nor `InvalidStateError` fit the problem.
- `WriteStateError`
  - Something went wrong while writing the state to an object.
- `InternalStateError`
  - This error is mostly thrown if methods are called in the wrong way, e.g. something is `nil` which is not expected to be `nil`. If it appears, it usually hints at a problem in the code, not in the configuration.

The `state` package has `Is<ErrorType>` functions to identify a returned error.

