// SPDX-FileCopyrightText: 2023 SAP SE or an SAP affiliate company and Gardener contributors.
//
// SPDX-License-Identifier: Apache-2.0

package config

import "github.com/gardener/k8syncer/pkg/utils"

type DeepCopyAble[T any] interface {
	DeepCopy() T
}

var _ DeepCopyAble[*K8SyncerConfiguration] = &K8SyncerConfiguration{}

func (in *K8SyncerConfiguration) DeepCopy() *K8SyncerConfiguration {
	if in == nil {
		return nil
	}
	return &K8SyncerConfiguration{
		SyncConfigs:        deepCopySlice[*SyncConfig](in.SyncConfigs),
		StorageDefinitions: deepCopySlice[*StorageDefinition](in.StorageDefinitions),
	}
}

func (in *SyncConfig) DeepCopy() *SyncConfig {
	if in == nil {
		return nil
	}
	return &SyncConfig{
		ID:          in.ID,
		Resource:    in.Resource.DeepCopy(),
		StorageRefs: deepCopySlice[*StorageReference](in.StorageRefs),
		State:       in.State.DeepCopy(),
		Finalize:    deepCopyBool(in.Finalize),
	}
}

func (in *ResourceSyncConfig) DeepCopy() *ResourceSyncConfig {
	if in == nil {
		return nil
	}
	return &ResourceSyncConfig{
		Namespace: in.Namespace,
		Group:     in.Group,
		Version:   in.Version,
		Kind:      in.Kind,
	}
}

func (in *StorageReference) DeepCopy() *StorageReference {
	if in == nil {
		return nil
	}
	return &StorageReference{
		Name:    in.Name,
		SubPath: in.SubPath,
	}
}

func (in *StorageDefinition) DeepCopy() *StorageDefinition {
	if in == nil {
		return nil
	}
	return &StorageDefinition{
		Name:             in.Name,
		Type:             in.Type,
		GitConfig:        in.GitConfig.DeepCopy(),
		FileSystemConfig: in.FileSystemConfig.DeepCopy(),
		MockConfig:       in.MockConfig.DeepCopy(),
	}
}

func (in *GitConfiguration) DeepCopy() *GitConfiguration {
	if in == nil {
		return nil
	}
	return &GitConfiguration{
		URL:       in.URL,
		Branch:    in.Branch,
		Exclusive: in.Exclusive,
		Auth:      in.Auth.DeepCopy(),
	}
}

func (in *GitRepoAuth) DeepCopy() *GitRepoAuth {
	if in == nil {
		return nil
	}
	return &GitRepoAuth{
		Type:           in.Type,
		Username:       in.Username,
		Password:       in.Password,
		PrivateKey:     in.PrivateKey,
		PrivateKeyFile: in.PrivateKeyFile,
	}
}

func (in *FileSystemConfiguration) DeepCopy() *FileSystemConfiguration {
	if in == nil {
		return nil
	}
	return &FileSystemConfiguration{
		NamespacePrefix:  in.NamespacePrefix,
		GVKNameSeparator: in.GVKNameSeparator,
		FileExtension:    in.FileExtension,
		RootPath:         in.RootPath,
		InMemory:         deepCopyBool(in.InMemory),
	}
}

func (in *MockConfiguration) DeepCopy() *MockConfiguration {
	if in == nil {
		return nil
	}
	return &MockConfiguration{
		LogPersisterCallsOnInfoLevel: in.LogPersisterCallsOnInfoLevel,
	}
}

func (in *StateConfiguration) DeepCopy() *StateConfiguration {
	if in == nil {
		return nil
	}
	return &StateConfiguration{
		Type:              in.Type,
		Verbosity:         in.Verbosity,
		StatusStateConfig: in.StatusStateConfig.DeepCopy(),
	}
}

func (in *StatusStateConfiguration) DeepCopy() *StatusStateConfiguration {
	if in == nil {
		return nil
	}
	return &StatusStateConfiguration{
		GenerationPath: in.GenerationPath,
		PhasePath:      in.PhasePath,
		DetailPath:     in.DetailPath,
	}
}

func deepCopySlice[T DeepCopyAble[T]](list []T) []T {
	res := make([]T, len(list))
	for i := range list {
		res[i] = list[i].DeepCopy()
	}
	return res
}

func deepCopyBool(in *bool) *bool {
	if in == nil {
		return nil
	}
	return utils.Ptr(*in)
}
