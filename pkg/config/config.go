// SPDX-FileCopyrightText: 2023 SAP SE or an SAP affiliate company and Gardener contributors.
//
// SPDX-License-Identifier: Apache-2.0

package config

// K8SyncerConfiguration contains the K8Syncer configuration.
type K8SyncerConfiguration struct {
	SyncConfigs        []*SyncConfig        `json:"syncConfigs,omitempty"`
	StorageDefinitions []*StorageDefinition `json:"storageDefinitions,omitempty"`
}

type SyncConfig struct {
	// ID is a unique identifier.
	// It has no effect except for being included in the logs, so it allows to filter for outputs from a specific watcher,
	// which is useful if there are multiple sync configs defined which watch the same resource.
	ID string
	// Resource specifies which resource should be synced.
	Resource *ResourceSyncConfig `json:"resource,omitempty"`
	// StorageRefs reference the storage definitions.
	StorageRefs []*StorageReference `json:"storageRefs"`
	// State contains the state display information.
	// If set, the controller will show the sync state for the reconciled resource in the configured way.
	// This allows other controllers to only react on changes if the resource has been persisted.
	// If nil or set to type 'none', no state will be displayed.
	// +optional
	State *StateConfiguration `json:"state,omitempty"`
	// Finalize specifies whether or not to use a finalizer on the specified resource.
	// Note that without a finalizer, no sync state will be shown for deletion, as the resource could be gone immediately.
	// Defaults to true.
	Finalize *bool `json:"finalize,omitempty"`
}

type ResourceSyncConfig struct {
	// Namespace is the namespace from which resources should be synced.
	// Leave empty for cluster-scoped or to sync namespaced resources from all namespaces.
	Namespace string `json:"namespace"`
	// Group is the group of the resource to watch.
	// Example: 'apps' for k8s deployments, 'landscaper.gardener.cloud' for Landscaper resources
	// Empty for k8s core api resources such as namespaces and secrets.
	Group string `json:"group"`
	// Version is the apiversion of the resource to watch.
	// Example: 'v1', 'v1alpha1'
	Version string `json:"version"`
	// Kind is the kind of the resource to watch.
	// Example: 'Deployment', 'Secret'
	Kind string `json:"kind"`
}

type StorageReference struct {
	// Name is the name of the storage definition this reference refers to.
	Name string `json:"name"`
	// SubPath is the path from the storage option's root element to the folder which should be used as root directory for the stored resources.
	// Leave empty for top-level.
	SubPath string `json:"subPath"`
}

type StorageDefinition struct {
	// Name is name for this storage option, used for referencing it.
	// Must be unique.
	Name string `json:"name"`
	// Type is the type of storage.
	Type StorageDefinitionType `json:"type"`
	// GitConfig contains the configuration for persisting data to git repositories.
	// Must only be set when type is 'git'.
	// Using git requires FileSystemConfig to be set too. All values there are optional, except for RootPath,
	// which specifies the path on the local filesystem where the repository will be checked out to. It has to exist and be empty.
	// +optional
	GitConfig *GitConfiguration `json:"gitConfig,omitempty"`
	// FileSystemConfig is the configuration for persisting data to the filesystem.
	// Must be set when type is 'filesystem'. As some other Persisters are using an in-memory filesystem internally, it can be set for some other types too.
	// +optional
	FileSystemConfig *FileSystemConfiguration `json:"filesystemConfig,omitempty"`
	// MockConfig is the configuration for logging changes to the persistency instead of actually persisting them.
	// An additional FileSystemConfig can be provided, as the MockPersister works with an in-memory filesystem internally.
	// Opposed to the other Persisters, the configuration for the MockPersister is optional.
	// Must only be set when type is 'mock'.
	// +optional
	MockConfig *MockConfiguration `json:"mockConfig,omitempty"`
}

type StorageDefinitionType string

const (
	// STORAGE_TYPE_GIT is the storage type for a git repository.
	STORAGE_TYPE_GIT StorageDefinitionType = "git"
	// STORAGE_TYPE_FILESYSTEM is the storage type for a filesystem.
	STORAGE_TYPE_FILESYSTEM StorageDefinitionType = "filesystem"
	// STORAGE_TYPE_MOCK is for testing purposes
	STORAGE_TYPE_MOCK StorageDefinitionType = "mock"
)

// GitConfiguration defines a git repository
type GitConfiguration struct {
	// URL is the repository URL.
	URL string `json:"url"`
	// Branch is the branch which should be used.
	// Defaults to 'master'.
	// +optional
	Branch string `json:"branch"`
	// Exclusive specifies whether the provided repository is exclusively pushed to by the created GitPersister.
	// If true, the code assumes to be the only source of changes and never pulls from the repo,
	// except for when initializing and if an error during push occurs.
	// Do not set this to true, if anyone else pushes to the repository while the controller is running.
	// Defaults to false.
	// +optional
	Exclusive bool `json:"exclusive"`
	// Auth contains the auth information needed to push commits to the repository.
	Auth *GitRepoAuth `json:"auth,omitempty"`
}

