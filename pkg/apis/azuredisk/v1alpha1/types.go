package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// AzVolumeOperation is a specification for a AzVolumeOperation resource
// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Namespaced
// +kubebuilder:printcolumn:name="State",type=string,JSONPath=`.status.State`,description="The attachment status of the VolumeOperation",priority=10
type AzVolumeOperation struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   AzVolumeOperationSpec   `json:"spec"`
	Status AzVolumeOperationStatus `json:"status"`
}

type AzVolumeOperationSpec struct {
	BlobUrl   string `json:"blobUrl"`
	DSASToken string `json:"dsasToken"`
}

type AzVolumeState string

const (
	VolumeAttaching AzVolumeState = "Attaching"
	VolumeDetaching AzVolumeState = "Detaching"
	VolumeAttached  AzVolumeState = "Attached"
)

type AzVolumeOperationStatus struct {
	State string `json:"state"`
	Lun   int    `json:"lun"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true
type AzVolumeOperationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []AzVolumeOperation `json:"items"`
}
