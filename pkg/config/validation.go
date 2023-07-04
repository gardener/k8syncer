// SPDX-FileCopyrightText: 2023 SAP SE or an SAP affiliate company and Gardener contributors.
//
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"fmt"
	"path/filepath"
	"regexp"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

// only letters, digits, and '-' and '_'
// starts with a letter
// '-' and '_' must always be followed by a letter or digit
var nameRegex = regexp.MustCompile("^[a-zA-Z]([-_]?[a-zA-Z0-9])*$")

type validator struct {
	storageDefs           map[string]*StorageDefinition
	sharedHostFsBasePaths sets.Set[string]
}

func newValidator() *validator {
	return &validator{
		// storageDefs contains a mapping from name to the storage definition
		// this is helpful for validating the storage references in the sync configs
		storageDefs: map[string]*StorageDefinition{},
		// all storage definitions which internally use a filesystem persister and have inMemory set to 'false' share the host system's filesystem
		// each mock persister always uses its own in-memory filesystem, independent of inMemory
		sharedHostFsBasePaths: sets.New[string](),
	}
}

func Validate(cfg *K8SyncerConfiguration) field.ErrorList {
	allErrs := field.ErrorList{}

	if cfg == nil {
		allErrs = append(allErrs, field.Required(field.NewPath(""), "K8Syncer Configuration must not be empty"))
		return allErrs
	}

	v := newValidator()
	allErrs = append(allErrs, v.validateStorageDefinitions(cfg.StorageDefinitions, field.NewPath("storageDefinitions"))...)
	allErrs = append(allErrs, v.validateSyncConfigs(cfg.SyncConfigs, field.NewPath("syncConfigs"))...)

	return allErrs
}

// needs to be called AFTER validateStorageDefinitions, as it depends on v.storageDefs being set
func (v *validator) validateSyncConfigs(syncConfigs []*SyncConfig, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if len(syncConfigs) == 0 {
		allErrs = append(allErrs, field.Required(fldPath, "no sync configs provided"))
	}

	// if there are multiple sync configs configured for the same resource, the same namespace and the same storages, this could cause write conflicts
	// to detect these configurations, synced resource GroupVersionKinds are mapped to namespaces (in which they are watched) and these are mapped to storage references
	avoidSyncConflicts := map[schema.GroupVersionKind]map[string]sets.Set[string]{}
	syncConfigIDs := sets.New[string]()
	for idx, sc := range syncConfigs {
		curPath := fldPath.Index(idx)

		// validate that IDs are unique
		if syncConfigIDs.Has(sc.ID) {
			allErrs = append(allErrs, field.Duplicate(curPath.Child("id"), sc.ID))
		}
		syncConfigIDs.Insert(sc.ID)

		// validate that there won't be any sync conflicts
		if sc.Resource != nil {
			srNames := []string{}
			for _, elem := range sc.StorageRefs {
				if elem == nil {
					continue
				}
				srNames = append(srNames, elem.Name)
			}
			gvk := schema.GroupVersionKind{Group: sc.Resource.Group, Version: sc.Resource.Version, Kind: sc.Resource.Kind}
			syncedNamespaces, ok := avoidSyncConflicts[gvk]
			if ok {
				allNs, allOk := syncedNamespaces[""]
				var ns sets.Set[string]
				nsOk := false
				if sc.Resource.Namespace != "" {
					ns, nsOk = syncedNamespaces[sc.Resource.Namespace]
				}
				if allOk {
					// there is another sync defined for the same resource across all namespaces => storage references must be disjunct
					if allNs.HasAny(srNames...) {
						allErrs = append(allErrs, field.Forbidden(curPath, "conflicting sync (same resource, all namespaces) with overlapping storage references found"))
					}
				}
				if nsOk {
					// there is another sync defined for the same resource in the same namespace => storage references must be disjunct
					for _, sr := range sc.StorageRefs {
						if sr == nil {
							continue
						}
						if ns.Has(sr.Name) {
							allErrs = append(allErrs, field.Forbidden(curPath, "conflicting sync (same resource, same namespace) with overlapping storage references found"))
							break
						}
					}
				}
			} else {
				avoidSyncConflicts[gvk] = map[string]sets.Set[string]{}
			}
			if _, ok := avoidSyncConflicts[gvk][sc.Resource.Namespace]; !ok {
				avoidSyncConflicts[gvk][sc.Resource.Namespace] = sets.New[string]()
			}
			avoidSyncConflicts[gvk][sc.Resource.Namespace].Insert(srNames...)
		}

		// validate syncConfig
		allErrs = append(allErrs, v.validateSyncConfig(sc, curPath)...)
	}

	return allErrs
}

