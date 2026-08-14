/*
Copyright 2026.

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

// ItemPriceWatchSpec defines the desired state of ItemPriceWatch.
type ItemPriceWatchSpec struct {
	// ItemSlug is the URL-friendly item identifier used by the Warframe Market API
	// (e.g. "ash_prime_set"). It serves as the primary key for looking up orders.
	// +kubebuilder:validation:MinLength=1
	ItemSlug string `json:"itemSlug"`

	// Threshold is the platinum price at or below which a notification is sent.
	// +kubebuilder:validation:Minimum=1
	Threshold int `json:"threshold"`

	// NotificationWindow restricts notifications to a specific time range each day.
	// +optional
	NotificationWindow *NotificationWindow `json:"notificationWindow,omitempty"`
}

// ItemPriceWatchStatus defines the observed state of ItemPriceWatch.
type ItemPriceWatchStatus struct {
	// CheapestPrice is the lowest platinum price found among the top sell orders.
	// +optional
	CheapestPrice int `json:"cheapestPrice,omitempty"`

	// LastNotifiedPrice is the platinum price at which the last notification was sent.
	// +optional
	LastNotifiedPrice *int `json:"lastNotifiedPrice,omitempty"`

	// LastNotifiedAt is the timestamp of the last sent notification.
	// +optional
	LastNotifiedAt *metav1.Time `json:"lastNotifiedAt,omitempty"`

	// Conditions represent the latest available observations of the ItemPriceWatch state.
	// +optional
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Item",type="string",JSONPath=".spec.itemSlug"
// +kubebuilder:printcolumn:name="Threshold",type="integer",JSONPath=".spec.threshold"
// +kubebuilder:printcolumn:name="Cheapest",type="integer",JSONPath=".status.cheapestPrice"
// +kubebuilder:printcolumn:name="Last Notified",type="integer",JSONPath=".status.lastNotifiedPrice"
// +kubebuilder:printcolumn:name="Notified At",type="date",JSONPath=".status.lastNotifiedAt"
// +kubebuilder:printcolumn:name="Synced",type="string",JSONPath=".status.conditions[?(@.type=='PriceSynced')].status"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// ItemPriceWatch is the Schema for the itempricewatches API.
type ItemPriceWatch struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ItemPriceWatchSpec   `json:"spec,omitempty"`
	Status ItemPriceWatchStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ItemPriceWatchList contains a list of ItemPriceWatch.
type ItemPriceWatchList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ItemPriceWatch `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ItemPriceWatch{}, &ItemPriceWatchList{})
}
