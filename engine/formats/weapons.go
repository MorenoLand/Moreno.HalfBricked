package formats

import (
	"encoding/xml"
	"fmt"
	"io"
	"strconv"
	"strings"
)

type Weapon struct {
	GunType      string
	GunCategory  string
	GunName      string
	RateOfFire   float64
	Ammo         int
	Life         float64
	Speed        float64
	BulletType   string
	Spread       float64
	SFXShoot     string
	TextureGun   string
	TextureFlash string
	TextureFlare string
}

type WeaponCatalog []Weapon

type weaponXML struct {
	GunType      string `xml:"Gun_Type"`
	GunCategory  string `xml:"Gun_Category"`
	GunName      string `xml:"Gun_Name"`
	RateOfFire   string `xml:"Rate_Of_Fire"`
	Ammo         string `xml:"Ammo"`
	Life         string `xml:"Life"`
	Speed        string `xml:"Speed"`
	BulletType   string `xml:"Bullet_Type"`
	Spread       string `xml:"Spread"`
	SFXShoot     string `xml:"SFX_Shoot"`
	TextureGun   string `xml:"Texture_Gun"`
	TextureFlash string `xml:"Texture_Flash"`
	TextureFlare string `xml:"Texture_Flare"`
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
		if strings.TrimSpace(item.GunType) == "" || strings.TrimSpace(item.GunName) == "" {
			return nil, fmt.Errorf("weapon %d is missing Gun_Type or Gun_Name", index)
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
		result = append(result, Weapon{GunType: item.GunType, GunCategory: item.GunCategory, GunName: item.GunName, RateOfFire: rate, Ammo: ammo, Life: life, Speed: speed, BulletType: item.BulletType, Spread: spread, SFXShoot: item.SFXShoot, TextureGun: item.TextureGun, TextureFlash: item.TextureFlash, TextureFlare: item.TextureFlare})
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