// storageDefNames is expected to be an empty set, which is filled with the names of the git repos by this function
func (v *validator) validateStorageDefinitions(storageDefs []*StorageDefinition, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if len(storageDefs) == 0 {
		allErrs = append(allErrs, field.Required(fldPath, "no storage definitions provided"))
	}

	gitRepoURLs := sets.New[string]()

	for idx, sd := range storageDefs {
		curPath := fldPath.Index(idx)

		// validate that IDs are unique
		if _, ok := v.storageDefs[sd.Name]; ok {
			allErrs = append(allErrs, field.Duplicate(curPath.Child("name"), sd.Name))
		} else {
			v.storageDefs[sd.Name] = sd
		}

		// validate storage definition
		allErrs = append(allErrs, v.validateStorageDefinition(sd, curPath, gitRepoURLs)...)
	}

	return allErrs
}

func (v *validator) validateStorageDefinition(sd *StorageDefinition, fldPath *field.Path, gitRepoURLs sets.Set[string]) field.ErrorList {
	allErrs := field.ErrorList{}

	// validate that names are unique
	if sd.Name == "" {
		allErrs = append(allErrs, field.Required(fldPath.Child("name"), "storage definition name must not be empty"))
	} else if !nameRegex.MatchString(sd.Name) {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("name"), sd.Name, fmt.Sprintf("name must match regex %s", nameRegex.String())))
	}

	switch sd.Type {
	case STORAGE_TYPE_FILESYSTEM:
		allErrs = append(allErrs, v.validateFileSystemConfig(sd.FileSystemConfig, fldPath.Child("filesystemConfig"))...)
	case STORAGE_TYPE_GIT:
		allErrs = append(allErrs, v.validateGitRepoConfig(sd.GitConfig, fldPath.Child("gitConfig"), gitRepoURLs)...)
	case STORAGE_TYPE_MOCK:
		// nothing to do
	default:
		allErrs = append(allErrs, field.NotSupported(fldPath.Child("type"), sd.Type, []string{string(STORAGE_TYPE_FILESYSTEM), string(STORAGE_TYPE_GIT)}))
	}

	return allErrs
}

// storageDefNames is expected to contain the names of all defined git repos
func (v *validator) validateSyncConfig(syncConfig *SyncConfig, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if syncConfig == nil {
		allErrs = append(allErrs, field.Required(fldPath, "sync config must not be empty"))
		return allErrs
	}

	if syncConfig.ID == "" {
		allErrs = append(allErrs, field.Required(fldPath.Child("id"), "ID must not be empty"))
	} else if !nameRegex.MatchString(syncConfig.ID) {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("id"), syncConfig.ID, fmt.Sprintf("ID must match regex %s", nameRegex.String())))
	}

	allErrs = append(allErrs, v.validateStorageReferences(syncConfig.StorageRefs, fldPath.Child("storageRefs"))...)
	allErrs = append(allErrs, v.validateResourceSyncConfig(syncConfig.Resource, fldPath.Child("resource"))...)
	allErrs = append(allErrs, v.validateStateConfiguration(syncConfig.State, fldPath.Child("state"))...)

	if syncConfig.Finalize == nil {
		allErrs = append(allErrs, field.Required(fldPath.Child("finalize"), "finalize is required, but it should have been defaulted, check coding"))
	}

	return allErrs
}

