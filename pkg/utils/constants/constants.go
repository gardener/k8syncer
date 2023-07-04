// SPDX-FileCopyrightText: 2023 SAP SE or an SAP affiliate company and Gardener contributors.
//
// SPDX-License-Identifier: Apache-2.0

package constants

// Logging is a helper struct for organizing logging constants.
// This looks ugly, but unfortunately, it's the only way to access the constants in a 'Logging.<key>' way
// without having to move them into their own package (which would need an extra import).
var Logging = struct {
	CALL_EXISTS_MSG                string
	CALL_GET_MSG                   string
	CALL_PERSIST_MSG               string
	CALL_PERSIST_DATA_MSG          string
	CALL_DELETE_MSG                string
	CALL_EXISTS_FINISHED_MSG       string
	CALL_GET_FINISHED_MSG          string
	CALL_PERSIST_FINISHED_MSG      string
	CALL_PERSIST_DATA_FINISHED_MSG string
	CALL_DELETE_FINISHED_MSG       string

	KEY_ERROR                  string
	KEY_DATA_EXISTS            string
	KEY_RESOURCE_NAME          string
	KEY_RESOURCE_NAMESPACE     string
	KEY_RESOURCE_GROUP         string
	KEY_RESOURCE_VERSION       string
	KEY_RESOURCE_RESOURCE      string
	KEY_RESOURCE_KIND          string
	KEY_LOG_ID                 string
	KEY_ERROR_OCCURRED         string
	KEY_PERSIST_DATA_IS_DELETE string
	KEY_RESOURCE_STORAGE       string
	KEY_RESOURCE_STORAGE_ID    string
	KEY_WATCHED_NAMESPACE      string
	KEY_EVENT_TYPE             string
	KEY_ID                     string
	KEY_DATA                   string
	KEY_PATH                   string
	KEY_REQUEUE_AFTER          string
	KEY_REQUEUE_COUNT          string
	KEY_DUE_TIME               string
	KEY_STATE_DISPLAY          string
	KEY_STATE_VERBOSITY        string
	KEY_CONFIGURED_STORAGES    string
}{
	CALL_EXISTS_MSG:                "Call to Exists",
	CALL_GET_MSG:                   "Call to Get",
	CALL_PERSIST_MSG:               "Call to Persist",
	CALL_PERSIST_DATA_MSG:          "Call to PersistData",
	CALL_DELETE_MSG:                "Call to Delete",
	CALL_EXISTS_FINISHED_MSG:       "Exists returned",
	CALL_GET_FINISHED_MSG:          "Get returned",
	CALL_PERSIST_FINISHED_MSG:      "Persist returned",
	CALL_PERSIST_DATA_FINISHED_MSG: "PersistData returned",
	CALL_DELETE_FINISHED_MSG:       "Delete returned",

	KEY_ERROR:                  "error",
	KEY_DATA_EXISTS:            "dataExists",
	KEY_RESOURCE_NAME:          "name",
	KEY_RESOURCE_NAMESPACE:     "namespace",
	KEY_RESOURCE_GROUP:         "group",
	KEY_RESOURCE_VERSION:       "version",
	KEY_RESOURCE_RESOURCE:      "resource",
	KEY_RESOURCE_KIND:          "kind",
	KEY_LOG_ID:                 "logID",
	KEY_ERROR_OCCURRED:         "errorOccurred",
	KEY_PERSIST_DATA_IS_DELETE: "persistDataIsDelete",
	KEY_RESOURCE_STORAGE:       "storage",
	KEY_RESOURCE_STORAGE_ID:    "storageID",
	KEY_WATCHED_NAMESPACE:      "watchedNamespace",
	KEY_EVENT_TYPE:             "event",
	KEY_ID:                     "id",
	KEY_DATA:                   "data",
	KEY_PATH:                   "path",
	KEY_REQUEUE_AFTER:          "requeueAfter",
	KEY_REQUEUE_COUNT:          "requeueCount",
	KEY_DUE_TIME:               "dueTime",
	KEY_STATE_DISPLAY:          "stateDisplay",
	KEY_STATE_VERBOSITY:        "stateVerbosity",
	KEY_CONFIGURED_STORAGES:    "configuredStorages",
}

type k8syncerContextKey string

const (
	K8SYNCER_GROUP                    = "k8syncer.gardener.cloud"
	ANNOTATION_LAST_SYNCED_GENERATION = "state." + K8SYNCER_GROUP + "/lastSyncedGeneration"
	ANNOTATION_PHASE                  = "state." + K8SYNCER_GROUP + "/phase"
	ANNOTATION_DETAIL                 = "state." + K8SYNCER_GROUP + "/detail"
	K8SYNCER_FINALIZER                = "finalizer." + K8SYNCER_GROUP

	CONTEXT_KEY_LOGGING_DATA k8syncerContextKey = "logging_data"
)

type EventType string

const (
	EVENT_TYPE_ADD    EventType = "add"
	EVENT_TYPE_UPDATE EventType = "update"
	EVENT_TYPE_DELETE EventType = "delete"
)
