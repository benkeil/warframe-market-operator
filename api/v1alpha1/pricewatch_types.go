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

// PriceWatchSpec defines the desired state of PriceWatch.
type PriceWatchSpec struct {
	// ItemSlug is the URL-friendly item identifier used by the Warframe Market API
	// (e.g. "ash_prime_set"). It serves as the primary key for looking up orders.
	// +kubebuilder:validation:MinLength=1
	ItemSlug string `json:"itemSlug"`

	// Threshold is the platinum price at or below which a notification is sent.
	// +kubebuilder:validation:Minimum=1
	Threshold int `json:"threshold"`

	// NotificationWindow restricts notifications to a specific time range each day.
	// If omitted, notifications are sent at any time.
	// When a new calendar day begins, the notification state resets so a fresh
	// notification can be sent regardless of the previous day's price.
	// +optional
	NotificationWindow *NotificationWindow `json:"notificationWindow,omitempty"`
}

// NotificationWindow defines a daily time range during which notifications are allowed.
type NotificationWindow struct {
	// From is the start of the window in "HH:MM" format (e.g. "10:00").
	// +kubebuilder:validation:Pattern=`^([01]\d|2[0-3]):[0-5]\d$`
	From string `json:"from"`

	// To is the end of the window in "HH:MM" format (e.g. "18:00").
	// +kubebuilder:validation:Pattern=`^([01]\d|2[0-3]):[0-5]\d$`
	To string `json:"to"`
}

// PriceWatchStatus defines the observed state of PriceWatch.
type PriceWatchStatus struct {
	// CheapestPrice is the lowest platinum price found among the top sell orders
	// from online users at the time of the last reconciliation.
	// +optional
	CheapestPrice int `json:"cheapestPrice,omitempty"`

	// LastNotifiedPrice is the platinum price at which the last notification was sent.
	// Nil means no notification has been sent yet in the current day.
	// +optional
	LastNotifiedPrice *int `json:"lastNotifiedPrice,omitempty"`

	// LastNotifiedAt is the timestamp of the last sent notification.
	// Used to detect day boundaries and reset the notification state.
	// +optional
	LastNotifiedAt *metav1.Time `json:"lastNotifiedAt,omitempty"`

	// Conditions represent the latest available observations of the PriceWatch state.
	// +optional
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// PriceWatch is the Schema for the pricewatches API.
type PriceWatch struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   PriceWatchSpec   `json:"spec,omitempty"`
	Status PriceWatchStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// PriceWatchList contains a list of PriceWatch.
type PriceWatchList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []PriceWatch `json:"items"`
}

func init() {
	SchemeBuilder.Register(&PriceWatch{}, &PriceWatchList{})
}
