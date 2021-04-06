/*
Copyright 2020 The Crossplane Authors.

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

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
)

// PartSpec defines the Part spec for a Config
type PartSpec struct {
	ContentFromSource `json:",inline,omitempty"`
	ContentType       string `json:"content_type,omitempty"`
	Content           string `json:"content,omitempty"`
	Filename          string `json:"filename,omitempty"`
	MergeType         string `json:"merge_type,omitempty"`
}

// NamespacedName represents a namespaced object name
type NamespacedName struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
}

// DataKeySelector defines required spec to access a key of a configmap or secret
type DataKeySelector struct {
	NamespacedName `json:",inline,omitempty"`
	Key            string `json:"key,omitempty"`
	Optional       bool   `json:"optional,omitempty"`
}

// ContentFromSource represents source of a value
type ContentFromSource struct {
	ConfigMapKeyRef *DataKeySelector `json:"configMapKeyRef,omitempty"`
	SecretKeyRef    *DataKeySelector `json:"secretKeyRef,omitempty"`
}

// ConfigParameters are the configurable fields of a Config.
type ConfigParameters struct {
	Gzip         bool       `json:"gzip,omitempty"`
	Base64Encode bool       `json:"base64_encode,omitempty"`
	Boundary     string     `json:"boundary,omitempty"`
	Parts        []PartSpec `json:"part,omitempty"`
}

// ConfigObservation are the observable fields of a Config.
type ConfigObservation struct {
	State string `json:"state,omitempty"`
}

// A ConfigSpec defines the desired state of a Config.
type ConfigSpec struct {
	xpv1.ResourceSpec   `json:",inline"`
	ForProvider         ConfigParameters `json:"forProvider"`
	WriteCloudInitToRef *xpv1.Reference  `json:"writeCloudInitToRef,omitempty"`
}

// A ConfigStatus represents the observed state of a Config.
type ConfigStatus struct {
	xpv1.ResourceStatus `json:",inline"`
	AtProvider          ConfigObservation `json:"atProvider,omitempty"`
	Failed              int32             `json:"failed,omitempty"`
	Synced              bool              `json:"synced,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:storageversion

// A Config is an example API type
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="CONFIGMAP",type="string",JSONPath=".spec.writeCloudInitToRef.name"
// +kubebuilder:printcolumn:name="SYNCED",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="STATE",type="string",JSONPath=".status.atProvider.state"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:resource:scope=Cluster,categories={crossplane,provider,cloudinit}
type Config struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ConfigSpec   `json:"spec"`
	Status ConfigStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ConfigList contains a list of Config
type ConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Config `json:"items"`
}
