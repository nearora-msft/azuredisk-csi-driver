package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// AzVolumeOperation is a specification for a AzVolumeOperation resource
// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Namespaced
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="State",type=string,JSONPath=`.status.State`,description="The attachment status of the VolumeOperation",priority=10
type AzVolumeOperation struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec AzVolumeOperationSpec `json:"spec"`

	//+optional
	Status AzVolumeOperationStatus `json:"status"`
}

type RequestedOperation string

const (
	Attach RequestedOperation = "Attach"
	Detach RequestedOperation = "Detach"
)

type AzVolumeOperationSpec struct {
	DiskURI string `json:"diskUri"`
	//+optional
	BlobURL string `json:"blobUrl"`
	//+optional
	DSASToken string `json:"dsasToken"`
	//+optional
	Lun                int                `json:"lun"`
	RequestedOperation RequestedOperation `json:"requestedOperation"`
}

type AzVolumeOperationState string

const (
	VolumeDetached AzVolumeOperationState = "Detached"
	VolumeAttached AzVolumeOperationState = "Attached"
)

type AzVolumeOperationStatus struct {
	State AzVolumeOperationState `json:"state"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true
type AzVolumeOperationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []AzVolumeOperation `json:"items"`
}
