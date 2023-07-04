// SPDX-FileCopyrightText: 2023 SAP SE or an SAP affiliate company and Gardener contributors.
//
// SPDX-License-Identifier: Apache-2.0

package config

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/gardener/k8syncer/pkg/utils"
)

// validTestConfig returns a valid test config
// tests can then easily test invalid configuration by adapting the config
func validTestConfig() *K8SyncerConfiguration {
	res := &K8SyncerConfiguration{
		SyncConfigs: []*SyncConfig{
			{
				ID: "dummyWatcher",
				Resource: &ResourceSyncConfig{
					Version: "v1",
					Kind:    "Dummy",
				},
				StorageRefs: []*StorageReference{
					{
						Name: "myStorage",
					},
				},
			},
		},
		StorageDefinitions: []*StorageDefinition{
			{
				Name: "myStorage",
				Type: STORAGE_TYPE_MOCK,
			},
		},
	}
	Expect(res.Complete()).To(Succeed())
	return res
}

var _ = Describe("Validation", func() {

	Context("Test", func() {
		Expect(Validate(validTestConfig())).To(BeEmpty(), "validTestConfig() should return a valid configuration")
	})

	Context("K8SyncerConfiguration", func() {

		It("should reject sync configs which refer to undefined storage definitions", func() {
			cfg := validTestConfig()
			cfg.SyncConfigs[0].StorageRefs[0].Name = "undefinedStorage"
			allErrs := Validate(cfg)

			Expect(allErrs).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":     Equal(field.ErrorTypeInvalid),
					"Field":    Equal("syncConfigs[0].storageRefs[0].name"),
					"BadValue": BeEquivalentTo("undefinedStorage"),
				})),
			))
		})

	})

	Context("SyncConfigs", func() {

		It("should reject duplicate IDs in a list of SyncConfigs", func() {
			cfg := validTestConfig()
			cfg.SyncConfigs = append(cfg.SyncConfigs, cfg.SyncConfigs[0].DeepCopy())
			cfg.SyncConfigs[1].StorageRefs[0] = &StorageReference{
				Name: "copy",
			}
			cfg.StorageDefinitions = append(cfg.StorageDefinitions, cfg.StorageDefinitions[0].DeepCopy())
			cfg.StorageDefinitions[1].Name = "copy"
			allErrs := Validate(cfg)

			Expect(allErrs).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeDuplicate),
					"Field": Equal("syncConfigs[1].id"),
				})),
			))
		})

		It("should reject a sync config with an empty ID or storage reference", func() {
			cfg := validTestConfig()
			cfg.SyncConfigs[0].ID = ""
			cfg.SyncConfigs[0].StorageRefs = nil
			allErrs := Validate(cfg)

			Expect(allErrs).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("syncConfigs[0].id"),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("syncConfigs[0].storageRefs"),
				})),
			))
		})

		It("should reject a sync config with an invalid ID", func() {
			cfg := validTestConfig()
			cfg.SyncConfigs[0].ID = "?"
			allErrs := Validate(cfg)

			Expect(allErrs).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("syncConfigs[0].id"),
				})),
			))
		})

		It("should reject conflicting resource syncs (same namespace)", func() {
			cfg := validTestConfig()
			cfg.SyncConfigs[0].Resource.Namespace = "foo"
			cfg.SyncConfigs = append(cfg.SyncConfigs, cfg.SyncConfigs[0].DeepCopy())
			cfg.SyncConfigs[1].ID = "copy"
			allErrs := Validate(cfg)

			Expect(allErrs).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeForbidden),
					"Field": Equal("syncConfigs[1]"),
				})),
			))
		})

		It("should reject conflicting resource syncs (one namespaced, one all namespaces)", func() {
			cfg := validTestConfig()
			cfg.SyncConfigs = append(cfg.SyncConfigs, cfg.SyncConfigs[0].DeepCopy())
			cfg.SyncConfigs[1].ID = "copy"
			cfg.SyncConfigs[1].Resource.Namespace = "foo"
			allErrs := Validate(cfg)

			Expect(allErrs).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeForbidden),
					"Field": Equal("syncConfigs[1]"),
				})),
			))
		})

		It("should reject sync configurations with nested base paths (host filesystem)", func() {
			cfg := validTestConfig()
			cfg.SyncConfigs[0].StorageRefs[0].Name = "sharedHost"
			cfg.SyncConfigs = append(cfg.SyncConfigs, &SyncConfig{
				ID: "asdf",
				Resource: &ResourceSyncConfig{
					Version: "v1",
					Kind:    "Dummy2",
				},
				StorageRefs: []*StorageReference{
					{
						Name:    "sharedHost",
						SubPath: "foo/bar",
					},
				},
			})
			cfg.StorageDefinitions[0] = &StorageDefinition{
				Name: "sharedHost",
				Type: STORAGE_TYPE_FILESYSTEM,
				FileSystemConfig: &FileSystemConfiguration{
					RootPath: "/",
					InMemory: utils.Ptr(false),
				},
			}
			Expect(cfg.Complete()).To(Succeed())
			allErrs := Validate(cfg)

			Expect(allErrs).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":   Equal(field.ErrorTypeForbidden),
					"Field":  Equal("syncConfigs[1].storageRefs[0]"),
					"Detail": ContainSubstring("parent base path '/'"),
				})),
			))
		})

	})

	Context("StorageDefinitions", func() {

		It("should reject duplicate names in a list of StorageDefinitions", func() {
			cfg := validTestConfig()
			cfg.StorageDefinitions = append(cfg.StorageDefinitions, cfg.StorageDefinitions[0])
			allErrs := Validate(cfg)

			Expect(allErrs).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeDuplicate),
					"Field": Equal("storageDefinitions[1].name"),
				})),
			))
		})

		It("should reject storage definitions with an empty or invalid name", func() {
			cfg := validTestConfig()
			cfg.StorageDefinitions = append(cfg.StorageDefinitions, cfg.StorageDefinitions[0].DeepCopy())
			cfg.StorageDefinitions[0].Name = ""
			cfg.StorageDefinitions[1].Name = "?"
			allErrs := Validate(cfg)

			Expect(allErrs).To(ContainElements(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("storageDefinitions[0].name"),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("storageDefinitions[1].name"),
				})),
			))
		})

		Context("GitRepoConfig", func() {

			It("should reject an empty repo configuration", func() {
				cfg := validTestConfig()
				cfg.StorageDefinitions = append(cfg.StorageDefinitions, &StorageDefinition{
					Name:      "myGit",
					Type:      STORAGE_TYPE_GIT,
					GitConfig: &GitConfiguration{},
				})
				allErrs := Validate(cfg)

				Expect(allErrs).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal("storageDefinitions[1].gitConfig.url"),
					})),
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal("storageDefinitions[1].gitConfig.auth"),
					})),
				))
			})

			It("should accept a valid repo configuration", func() {
				cfg := validTestConfig()
				cfg.StorageDefinitions = append(cfg.StorageDefinitions, &StorageDefinition{
					Name: "myGit",
					Type: STORAGE_TYPE_GIT,
					GitConfig: &GitConfiguration{
						URL: "https://github.com/example/example.git",
						Auth: &GitRepoAuth{
							Type:     GIT_AUTH_USERNAME_PASSWORD,
							Username: "foo",
							Password: "bar",
						},
					},
				})
				allErrs := Validate(cfg)

				Expect(allErrs).To(BeEmpty())
			})

			It("should reject duplicate repo URLs", func() {
				cfg := validTestConfig()
				gitCfg := &StorageDefinition{
					Name: "myGit",
					Type: STORAGE_TYPE_GIT,
					GitConfig: &GitConfiguration{
						URL: "https://github.com/example/example.git",
						Auth: &GitRepoAuth{
							Type:     GIT_AUTH_USERNAME_PASSWORD,
							Username: "foo",
							Password: "bar",
						},
					},
				}
				gitCfg2 := gitCfg.DeepCopy()
				gitCfg2.Name = "myGit2"
				cfg.StorageDefinitions = append(cfg.StorageDefinitions, gitCfg, gitCfg2)
				allErrs := Validate(cfg)

				Expect(allErrs).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeDuplicate),
						"Field": Equal("storageDefinitions[2].gitConfig.url"),
					})),
				))
			})

			Context("GitRepoAuth", func() {

				Context("AuthType", func() {

					It("should reject an empty auth type", func() {
						cfg := &GitRepoAuth{}
						v := newValidator()
						allErrs := v.validateGitRepoAuth(cfg, field.NewPath("auth"))

						Expect(allErrs).To(ConsistOf(
							PointTo(MatchFields(IgnoreExtras, Fields{
								"Type":  Equal(field.ErrorTypeRequired),
								"Field": Equal("auth.type"),
							})),
						))
					})

					It("should reject an unknown auth type", func() {
						cfg := &GitRepoAuth{
							Type: GitAuthenticationType("foo"),
						}
						v := newValidator()
						allErrs := v.validateGitRepoAuth(cfg, field.NewPath("auth"))

						Expect(allErrs).To(ConsistOf(
							PointTo(MatchFields(IgnoreExtras, Fields{
								"Type":  Equal(field.ErrorTypeNotSupported),
								"Field": Equal("auth.type"),
							})),
						))
					})

					It("should accept a valid config using username/password auth type", func() {
						cfg := &GitRepoAuth{
							Type:     GIT_AUTH_USERNAME_PASSWORD,
							Username: "foo",
							Password: "bar",
						}
						v := newValidator()
						allErrs := v.validateGitRepoAuth(cfg, field.NewPath("auth"))

						Expect(allErrs).To(BeEmpty())
					})

					It("should accept a valid config using ssh auth type", func() {
						cfg := &GitRepoAuth{
							Type:       GIT_AUTH_SSH,
							PrivateKey: "myPrivateKey",
						}
						v := newValidator()
						allErrs := v.validateGitRepoAuth(cfg, field.NewPath("auth"))

						Expect(allErrs).To(BeEmpty())
					})

				})

				Context("Username/Password", func() {

					It("should reject if username or password is missing", func() {
						cfg := &GitRepoAuth{}
						v := newValidator()
						allErrs := v.validateGitRepoAuthForUserPass(cfg, field.NewPath("auth"))

						Expect(allErrs).To(ConsistOf(
							PointTo(MatchFields(IgnoreExtras, Fields{
								"Type":  Equal(field.ErrorTypeRequired),
								"Field": Equal("auth.username"),
							})),
							PointTo(MatchFields(IgnoreExtras, Fields{
								"Type":  Equal(field.ErrorTypeRequired),
								"Field": Equal("auth.password"),
							})),
						))
					})

					It("should accept a valid configuration", func() {
						cfg := &GitRepoAuth{
							Username: "foo",
							Password: "bar",
						}
						v := newValidator()
						allErrs := v.validateGitRepoAuthForUserPass(cfg, field.NewPath("auth"))

						Expect(allErrs).To(BeEmpty())
					})

					It("should reject if privateKey or privateKeyFile is set", func() {
						cfg := &GitRepoAuth{
							Username:       "foo",
							Password:       "bar",
							PrivateKey:     "myPrivateKey",
							PrivateKeyFile: "myPrivateKeyFile",
						}
						v := newValidator()
						allErrs := v.validateGitRepoAuthForUserPass(cfg, field.NewPath("auth"))

						Expect(allErrs).To(ConsistOf(
							PointTo(MatchFields(IgnoreExtras, Fields{
								"Type":  Equal(field.ErrorTypeInvalid),
								"Field": Equal("auth.privateKey"),
							})),
							PointTo(MatchFields(IgnoreExtras, Fields{
								"Type":  Equal(field.ErrorTypeInvalid),
								"Field": Equal("auth.privateKeyFile"),
							})),
						))
					})

				})

				Context("SSH", func() {

					It("should reject if neither privateKey nor privateKeyFile is set", func() {
						cfg := &GitRepoAuth{}
						v := newValidator()
						allErrs := v.validateGitRepoAuthForSSH(cfg, field.NewPath("auth"))

						Expect(allErrs).To(ConsistOf(
							PointTo(MatchFields(IgnoreExtras, Fields{
								"Type":   Equal(field.ErrorTypeInvalid),
								"Field":  Equal("auth"),
								"Detail": And(ContainSubstring("'privateKey'"), ContainSubstring("'privateKeyFile'")),
							})),
						))
					})

					It("should reject if both privateKey and privateKeyFile are set", func() {
						cfg := &GitRepoAuth{
							PrivateKey:     "myPrivateKey",
							PrivateKeyFile: "myPrivateKeyFile",
						}
						v := newValidator()
						allErrs := v.validateGitRepoAuthForSSH(cfg, field.NewPath("auth"))

						Expect(allErrs).To(ConsistOf(
							PointTo(MatchFields(IgnoreExtras, Fields{
								"Type":   Equal(field.ErrorTypeInvalid),
								"Field":  Equal("auth"),
								"Detail": And(ContainSubstring("'privateKey'"), ContainSubstring("'privateKeyFile'")),
							})),
						))
					})

					It("should accept a valid configuration (privateKey)", func() {
						cfg := &GitRepoAuth{
							PrivateKey: "myPrivateKey",
						}
						v := newValidator()
						allErrs := v.validateGitRepoAuthForSSH(cfg, field.NewPath("auth"))

						Expect(allErrs).To(BeEmpty())
					})

					It("should accept a valid configuration (privateKeyFile)", func() {
						cfg := &GitRepoAuth{
							PrivateKeyFile: "myPrivateKeyFile",
						}
						v := newValidator()
						allErrs := v.validateGitRepoAuthForSSH(cfg, field.NewPath("auth"))

						Expect(allErrs).To(BeEmpty())
					})

					It("should reject if username or password is set", func() {
						cfg := &GitRepoAuth{
							Username:   "foo",
							Password:   "bar",
							PrivateKey: "myPrivateKey",
						}
						v := newValidator()
						allErrs := v.validateGitRepoAuthForSSH(cfg, field.NewPath("auth"))

						Expect(allErrs).To(ConsistOf(
							PointTo(MatchFields(IgnoreExtras, Fields{
								"Type":  Equal(field.ErrorTypeInvalid),
								"Field": Equal("auth.username"),
							})),
							PointTo(MatchFields(IgnoreExtras, Fields{
								"Type":  Equal(field.ErrorTypeInvalid),
								"Field": Equal("auth.password"),
							})),
						))
					})

				})

			})

		})

	})

})