func (v *validator) validateFileSystemConfig(fsConfig *FileSystemConfiguration, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if fsConfig == nil {
		allErrs = append(allErrs, field.Required(fldPath, "filesystem configuration must not be empty"))
		return allErrs
	}

	if fsConfig.RootPath == "" {
		allErrs = append(allErrs, field.Required(fldPath.Child("rootPath"), "root path must not be empty"))
	}
	if fsConfig.InMemory == nil {
		allErrs = append(allErrs, field.Required(fldPath.Child("inMemory"), "inMemory is required, but it should have been defaulted, check coding"))
	}

	return allErrs
}

func (v *validator) validateStorageReferences(refs []*StorageReference, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if len(refs) == 0 {
		allErrs = append(allErrs, field.Required(fldPath, "storage references must not be empty"))
	}

	for idx, ref := range refs {
		curPath := fldPath.Index(idx)

		if ref == nil {
			allErrs = append(allErrs, field.Required(curPath, "storage reference must not be empty"))
			continue
		}

		if ref.Name == "" {
			allErrs = append(allErrs, field.Required(curPath.Child("name"), "storage reference name must not be empty"))
		}

		// validate that only existing storage definitions are referenced and that base paths on shared filesystems are not nested
		sd, ok := v.storageDefs[ref.Name]
		if ok {
			if sd.FileSystemConfig != nil && sd.Type != STORAGE_TYPE_MOCK {
				basePath := filepath.Join(sd.FileSystemConfig.RootPath, ref.SubPath)
				if basePath == "" {
					basePath = "/"
				}
				if !*sd.FileSystemConfig.InMemory {
					// host filesystem is always shared
					for parent := filepath.Dir(basePath); ; parent = filepath.Dir(parent) {
						if v.sharedHostFsBasePaths.Has(parent) {
							allErrs = append(allErrs, field.Forbidden(curPath, fmt.Sprintf("base paths (storage rootPath + reference subPath) must not be nested for shared filesystems (shared host filesystem), found parent base path '%s'", parent)))
						}
						if parent == "/" || parent == "." {
							break
						}
					}
					v.sharedHostFsBasePaths.Insert(basePath)
				}
			}
		} else {
			allErrs = append(allErrs, field.Invalid(curPath.Child("name"), ref.Name, "storage definition with this name does not exist"))
		}
	}

	return allErrs
}

func (v *validator) validateGitRepoConfig(repoConfig *GitConfiguration, fldPath *field.Path, gitRepoURLs sets.Set[string]) field.ErrorList {
	allErrs := field.ErrorList{}

	if repoConfig == nil {
		allErrs = append(allErrs, field.Required(fldPath, "repo config must not be empty"))
		return allErrs
	}

	if repoConfig.URL == "" {
		allErrs = append(allErrs, field.Required(fldPath.Child("url"), "repository url must not be empty"))
	} else {
		if gitRepoURLs.Has(repoConfig.URL) {
			allErrs = append(allErrs, field.Duplicate(fldPath.Child("url"), repoConfig.URL))
		}
		gitRepoURLs.Insert(repoConfig.URL)
	}

	allErrs = append(allErrs, v.validateGitRepoAuth(repoConfig.Auth, fldPath.Child("auth"))...)

	return allErrs
}

func (v *validator) validateResourceSyncConfig(resourceSyncConfig *ResourceSyncConfig, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if resourceSyncConfig == nil {
		return append(allErrs, field.Required(fldPath, "resource sync config must not be empty"))
	}

	if resourceSyncConfig.Kind == "" {
		allErrs = append(allErrs, field.Required(fldPath.Child("kind"), "resource kind must not be empty"))
	}
	if resourceSyncConfig.Version == "" {
		allErrs = append(allErrs, field.Required(fldPath.Child("version"), "resource version must not be empty"))
	}

	return allErrs
}