// GitRepoAuth represents different possibilities to authenticate against a git repository
//
//	Auth via access token
//	  'password' has to be set
//	Auth via username/password
//	  'username' and 'password' have to be set
//	Auth via SSH
//	  either 'privateKey' or 'privateKeyFile' has to be set
//	  'password' has to be set if the specified private key contains an encrypted PEM block
type GitRepoAuth struct {
	// Type is the method used for authentication.
	// Valid values are:
	//   'username_password' for authentication via username and password (also used for access tokens)
	//   'ssh' for authentication via SSH
	// This field is evaluated in a case-insensitive way.
	Type GitAuthenticationType `json:"type"`
	// Username is the git username for authentication.
	// It is required for authentication via username/password and must not be set otherwise.
	// +optional
	Username string `json:"username"`
	// Password is either the password for username/password or the access token.
	// It is required for both cases and optional for authentication via SSH.
	// +optional
	Password string `json:"password"`
	// PrivateKey is the private key for authentication via SSH.
	// This field is for providing the key inline, for a file path use PrivateKeyFile instead.
	// Only one of PrivateKey and PrivateKeyFile must be set for authentication via SSH and none must be set for other auth methods.
	// +optional
	PrivateKey string `json:"privateKey"`
	// PrivateKeyFile is a path to a file containing the private key for authentication via SSH.
	// This field is for providing a file path, for an inline private key use PrivateKey instead.
	// Only one of PrivateKey and PrivateKeyFile must be set for authentication via SSH and none must be set for other auth methods.
	// +optional
	PrivateKeyFile string `json:"privateKeyFile"`
}

type GitAuthenticationType string

const (
	// GIT_AUTH_USERNAME_PASSWORD is the auth type for authentication via username and password.
	GIT_AUTH_USERNAME_PASSWORD GitAuthenticationType = "username_password"
	// GIT_AUTH_SSH is the auth type for authentication via SSH.
	GIT_AUTH_SSH GitAuthenticationType = "ssh"
)

type FileSystemConfiguration struct {
	// NamespacePrefix is the prefix used for namespace folders on the filesystem.
	// Defaults to 'ns_'
	// Example: namespace 'foo' => folder 'ns_foo'
	// +optional
	NamespacePrefix *string `json:"namespacePrefix"`
	// GVKNameSeparator is the separator between the GroupVersionKind and the resource name used in the filename.
	// Defaults to '_'
	// Example: Deployment 'foo' => filename 'deployments.v1.apps_foo.yaml'
	// +optional
	GVKNameSeparator *string `json:"gvrNameSeparator"`
	// FileExtension is the file extension used for the files.
	// May be specified with or without preceding '.'
	// Defaults to 'yaml'
	// +optional
	FileExtension *string `json:"fileExtension"`
	// RootPath specifies which path within the filesystem should be used as root folder.
	// The specified directory has to exist.
	RootPath string `json:"rootPath"`
	// InMemory makes the FileSystemPersister use an in-memory filesystem, if set to true.
	// Defaults to false for type 'filesystem' and to true for type 'git'.
	InMemory *bool `json:"inMemory,omitempty"`
}

type MockConfiguration struct {
	// LogPersisterCallsOnInfoLevel controls the log level for the Persister function calls.
	// They are always logged, but usually on Debug verbosity.
	// If set to true, this is switched to Info for this MockPersister.
	LogPersisterCallsOnInfoLevel bool `json:"logPersisterCallsOnInfoLevel"`
}

type StateConfiguration struct {
	// Type is the type of state display which should be used.
	// Supported values are
	//   'none' for no state display
	//   'status' for writing it into the resource's status
	//   'annotation' for writing it on the resource as annotations
	Type StateType `json:"type"`
	// Verbosity defines what is displayed as state.
	// Supported values are
	//   'generation' - only the last synced generation will be displayed
	//   'phase' - above + current phase
	//   'detail' - above + details in case of error
	Verbosity StateVerbosity `json:"verbosity"`
	// StatusStateConfig is the configuration required for storing the state in the resource's status.
	// It has to be set for type 'status'.
	// +optional
	StatusStateConfig *StatusStateConfiguration `json:"statusConfig,omitempty"`
}

type StatusStateConfiguration struct {
	// GenerationPath is the jsonpath to the field in the resource's status where the last observed generation should be stored.
	// Required for type 'status'.
	// +optional
	GenerationPath string `json:"generationPath"`
	// PhasePath is the jsonpath to the field in the resource's status where the current phase should be stored.
	// Required for type 'status' if verbosity includes phase, ignored otherwise.
	// +optional
	PhasePath string `json:"phasePath"`
	// DetailPath is the jsonpath to the field in the resource's status where details about errors should be stored.
	// Required for type 'status' if verbosity includes details, ignored otherwise.
	// +optional
	DetailPath string `json:"detailPath"`
}

type StateType string

const (
	// STATE_TYPE_NONE disables state displaying.
	STATE_TYPE_NONE StateType = "none"
	// STATE_TYPE_STATUS configures state display via the resource's status.
	STATE_TYPE_STATUS StateType = "status"
	// STATE_TYPE_ANNOTATION configures state display via annotations on the resource.
	STATE_TYPE_ANNOTATION StateType = "annotation"
)

type StateVerbosity string

const (
	// STATE_VERBOSITY_GENERATION means that only the last synced generation is displayed.
	STATE_VERBOSITY_GENERATION StateVerbosity = "generation"
	// STATE_VERBOSITY_PHASE means that last synced generation and phase are displayed.
	STATE_VERBOSITY_PHASE StateVerbosity = "phase"
	// STATE_VERBOSITY_DETAIL means that last synced generation, phase, and details are displayed.
	STATE_VERBOSITY_DETAIL StateVerbosity = "detail"
)
