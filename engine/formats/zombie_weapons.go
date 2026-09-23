package formats

import (
	"encoding/xml"
	"fmt"
	"io"
	"strings"
)

type ZombieWeapon struct {
	GunType       string
	RecoilSeconds float64
	RateOfFire    float64
	SpreadUnits   float64
	Ammo          int
	AmmoPerShot   int
	Life          float64
	Speed         float64
	BulletType    string
}

type ZombieWeaponCatalog []ZombieWeapon

type zombieWeaponXML struct {
	GunType     string `xml:"Gun_Type"`
	Recoil      string `xml:"Recoil"`
	RateOfFire  string `xml:"Rate_Of_Fire"`
	Spread      string `xml:"Spread"`
	Ammo        string `xml:"Ammo"`
	AmmoPerShot string `xml:"Ammo_Per_Shot"`
	Life        string `xml:"Life"`
	Speed       string `xml:"Speed"`
	BulletType  string `xml:"Bullet_Type"`
}

func ParseZombieWeapons(reader io.Reader) (ZombieWeaponCatalog, error) {
	var document struct {
		Weapons []zombieWeaponXML `xml:"Weapon"`
	}
	if err := xml.NewDecoder(reader).Decode(&document); err != nil {
		return nil, err
	}
	if len(document.Weapons) == 0 {
		return nil, fmt.Errorf("zombie weapon catalog is empty")
	}
	result := make(ZombieWeaponCatalog, 0, len(document.Weapons))
	for index, item := range document.Weapons {
		gunType := strings.TrimSpace(item.GunType)
		if gunType == "" {
			return nil, fmt.Errorf("zombie weapon %d is missing Gun_Type", index)
		}
		recoil, err := parseWeaponInt(item.Recoil, "Recoil", index)
		if err != nil {
			return nil, err
		}
		rateOfFire, err := parseWeaponInt(item.RateOfFire, "Rate_Of_Fire", index)
		if err != nil {
			return nil, err
		}
		spread, err := parseWeaponInt(item.Spread, "Spread", index)
		if err != nil {
			return nil, err
		}
		ammo, err := parseWeaponInt(item.Ammo, "Ammo", index)
		if err != nil {
			return nil, err
		}
		ammoPerShot, err := parseWeaponInt(item.AmmoPerShot, "Ammo_Per_Shot", index)
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
		spreadUnits := uint32(int32(spread)*0x00b60000) >> 16
		result = append(result, ZombieWeapon{GunType: gunType, RecoilSeconds: float64(recoil) / 1000, RateOfFire: float64(rateOfFire) / 1000, SpreadUnits: float64(spreadUnits), Ammo: ammo, AmmoPerShot: ammoPerShot, Life: life, Speed: speed, BulletType: strings.TrimSpace(item.BulletType)})
	}
	return result, nil
}