func (v *validator) validateStateConfiguration(sdCfg *StateConfiguration, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	if sdCfg == nil || sdCfg.Type == STATE_TYPE_NONE {
		return allErrs
	}

	switch sdCfg.Verbosity {
	case STATE_VERBOSITY_GENERATION:
	case STATE_VERBOSITY_PHASE:
	case STATE_VERBOSITY_DETAIL:
	default:
		allErrs = append(allErrs, field.NotSupported(fldPath.Child("verbosity"), string(sdCfg.Verbosity), []string{string(STATE_VERBOSITY_GENERATION), string(STATE_VERBOSITY_PHASE), string(STATE_VERBOSITY_DETAIL)}))
	}

	switch sdCfg.Type {
	case STATE_TYPE_NONE:
	case STATE_TYPE_ANNOTATION:
	case STATE_TYPE_STATUS:
		allErrs = append(allErrs, v.validateStatusStateConfiguration(sdCfg.StatusStateConfig, sdCfg.Verbosity, fldPath.Child("statusConfig"))...)
	default:
		allErrs = append(allErrs, field.NotSupported(fldPath.Child("type"), string(sdCfg.Type), []string{string(STATE_TYPE_NONE), string(STATE_TYPE_ANNOTATION), string(STATE_TYPE_STATUS)}))
	}

	return allErrs
}

func (v *validator) validateStatusStateConfiguration(ssCfg *StatusStateConfiguration, verbosity StateVerbosity, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	if ssCfg == nil {
		allErrs = append(allErrs, field.Required(fldPath, "status state configuration must not be empty for configured state type"))
	}

	switch verbosity {
	case STATE_VERBOSITY_DETAIL:
		if ssCfg.DetailPath == "" {
			allErrs = append(allErrs, field.Required(fldPath.Child("detailPath"), "detail path is required for the configured verbosity"))
		}
		fallthrough
	case STATE_VERBOSITY_PHASE:
		if ssCfg.PhasePath == "" {
			allErrs = append(allErrs, field.Required(fldPath.Child("phasePath"), "phase path is required for the configured verbosity"))
		}
		fallthrough
	case STATE_VERBOSITY_GENERATION:
		if ssCfg.GenerationPath == "" {
			allErrs = append(allErrs, field.Required(fldPath.Child("generationPath"), "generation path is required for the configured verbosity"))
		}
	}

	return allErrs
}

func (v *validator) validateGitRepoAuth(auth *GitRepoAuth, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if auth == nil {
		return append(allErrs, field.Required(fldPath, "git authentication configuration must not be empty"))
	}

	if auth.Type == "" {
		allErrs = append(allErrs, field.Required(fldPath.Child("type"), "git authentication type must not be empty"))
		return allErrs
	}

	switch auth.Type {
	case GIT_AUTH_USERNAME_PASSWORD:
		allErrs = append(allErrs, v.validateGitRepoAuthForUserPass(auth, fldPath)...)
	case GIT_AUTH_SSH:
		allErrs = append(allErrs, v.validateGitRepoAuthForSSH(auth, fldPath)...)
	default:
		allErrs = append(allErrs, field.NotSupported(fldPath.Child("type"), string(auth.Type), []string{string(GIT_AUTH_USERNAME_PASSWORD), string(GIT_AUTH_SSH)}))
	}

	return allErrs
}

func (v *validator) validateGitRepoAuthForUserPass(auth *GitRepoAuth, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if auth.Username == "" {
		allErrs = append(allErrs, field.Required(fldPath.Child("username"), "username is required for the chosen authentication type"))
	}
	if auth.Password == "" {
		allErrs = append(allErrs, field.Required(fldPath.Child("password"), "password is required for the chosen authentication type"))
	}

	if auth.PrivateKey != "" {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("privateKey"), auth.PrivateKey, "privateKey must not be set for the chosen authentication type"))
	}
	if auth.PrivateKeyFile != "" {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("privateKeyFile"), auth.PrivateKeyFile, "privateKeyFile must not be set for the chosen authentication type"))
	}

	return allErrs
}

func (v *validator) validateGitRepoAuthForSSH(auth *GitRepoAuth, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if (auth.PrivateKey == "" && auth.PrivateKeyFile == "") || (auth.PrivateKey != "" && auth.PrivateKeyFile != "") {
		allErrs = append(allErrs, field.Invalid(fldPath, auth, "exactly one of 'privateKey' and 'privateKeyFile' must be set for the chosen authentication type"))
	}

	if auth.Username != "" {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("username"), auth.Username, "username must not be set for the chosen authentication type"))
	}
	if auth.Password != "" {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("password"), auth.Password, "password must not be set for the chosen authentication type"))
	}

	return allErrs
}
