package usecase

import (
	"math"

	"github.com/benkeil/warframe-market-operator/internal/domain/service"
)

// RivenCalculator computes roll quality for riven mod attributes.
// Formula sourced from https://calamity-inc.github.io/warframe-riven-info/RivenParser.js
//
// actualDisplayValue = baseTagValue × 1.5 × 10 × disposition × curseAtten
//
//	× lerp(0.9, 1.1, roll) × numBuffsAtten[numBuffs] × (rank+1) × 100
//
// Roll quality = (rollFactor - 0.9) / 0.2 × 100%
// 0% = worst possible roll, 100% = best possible roll.
type RivenCalculator struct{}

// numBuffsAtten are the per-stat attenuation multipliers indexed by number of positive stats.
var numBuffsAtten = []float64{0, 1, 0.66000003, 0.5, 0.40000001, 0.34999999}

// categoryToRivenType maps a weapon productCategory to the riven_tags key.
//
//nolint:goconst
var categoryToRivenType = map[string]string{
	"Melee":    "PlayerMeleeWeaponRandomModRare",
	"Swords":   "PlayerMeleeWeaponRandomModRare",
	"Pistols":  "LotusPistolRandomModRare",
	"Primary":  "LotusRifleRandomModRare",
	"Shotguns": "LotusShotgunRandomModRare",
	"Archguns": "LotusArchgunRandomModRare",
	"Zaw":      "LotusModularMeleeRandomModRare",
	"Kitgun":   "LotusModularPistolRandomModRare",
}

// rivenBaseValues maps rivenType → url_name → base value (from RivenParser.js riven_tags).
//
//nolint:goconst
var rivenBaseValues = map[string]map[string]float64{
	"PlayerMeleeWeaponRandomModRare": {
		"base_damage_/_melee_damage":       0.0183,
		"puncture_damage":                  0.0133,
		"impact_damage":                    0.0133,
		"slash_damage":                     0.0133,
		"critical_chance":                  0.02,
		"critical_damage":                  0.01,
		"electric_damage":                  0.01,
		"heat_damage":                      0.01,
		"cold_damage":                      0.01,
		"toxin_damage":                     0.01,
		"damage_vs_corpus":                 0.005,
		"damage_vs_grineer":                0.005,
		"damage_vs_infested":               0.005,
		"fire_rate_/_attack_speed":         0.0061,
		"combo_duration":                   0.09,
		"critical_chance_on_slide_attack":  0.013334,
		"range":                            0.02158,
		"finisher_damage":                  0.0133,
		"status_chance":                    0.01,
		"status_duration":                  0.01111,
		"channeling_efficiency":            0.00816,
		"chance_to_gain_combo_count":       0.00653,
		"chance_to_gain_extra_combo_count": 0.27224,
		"punch_through":                    0.015,
	},
	"LotusModularMeleeRandomModRare": {
		"base_damage_/_melee_damage":       0.0183,
		"puncture_damage":                  0.0133,
		"impact_damage":                    0.0133,
		"slash_damage":                     0.0133,
		"critical_chance":                  0.02,
		"critical_damage":                  0.01,
		"electric_damage":                  0.01,
		"heat_damage":                      0.01,
		"cold_damage":                      0.01,
		"toxin_damage":                     0.01,
		"damage_vs_corpus":                 0.005,
		"damage_vs_grineer":                0.005,
		"damage_vs_infested":               0.005,
		"fire_rate_/_attack_speed":         0.0061,
		"combo_duration":                   0.09,
		"critical_chance_on_slide_attack":  0.013334,
		"range":                            0.02158,
		"finisher_damage":                  0.0133,
		"status_chance":                    0.01,
		"status_duration":                  0.01111,
		"channeling_efficiency":            0.00816,
		"chance_to_gain_combo_count":       0.00653,
		"chance_to_gain_extra_combo_count": 0.27224,
	},
	"LotusRifleRandomModRare": {
		"base_damage_/_base_damage": 0.01333,
		"critical_chance":           0.016666,
		"critical_damage":           0.013333,
		"electric_damage":           0.013333,
		"heat_damage":               0.013333,
		"cold_damage":               0.013333,
		"toxin_damage":              0.013333,
		"status_chance":             0.016666,
		"fire_rate":                 0.0125,
		"multishot":                 0.013333,
		"reload_speed":              0.013333,
		"magazine_capacity":         0.013333,
		"ammo_maximum":              0.013333,
		"damage_vs_corpus":          0.01333,
		"damage_vs_grineer":         0.01333,
		"damage_vs_infested":        0.01333,
		"punch_through":             0.015,
		"zoom":                      0.016666,
		"recoil":                    0.01333,
		"projectile_speed":          0.016666,
		"puncture_damage":           0.01333,
		"impact_damage":             0.01333,
		"slash_damage":              0.01333,
	},
	"LotusPistolRandomModRare": {
		"base_damage_/_base_damage": 0.01333,
		"critical_chance":           0.016666,
		"critical_damage":           0.013333,
		"electric_damage":           0.013333,
		"heat_damage":               0.013333,
		"cold_damage":               0.013333,
		"toxin_damage":              0.013333,
		"status_chance":             0.016666,
		"fire_rate":                 0.0125,
		"multishot":                 0.013333,
		"reload_speed":              0.013333,
		"magazine_capacity":         0.013333,
		"ammo_maximum":              0.013333,
		"damage_vs_corpus":          0.01333,
		"damage_vs_grineer":         0.01333,
		"damage_vs_infested":        0.01333,
		"punch_through":             0.015,
		"zoom":                      0.016666,
		"recoil":                    0.01333,
		"projectile_speed":          0.016666,
		"puncture_damage":           0.01333,
		"impact_damage":             0.01333,
		"slash_damage":              0.01333,
	},
	"LotusShotgunRandomModRare": {
		"base_damage_/_base_damage": 0.01333,
		"critical_chance":           0.016666,
		"critical_damage":           0.013333,
		"electric_damage":           0.013333,
		"heat_damage":               0.013333,
		"cold_damage":               0.013333,
		"toxin_damage":              0.013333,
		"status_chance":             0.016666,
		"fire_rate":                 0.0125,
		"multishot":                 0.013333,
		"reload_speed":              0.013333,
		"magazine_capacity":         0.013333,
		"ammo_maximum":              0.013333,
		"damage_vs_corpus":          0.01333,
		"damage_vs_grineer":         0.01333,
		"damage_vs_infested":        0.01333,
		"punch_through":             0.015,
		"zoom":                      0.016666,
		"recoil":                    0.01333,
		"projectile_speed":          0.016666,
		"puncture_damage":           0.01333,
		"impact_damage":             0.01333,
		"slash_damage":              0.01333,
	},
	"LotusArchgunRandomModRare": {
		"base_damage_/_base_damage": 0.0111,
		"critical_chance":           0.0111,
		"critical_damage":           0.0089,
		"electric_damage":           0.0133,
		"heat_damage":               0.0133,
		"cold_damage":               0.0133,
		"toxin_damage":              0.0133,
		"status_chance":             0.0067,
		"fire_rate":                 0.00667,
		"multishot":                 0.0067,
		"reload_speed":              0.0067,
		"magazine_capacity":         0.0067,
		"ammo_maximum":              0.0111,
		"damage_vs_corpus":          0.01,
		"damage_vs_grineer":         0.01,
		"damage_vs_infested":        0.01,
		"punch_through":             0.015,
		"impact_damage":             0.01,
		"puncture_damage":           0.01,
		"slash_damage":              0.01,
	},
}

