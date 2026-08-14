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

// RivenPriceWatchSpec defines the desired state of RivenPriceWatch.
type RivenPriceWatchSpec struct {
	// WeaponSlug is the URL-friendly weapon identifier (e.g. "falcor", "gram_prime").
	// +kubebuilder:validation:MinLength=1
	WeaponSlug string `json:"weaponSlug"`

	// PositiveStats lists the required positive stat url_names
	// (e.g. ["critical_chance", "critical_damage"]).
	// +optional
	PositiveStats []string `json:"positiveStats,omitempty"`

	// NegativeStats controls the negative stat filter:
	// "" = any, "has" = must have one, "none" = no negative, or a specific stat url_name.
	// +optional
	NegativeStats string `json:"negativeStats,omitempty"`

	// MaxReRolls limits results to rivens with at most this many re-rolls. 0 means no limit.
	// +optional
	// +kubebuilder:validation:Minimum=0
	MaxReRolls int `json:"maxReRolls,omitempty"`

	// PlayerStatus filters auctions by seller online status.
	// Valid values: ingame, online, offline. Defaults to [ingame, online] if not set.
	// +optional
	// +kubebuilder:validation:items:Enum=ingame;online;offline
	PlayerStatus []string `json:"playerStatus,omitempty"`

	// MinRollQuality is the minimum average roll quality (0–100%) required across
	// positive stats for a notification to be sent.
	// +optional
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=100
	MinRollQuality int `json:"minRollQuality,omitempty"`

	// BuyoutOnly restricts results to direct-sell auctions, excluding bidding-only listings.
	// +optional
	BuyoutOnly bool `json:"buyoutOnly,omitempty"`

	// Threshold is the platinum price at or below which a notification is sent.
	// +kubebuilder:validation:Minimum=1
	Threshold int `json:"threshold"`

	// NotificationWindow restricts notifications to a specific time range each day.
	// +optional
	NotificationWindow *NotificationWindow `json:"notificationWindow,omitempty"`
}

// RivenPriceWatchStatus defines the observed state of RivenPriceWatch.
type RivenPriceWatchStatus struct {
	// CheapestPrice is the lowest platinum price found among matching auctions.
	// +optional
	CheapestPrice int `json:"cheapestPrice,omitempty"`

	// BestRollQuality is the average roll quality of the cheapest matching auction.
	// +optional
	BestRollQuality int `json:"bestRollQuality,omitempty"`

	// NotifiedAuctionIDs tracks auction IDs for which a notification has already been sent.
	// This prevents duplicate notifications across reconcile cycles.
	// +optional
	NotifiedAuctionIDs []string `json:"notifiedAuctionIds,omitempty"`

	// LastNotifiedAt is the timestamp of the last sent notification.
	// +optional
	LastNotifiedAt *metav1.Time `json:"lastNotifiedAt,omitempty"`

	// Conditions represent the latest available observations of the RivenPriceWatch state.
	// +optional
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Weapon",type="string",JSONPath=".spec.weaponSlug"
// +kubebuilder:printcolumn:name="Threshold",type="integer",JSONPath=".spec.threshold"
// +kubebuilder:printcolumn:name="Cheapest",type="integer",JSONPath=".status.cheapestPrice"
// +kubebuilder:printcolumn:name="Roll Quality",type="integer",JSONPath=".status.bestRollQuality"
// +kubebuilder:printcolumn:name="Notified At",type="date",JSONPath=".status.lastNotifiedAt"
// +kubebuilder:printcolumn:name="Synced",type="string",JSONPath=".status.conditions[?(@.type=='PriceSynced')].status"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// RivenPriceWatch is the Schema for the rivenpricewatches API.
type RivenPriceWatch struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RivenPriceWatchSpec   `json:"spec,omitempty"`
	Status RivenPriceWatchStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// RivenPriceWatchList contains a list of RivenPriceWatch.
type RivenPriceWatchList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []RivenPriceWatch `json:"items"`
}

func init() {
	SchemeBuilder.Register(&RivenPriceWatch{}, &RivenPriceWatchList{})
}
