package formats

import (
	"encoding/xml"
	"fmt"
	"io"
	"strconv"
	"strings"
)

type Weapon struct {
	GunType          string
	GunCategory      string
	GunName          string
	Value            int
	RateOfFire       float64
	Ammo             int
	Life             float64
	Speed            float64
	BulletType       string
	Spread           float64
	SpreadUnits      uint16
	SFXVO            string
	SFXStart         string
	SFXShoot         string
	SFXEnd           string
	TextureGun       string
	TextureFlash     string
	TextureFlare     string
	ScoreMultipliers []WeaponScoreMultiplier
}

type WeaponScoreMultiplier struct {
	Threshold  int    `xml:"threshold,attr"`
	Text       string `xml:"text,attr"`
	Multiplier int    `xml:"multiplier,attr"`
}

type WeaponCatalog []Weapon

type weaponXML struct {
	GunType          string                  `xml:"Gun_Type"`
	GunCategory      string                  `xml:"Gun_Category"`
	GunName          string                  `xml:"Gun_Name"`
	Value            string                  `xml:"Value"`
	RateOfFire       string                  `xml:"Rate_Of_Fire"`
	Ammo             string                  `xml:"Ammo"`
	Life             string                  `xml:"Life"`
	Speed            string                  `xml:"Speed"`
	BulletType       string                  `xml:"Bullet_Type"`
	Spread           string                  `xml:"Spread"`
	SFXVO            string                  `xml:"SFX_VO"`
	SFXStart         string                  `xml:"SFX_Start"`
	SFXShoot         string                  `xml:"SFX_Shoot"`
	SFXEnd           string                  `xml:"SFX_End"`
	TextureGun       string                  `xml:"Texture_Gun"`
	TextureFlash     string                  `xml:"Texture_Flash"`
	TextureFlare     string                  `xml:"Texture_Flare"`
	ScoreMultipliers []WeaponScoreMultiplier `xml:"Score_Multiplier>SM_Info"`
}

func ParseWeapons(reader io.Reader) (WeaponCatalog, error) {
	var document struct {
		Weapons []weaponXML `xml:"Weapon"`
	}
	if err := xml.NewDecoder(reader).Decode(&document); err != nil {
		return nil, err
	}
	if len(document.Weapons) == 0 {
		return nil, fmt.Errorf("weapon catalog is empty")
	}
	result := make(WeaponCatalog, 0, len(document.Weapons))
	for index, item := range document.Weapons {
		if strings.TrimSpace(item.GunType) == "" {
			return nil, fmt.Errorf("weapon %d is missing Gun_Type", index)
		}
		value, err := parseWeaponInt(item.Value, "Value", index)
		if err != nil {
			return nil, err
		}
		rate, err := parseWeaponFloat(item.RateOfFire, "Rate_Of_Fire", index)
		if err != nil {
			return nil, err
		}
		ammo, err := parseWeaponInt(item.Ammo, "Ammo", index)
		if err != nil {
			return nil, err
		}
		life, err := parseWeaponFloat(item.Life, "Life", index)
		if err != nil {
			return nil, err
		}
		speed, err := parseWeaponFloat(item.Speed, "Speed", index)
		if err != nil {
			return nil, err
		}
		spread, err := parseWeaponFloat(item.Spread, "Spread", index)
		if err != nil {
			return nil, err
		}
		spreadDegrees, err := parseWeaponInt(item.Spread, "Spread", index)
		if err != nil {
			return nil, err
		}
		result = append(result, Weapon{GunType: item.GunType, GunCategory: item.GunCategory, GunName: item.GunName, Value: value, RateOfFire: rate, Ammo: ammo, Life: life, Speed: speed, BulletType: item.BulletType, Spread: spread, SpreadUnits: uint16(spreadDegrees * 182), SFXVO: item.SFXVO, SFXStart: item.SFXStart, SFXShoot: item.SFXShoot, SFXEnd: item.SFXEnd, TextureGun: item.TextureGun, TextureFlash: item.TextureFlash, TextureFlare: item.TextureFlare, ScoreMultipliers: item.ScoreMultipliers})
	}
	return result, nil
}

func (catalog WeaponCatalog) Find(gunType string) (Weapon, bool) {
	for _, weapon := range catalog {
		if strings.EqualFold(weapon.GunType, gunType) {
			return weapon, true
		}
	}
	return Weapon{}, false
}

func parseWeaponFloat(value, field string, index int) (float64, error) {
	if strings.TrimSpace(value) == "" {
		return 0, nil
	}
	parsed, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
	if err != nil {
		return 0, fmt.Errorf("weapon %d %s: %w", index, field, err)
	}
	return parsed, nil
}

func parseWeaponInt(value, field string, index int) (int, error) {
	if strings.TrimSpace(value) == "" {
		return 0, nil
	}
	parsed, err := strconv.ParseInt(strings.TrimSpace(value), 10, 0)
	if err != nil {
		return 0, fmt.Errorf("weapon %d %s: %w", index, field, err)
	}
	return int(parsed), nil
}