// RivenStatScore holds the computed roll quality for a single riven attribute.
type RivenStatScore struct {
	URLName     string
	Value       float64
	Positive    bool
	RollQuality float64 // 0–100%, negative for curse
	Known       bool    // false if base value is unknown
}

// ScoreAuction computes the roll quality for each stat of the given auction.
// Returns nil scores if weapon info is not provided.
func (c *RivenCalculator) ScoreAuction(auction service.Auction, weapon *service.WeaponInfo) []RivenStatScore {
	if weapon == nil {
		return nil
	}

	rivenType := categoryToRivenType[weapon.Category]
	if rivenType == "" {
		rivenType = "PlayerMeleeWeaponRandomModRare" // fallback
	}
	baseValues := rivenBaseValues[rivenType]

	numPositive := 0
	numNegative := 0
	for _, a := range auction.Item.Attributes {
		if a.Positive {
			numPositive++
		} else {
			numNegative++
		}
	}

	curseAtten := math.Pow(1.25, float64(numNegative))
	nba := numBuffsAtten[min(numPositive, len(numBuffsAtten)-1)]
	rankMult := float64(auction.Item.ModRank + 1)
	// SPECIFIC_FIT_ATTENUATION × getBaseDrain(RIVEN_BASE_DRAIN)
	baseMult := 1.5 * 10.0 * weapon.Disposition * 100.0

	scores := make([]RivenStatScore, 0, len(auction.Item.Attributes))
	for _, attr := range auction.Item.Attributes {
		baseVal, ok := baseValues[attr.URLName]
		score := RivenStatScore{
			URLName:  attr.URLName,
			Value:    attr.Value,
			Positive: attr.Positive,
			Known:    ok,
		}
		if ok {
			// Compute the lerp factor: actualDisplayValue = baseVal × baseMult × curseAtten × lerpFactor × nba × rankMult × 100
			// lerpFactor is in [0.9, 1.1]; rollQuality% = (lerpFactor - 0.9) / 0.2 × 100
			denominator := baseVal * baseMult * nba * rankMult
			if attr.Positive {
				denominator *= curseAtten
			}
			if denominator != 0 {
				lerpFactor := math.Abs(attr.Value) / denominator
				score.RollQuality = (lerpFactor - 0.9) / 0.2 * 100
				score.RollQuality = math.Max(0, math.Min(100, score.RollQuality))
			}
		}
		scores = append(scores, score)
	}
	return scores
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
