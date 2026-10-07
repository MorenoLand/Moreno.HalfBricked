package formats

import (
	"bytes"
	"encoding/xml"
	"io"
	"os"
	"strings"
	"testing"
)

func TestWeaponRecordFields(t *testing.T) {
	catalog, err := ParseWeapons(strings.NewReader(`<WeaponData><Weapon><Gun_Type>SHOTGUN</Gun_Type><Gun_Category>COMMON</Gun_Category><Gun_Name>Shotgun</Gun_Name><SFX_VO>SFX_VO_SHOTGUN</SFX_VO><Value>0</Value><Rate_Of_Fire>0.7</Rate_Of_Fire><Ammo>16</Ammo><Life>0.25</Life><Speed>800.0</Speed><Bullet_Type>NORMAL</Bullet_Type><Spread>19</Spread><SFX_Start>SFX_SHOTGUN_1</SFX_Start><SFX_Shoot>0</SFX_Shoot><SFX_End>SFX_SHOTGUN_2</SFX_End><Texture_Gun>Characters/barrygun_01</Texture_Gun><Texture_Flash>Characters/barrygun_01_flash</Texture_Flash><Texture_Flare>Textures/muzzleflash_01</Texture_Flare><Score_Multiplier><SM_Info threshold="10" text="Poor" multiplier="1"/><SM_Info threshold="80" text="AWESOME" multiplier="8"/></Score_Multiplier></Weapon></WeaponData>`))
	if err != nil {
		t.Fatal(err)
	}
	w := catalog[0]
	if w.RateOfFire != .7 || w.Life != .25 || w.Speed != 800 || w.Ammo != 16 || w.SpreadUnits != 3458 {
		t.Fatalf("numeric fields: %+v", w)
	}
	if w.SFXVO != "SFX_VO_SHOTGUN" || w.SFXStart != "SFX_SHOTGUN_1" || w.SFXShoot != "0" || w.SFXEnd != "SFX_SHOTGUN_2" {
		t.Fatalf("audio phases: %+v", w)
	}
	if w.TextureFlash != "Characters/barrygun_01_flash" || w.TextureFlare != "Textures/muzzleflash_01" {
		t.Fatalf("effects: %+v", w)
	}
	if len(w.ScoreMultipliers) != 2 || w.ScoreMultipliers[1] != (WeaponScoreMultiplier{Threshold: 80, Text: "AWESOME", Multiplier: 8}) {
		t.Fatalf("multipliers: %+v", w.ScoreMultipliers)
	}
}

func TestWeaponOptionalNameAndFields(t *testing.T) {
	catalog, err := ParseWeapons(strings.NewReader(`<WeaponData><Weapon><Gun_Type>SENTRY_UZI</Gun_Type><Gun_Category>COMMON</Gun_Category><Value>2</Value><Spread>360</Spread></Weapon></WeaponData>`))
	if err != nil {
		t.Fatal(err)
	}
	w := catalog[0]
	if w.GunName != "" || w.Value != 2 || w.SpreadUnits != 65520 || w.SFXStart != "" || len(w.ScoreMultipliers) != 0 {
		t.Fatalf("optional fields: %+v", w)
	}
}

func TestWeaponInvalidExpandedFields(t *testing.T) {
	for _, field := range []string{"Value", "Spread"} {
		_, err := ParseWeapons(strings.NewReader("<WeaponData><Weapon><Gun_Type>PISTOL</Gun_Type><" + field + ">invalid</" + field + "></Weapon></WeaponData>"))
		if err == nil || !strings.Contains(err.Error(), field) {
			t.Fatalf("%s: %v", field, err)
		}
	}
}

func TestOriginalWeaponCatalog(t *testing.T) {
	file, err := os.Open("../../bin/data-cache/source/Common0/Xml/Common0_Weapons.xml")
	if os.IsNotExist(err) {
		t.Skip("original local weapon XML unavailable")
	}
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	data, err := io.ReadAll(file)
	if err != nil {
		t.Fatal(err)
	}
	catalog, err := ParseWeapons(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	var declared struct {
		Weapons []struct {
			GunType string `xml:"Gun_Type"`
		} `xml:"Weapon"`
	}
	if err := xml.Unmarshal(data, &declared); err != nil {
		t.Fatal(err)
	}
	if len(catalog) == 0 || len(catalog) != len(declared.Weapons) {
		t.Fatalf("records: %d, native XML declares %d", len(catalog), len(declared.Weapons))
	}
	for _, entry := range declared.Weapons {
		if _, ok := catalog.Find(entry.GunType); !ok {
			t.Fatalf("native weapon %s missing", entry.GunType)
		}
	}
	for _, want := range []struct {
		gun, bullet, start, shoot, end string
		rate, life                     float64
		ammo                           int
		spread                         uint16
	}{
		{"SHOTGUN", "NORMAL", "SFX_SHOTGUN_1", "0", "SFX_SHOTGUN_2", .7, .25, 16, 3458},
		{"FLAMER", "FLAME", "0", "SFX_FLAMETHROWER", "0", .035, 1, 250, 3276},
		{"MINIGUN", "NORMAL", "SFX_MINIGUN_SPIN_UP", "SFX_MINIGUN", "SFX_MINIGUN_SPIN_DOWN", .03, .75, 200, 910},
	} {
		w, ok := catalog.Find(want.gun)
		if !ok || w.BulletType != want.bullet || w.SFXStart != want.start || w.SFXShoot != want.shoot || w.SFXEnd != want.end || w.RateOfFire != want.rate || w.Life != want.life || w.Ammo != want.ammo || w.SpreadUnits != want.spread {
			t.Fatalf("%s: %+v", want.gun, w)
		}
		if len(w.ScoreMultipliers) < 5 {
			t.Fatalf("%s thresholds missing", want.gun)
		}
	}
	if _, ok := catalog.Find("LASER"); ok {
		t.Fatal("unexpected laser record")
	}
}
