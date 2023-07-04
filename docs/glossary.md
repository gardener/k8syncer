# Glossary

### Persister
A Persister contains the logic for how to store data for a given storage type. Code-wise, it's an interface which is implemented for each storage option. For the documentation, see [storage](storage/README.md). Each storage definition provided in the configuration will result in one initialization of the corresponding Persister.

### State
The current state of the sync of a specific resource. Depending on the configuration it can contain the last successfully synced generation of the resource, whether there is an ongoing sync, and details (mostly error messages) about the current sync, if any.

### Storage
The place where the synced resources are synced to. Different storage types are supported. More or less equivalent to [Persister](#persister).

### Storage Definition
One element of the `storageDefinitions` list of the K8Syncer configuration. See also [Storage](#storage) and [Persister](#persister).

### Storage Option/Type
See [Storage](#storage).

### Sync Configuration
One sync configuration defines a single resource type which should be watched and stored. Configuration-wise, each element of `syncConfigs` in the configuration is a sync configuration. It defines the resource being synced, to which storages it is synced, and how the sync state is shown on the resource. The configuration is described [here].(usage/configuration.md)

### Transformer
Is used to transform a k8s resource into something which can be persisted in a storage. See [transformers](transformers/README.md).
