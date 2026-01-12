// SPDX-License-Identifier: MIT
// Copyright (c) 2023-2026 Posit Software, PBC

package v1beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// PostgresDatabaseSpec defines the desired state of PostgresDatabase
type PostgresDatabaseSpec struct {
	// +kubebuilder:validation:Pattern:=^postgres.+@.+/.+
	URL string `json:"url"`

	// SecretVault is the secretId to use for retrieving the password
	SecretVault string `json:"secretVault"`
	// SecretPasswordKey is the password key to use (within the SecretVault)
	SecretPasswordKey string `json:"secretPasswordKey"`

	// Secret is configuration to use for retrieving the password
	Secret SecretConfig `json:"secret,omitempty"`

	// WorkloadSecret configures the managed persistent secret for the entire workload account
	WorkloadSecret SecretConfig `json:"workloadSecret,omitempty"`

	// MainDatabaseCredentialSecret configures the secret used for storing the main database credentials
	MainDatabaseCredentialSecret SecretConfig `json:"mainDbCredentialSecret,omitempty"`

	// +optional
	Extensions []string `json:"extensions,omitempty"`
	// +optional
	Schemas []string `json:"schemas"`
	// +optional
	Teardown *PostgresDatabaseSpecTeardown `json:"teardown,omitempty"`
}

type PostgresDatabaseConfig struct {
	Host                  string `json:"host,omitempty"`
	SslMode               string `json:"sslMode,omitempty"`
	DropOnTeardown        bool   `json:"dropOnTeardown,omitempty"`
	Schema                string `json:"schema,omitempty"`
	InstrumentationSchema string `json:"instrumentationSchema,omitempty"`
}

type PostgresDatabaseSpecTeardown struct {
	// +optional
	Drop bool `json:"drop,omitempty"`
}

// PostgresDatabaseStatus defines the observed state of PostgresDatabase
type PostgresDatabaseStatus struct{}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:shortName={pgdb,pgdbs},path=postgresdatabases
//+genclient

// PostgresDatabase is the Schema for the postgresdatabases API
type PostgresDatabase struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   PostgresDatabaseSpec   `json:"spec,omitempty"`
	Status PostgresDatabaseStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// PostgresDatabaseList contains a list of PostgresDatabase
type PostgresDatabaseList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []PostgresDatabase `json:"items"`
}

func init() {
	SchemeBuilder.Register(&PostgresDatabase{}, &PostgresDatabaseList{})
}
