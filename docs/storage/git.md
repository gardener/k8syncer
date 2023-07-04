# Git Storage

The `git` storage allows syncing k8s resources into a git repository. It requires `gitConfig` to be set. Additionally, because the git persister internally uses a filesystem persister, specifying `filesystemConfig` is also possible, but not required.

The `subPath` field in the storage reference is expected to be a directory path without leading or trailing `/`, e.g. `a/b/c`. If empty, it will use the configured filesystem's root path (`filesystemConfig.rootPath`).

## Configuration

```yaml
- name: myStorage
  type: git
  filesystemConfig: ... # optional
  gitConfig:
    url: "https://github.com/example/example.git"
    branch: master # optional
    exclusive: true # optional
    auth:
      type: username_password
      username: my_user
      password: my_password
      privateKey: |
        -----BEGIN RSA PRIVATE KEY-----
        ...
      privateKeyFile: /etc/ssh/foo/cert.key
```

- `url` - The git repo URL. To avoid problems with concurrency, there must only be one git storage definition for a given URL, so it has to be unique.
- `branch` - The branch to which the changes should be pushed. Defaults to `master` if not set.
- `exclusive` - If set to true, it is assumed that no one else pushes to the specified branch while the controller is running. This means the controller will pull the repository only during checkout, and if pushing a change fails. If false, the controller will perform a pull before each operation, which slows it down significally. It is strongly recommended to reserve the branch for the K8Syncer controller and set this to true for best performance. Defaults to `false` if not set.
- `auth` - The authentication information for the git repository.
  - `type` - The authentication type. Must be one of `username_password` or `ssh`.
    - Note that the `username_password` type can also be used for authentication via access token. For github.com, put the access token under `password` and _set an arbitrary, non-empty username_. Other git repositories might potentially use the username field for this.
    - For `username_password`, the `username` and `password` fields have to be set.
    - For `ssh`, either `privateKey` or `privateKeyFile` has to be set, `password` is optional.
  - `username` - The username. Ignored if the type is not `username_password`.
  - `password` - The password, if the type is `username_password`. If the type is `ssh`, the decryption key for the SSH private key must be specified here, unless it is not encrypted.
  - `privateKey` - The SSH private key as inline text. If encrypted, the decryption key must be provided via the `password` field. For type `ssh`, either `privateKey` or `privateKeyFile` must be set. The field is ignored for type `username_password`.
  - `privateKeyFile` - The path to the file containing the SSH private key. If the key is encrypted, the decryption key must be provided via the `password` field. For type `ssh`, either `privateKey` or `privateKeyFile` must be set. The field is ignored for type `username_password`.

Optionally, a [filesystem configuration](filesystem.md) can be provided. If not, the filesystem default values (described in the linked documentation) are used, except that `inMemory` defaults to `true` in this case.


## Limitations

It is recommended to use this storage type only for resources which are changed rarely. Frequent changes could cause problems with rate limits on the git repository.

Furthermore, each configured git repository will be checked out and stored locally, by default in memory. Huge repositories or many configured git storages can therefore significantly increase memory usage.

If the host filesystem is used to store the checked-out git repositories, the [nested base path limitation from the filesystem storage](filesystem.md#limitations) applies too.


## Examples

The examples are the same as the [filesystem storage examples](filesystem.md#examples), except that they are packed into a commit (usually with commit message `update <kind>.<version>.<group> <namespace>/<name>`) and pushed to the repository afterwards.
