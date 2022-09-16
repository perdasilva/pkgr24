/*
Copyright 2022.

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
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// +kubebuilder:validation:Required

const (
	RemovalPolicyCascade          = "cascade"
	RemovalPolicyKeepDependencies = "keepDependencies"

	StateNotInstalled = "NotInstalled"
	StateInstalled    = "Installed"
	StateInstalling   = "Installing"
	StateRemoving     = "Removing"
	StateFailure      = "Failure"
	StateUpgradeable  = "Upgradable"
	StateNoState      = ""

	PhaseResolvingDependencies  = "ResolvingDependencies"
	PhaseInstallingDependencies = "InstallingDependencies"
	PhaseDeployingBundle        = "DeployingBundle"
	PhaseInstallingBundle       = "InstallingBundle"
	PhaseRemovingDependencies   = "RemovingDependencies"
	PhaseRemovingBundle         = "RemovingBundle"
	PhaseFailure                = "Failure"
	PhaseNoPhase                = ""

	ReasonNoReason = ""
)

type RemovalPolicy string
type State string
type Phase string

// PackageSpec defines the desired state of Package
type PackageSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	// +kubebuilder:default=false
	Install bool `json:"install"`

	// +optional
	Version string `json:"version,omitempty"`

	// +kubebuilder:validation:MinItems=1
	Bundles []Bundle `json:"bundles"`

	// +kubebuilder:validation:Enum={"cascade", "keepDependencies"}
	// +kubebuilder:default="cascade"
	RemovalPolicy RemovalPolicy `json:"removalPolicy"`
}

// PackageStatus defines the observed state of Package
type PackageStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	State State `json:"state"`

	// +optional
	CurrentVersion string `json:"currentVersion,omitempty"`

	// +optional
	Phase Phase `json:"phase,omitempty"`

	// +optional
	Reason string `json:"reason,omitempty"`

	// +optional
	BundleRef *v1.ObjectReference `json:"bundleRef,omitempty"`

	// +optional
	Dependencies []DependencyRef `json:"dependencies"`
}

type Bundle struct {
	Version string `json:"version"`
	Repo    string `json:"repo"`

	// +optional
	Dependencies []Dependency `json:"dependencies,omitempty"`

	// +optional
	Replaces string `json:"replaces,omitempty"`
}

type Dependency struct {
	Package string `json:"package"`
	Version string `json:"version"`
}

type DependencyRef struct {
	Package string `json:"package"`
	Version string `json:"version"`

	// +optional
	BundleRef *v1.ObjectReference `json:"bundleRef,omitempty"`

	// +optional
	State State `json:"state"`

	// +optional
	Reason string `json:"reason,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:scope=Cluster,shortName={"pkg","pkgs"}

// Package is the Schema for the packages API
type Package struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   PackageSpec   `json:"spec,omitempty"`
	Status PackageStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// PackageList contains a list of Package
type PackageList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Package `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Package{}, &PackageList{})
}
