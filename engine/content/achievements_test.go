package content

import (
	"github.com/MorenoLand/Moreno.HalfBricked/engine/formats"
	"strings"
	"testing"
)

func TestAchievementsFollowDeclaredCatalogNotVariantGuess(t *testing.T) {
	source := memoryAssetSource{manifest: PackManifest{SchemaVersion: 1, Files: map[string]string{"Common0/Xml/Common0_Manifest.xml": "manifest", "Common0/Xml/Common0_Achievements.xml": "regular", "Common0/Xml/Common0_Achievements_Anniversary.xml": "anniversary"}}, files: map[string]string{"manifest": `<PackageManifest><XmlInfo achievement="Common0_Achievements"/></PackageManifest>`, "regular": `<achievementManagerFile><achievement id="AOZ_1" name="SMG" type="KILLS" check="ge" total="125" specific_type="smg"/></achievementManagerFile>`, "anniversary": `<achievementManagerFile><achievement id="2" name="SMG" total="100"/></achievementManagerFile>`}}
	p, err := NewPack(source)
	if err != nil {
		t.Fatal(err)
	}
	entries, err := p.Achievements()
	if err != nil || len(entries) != 1 || entries[0].ID != "AOZ_1" || entries[0].Total != 125 {
		t.Fatalf("catalog selection lost: %+v %v", entries, err)
	}
	delete(source.manifest.Files, "Common0/Xml/Common0_Achievements.xml")
	if _, err := p.Achievements(); err == nil {
		t.Fatal("missing original path silently replaced")
	}
}
func TestAchievementXMLRejectsBadDefinitions(t *testing.T) {
	for _, text := range []string{`<wrong/>`, `<achievementManagerFile><achievement name="missing ID"/></achievementManagerFile>`, `<achievementManagerFile><achievement id="1" name="A"/><achievement id="1" name="B"/></achievementManagerFile>`} {
		if _, err := formats.ParseAchievements(strings.NewReader(text)); err == nil {
			t.Fatal("bad catalog accepted")
		}
	}
}
func TestPackHDOnlySpritesKeepOriginalResource(t *testing.T) {
	source := memoryAssetSource{manifest: PackManifest{SchemaVersion: 1, Files: map[string]string{"Common0/Xml/Common0_Sprites_HD.xml": "hd"}}, files: map[string]string{"hd": `<SpriteLibrary><Sprites><Sprite name="Characters/cavezombie"><Anim name="Idle" texture="Textures/Characters/cavezombie_HD" numFrames="4" numAngles="5" fps="8" loop="1"/></Sprite></Sprites></SpriteLibrary>`}}
	p, err := NewPack(source)
	if err != nil {
		t.Fatal(err)
	}
	entries, err := p.Sprites()
	entry, found := entries.Find("Characters/cavezombie")
	animation, exists := entry.Animation("Idle")
	if err != nil || len(entries) != 1 || !found || !exists || animation.Texture != "Textures/Characters/cavezombie_HD" {
		t.Fatalf("HD resource replaced: %+v %v", entries, err)
	}
}
func TestHDResourceResolutionPreservesExistingSD(t *testing.T) {
	p := &Pack{manifest: PackManifest{Files: map[string]string{"Common0/Xml/Common0_Sprites_HD.xml": "hd"}, Textures: map[string]string{"frontend0/textures/portal_menu_hd": "textures/Portal_Menu_HD.png"}}}
	if path, ok := p.TexturePath("Frontend0/Textures/Portal_Menu_SD"); !ok || path != "textures/Portal_Menu_HD.png" || p.TextureSourceScale("Frontend0/Textures/Portal_Menu_SD") != 2 {
		t.Fatal("existing HD counterpart not resolved")
	}
	if _, ok := p.TexturePath("Frontend0/Textures/missing_SD"); ok {
		t.Fatal("missing original asset fabricated")
	}
	p.manifest.Textures["frontend0/textures/portal_menu_sd"] = "textures/Portal_Menu_SD.png"
	if path, ok := p.TexturePath("Frontend0/Textures/Portal_Menu_SD"); !ok || path != "textures/Portal_Menu_SD.png" || p.TextureSourceScale("Frontend0/Textures/Portal_Menu_SD") != 1 {
		t.Fatal("existing SD overridden")
	}
	delete(p.manifest.Textures, "frontend0/textures/portal_menu_sd")
	p.manifest.Files["Common0/Xml/Common0_Sprites_SD.xml"] = "sd"
	if _, ok := p.TexturePath("Frontend0/Textures/Portal_Menu_SD"); ok {
		t.Fatal("ambiguous mixed catalog silently selected HD")
	}
}
