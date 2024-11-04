/*
Copyright 2024.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type BackupStorageType string

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// EtcdBackupSpec defines the desired state of EtcdBackup.
type EtcdBackupSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Specific Backup Etcd Endpoints
	EtcdUrl string `json:"etcdUrl"`

	// Storage Type: s3 or oss
	StorageType BackupStorageType `json:"storageType"`

	//Backup Source
	BackupSource `json:",inline"`
}

// BackupSource contains the supported backup sources
type BackupSource struct {
	// S3 defines the s3 backup source spec
	S3 *S3BackupSource `json:"s3,omitempty"`
	// OSS defines the oss backup source spec
	OSS *OSSBackupSource `json:"OSS,omitempty"`
}

// S3BackupSource provides the spec how to store backups on s3
type S3BackupSource struct {
	// Path is the full s3 path where the backup is saved.
	// The format of the path must be: "<s3-bucket-name>/<path-to-backup-file>"
	// e.g. "mybucket/myetcd.backup"
	Path string `json:"path"`

	// The name of the secret object that stores the credential which will be used
	// to access s3
	//
	// The secret must contain the following keys/fields:
	//  accessKeyID
	//  accessKeySecret
	S3Secret string `json:"s3Secret"`

	// Endpoint if blank points to aws. If Specifid, can point ti s3 compatible object
	// stores
	Endpoint string `json:"endpoint,omitempty"`
}

// OSSBackupSource provides the spec how to store backups on oss
type OSSBackupSource struct {
	// Path is the full abs path where the backup is saved.
	// The format of the path must be: "<oss-bucket-name>/<path-to-backup-file>"
	// e.g. "mybucket/myetcd.backup"
	Path string `json:"path"`

	// The name of the secret object that stores the credential which will be used
	// to access oss
	//
	// The secret must contains the following keys/fields:
	//  accessKeyID
	//  accessKeySecret
	OSSSecret string `json:"OSSSecret"`

	// Endpoint is the OSS service endpoint on oss
	// Default to "https://oss-cn-hangzhou.aliyuncs.com"
	//
	// Details about regions and endpoints, see:
	// https://www.alibabacloud.com/help/doc-detail/31837.htm
	Endpoint string `json:"endpoint,omitempty"`
}

type EtcdBackupPhase string

var (
	EtcdBackupPhaseBackingUp EtcdBackupPhase = "BackingUp"
	EtcdBackupPhaseCompleted EtcdBackupPhase = "Completed"
	EtcdbackupPhaseFailed    EtcdBackupPhase = "Failed"
)

// EtcdBackupStatus defines the observed state of EtcdBackup.
type EtcdBackupStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Phase defines the current operation that the backup process is taking
	Phase EtcdBackupPhase `json:"phase,omitempty"`

	// StartTime is the times that this backup entered the `BackingUp` Phase
	// +optional
	StartTime *metav1.Time `json:"startTime,omitempty"`

	// CompletionTime is the time that this backup entered the `Completed` phase.
	// +optional
	CompletionTime *metav1.Time `json:"completionTime,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// EtcdBackup is the Schema for the etcdbackups API.
type EtcdBackup struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   EtcdBackupSpec   `json:"spec,omitempty"`
	Status EtcdBackupStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// EtcdBackupList contains a list of EtcdBackup.
type EtcdBackupList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []EtcdBackup `json:"items"`
}

func init() {
	SchemeBuilder.Register(&EtcdBackup{}, &EtcdBackupList{})
}
