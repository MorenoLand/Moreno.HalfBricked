package content

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/MorenoLand/Moreno.HalfBricked/engine/formats"
)

type PackManifest struct {
	SchemaVersion int                        `json:"schemaVersion"`
	SourceVersion string                     `json:"sourceVersion"`
	Levels        []formats.LevelInfo        `json:"levels"`
	ScriptLevels  map[string]string          `json:"scriptLevels,omitempty"`
	TileSets      map[string]formats.TileSet `json:"tileSets"`
	Textures      map[string]string          `json:"textures"`
	Files         map[string]string          `json:"files"`
	WaveBuild     string                     `json:"-"`
}

// Builds whose wave rules were recovered from libmortargame.so (Research/native/wave-advance-2026-10-09.md).
// Each data cache follows the build it was imported from.
const (
	WaveBuildV7  = "1.2.1" // SD cache: libmortargame.so (v7), the 1.2.1 build
	WaveBuild125 = "1.2.5" // HD cache: the 1.2.5 build
)

// apkSourceBuilds maps the SHA-256 of an imported APK (written by PrepareAPK to .apk-source.sha256) to its build.
var apkSourceBuilds = map[string]string{
	"db68e6368192520b2d68c7af49ad90d6e66e2a7f57dacddd7b20607e73b8836a": WaveBuild125,
	"eed8fd097359d60b5142f4000ad11c885cb3146547ba717268c8e5ad2fef958c": WaveBuildV7,
}

// waveBuildOf reads the build of a cache from its .apk-source.sha256 marker. A cache whose marker is missing
// or unknown follows the 1.2.5 rule (the HD cache is the default data set).
func waveBuildOf(source AssetSource) string {
	r, err := source.Open(".apk-source.sha256")
	if err != nil {
		return WaveBuild125
	}
	defer r.Close()
	data, err := io.ReadAll(io.LimitReader(r, 256))
	if err != nil {
		return WaveBuild125
	}
	if fields := strings.Fields(string(data)); len(fields) > 0 {
		if build, ok := apkSourceBuilds[strings.ToLower(fields[0])]; ok {
			return build
		}
	}
	return WaveBuild125
}

type AssetSource interface {
	Open(path string) (io.ReadCloser, error)
	Manifest() (PackManifest, error)
}
type LevelRepository interface {
	List() []formats.LevelInfo
	Load(id string) (formats.Level, error)
}

type Pack struct {
	source          AssetSource
	audioOnce       sync.Once
	audioIndex      map[string]string
	manifest        PackManifest
	levels          map[string]formats.Level
	variables       formats.FrontendVariables
	weapons         formats.WeaponCatalog
	weaponErr       error
	weaponsOK       bool
	zombieWeapons   formats.ZombieWeaponCatalog
	zombieWeaponErr error
	zombieWeaponsOK bool
	sprites         formats.SpriteCatalog
	spriteErr       error
	spritesOK       bool
}

func NewPack(source AssetSource) (*Pack, error) {
	manifest, err := source.Manifest()
	if err != nil {
		return nil, err
	}
	if manifest.SchemaVersion != 1 {
		return nil, fmt.Errorf("unsupported cache schema %d", manifest.SchemaVersion)
	}
	manifest.WaveBuild = waveBuildOf(source)
	if err := hydrateLevelMetadata(source, &manifest); err != nil {
		return nil, err
	}
	pack := &Pack{source: source, manifest: manifest, levels: make(map[string]formats.Level)}
	if path, ok := manifestPath(manifest.Files, "Common0/Xml/Common0_Variables.xml"); ok {
		r, err := source.Open(path)
		if err != nil {
			return nil, err
		}
		pack.variables, err = formats.ParseVariables(r)
		r.Close()
		if err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
	}
	return pack, nil
}
func (p *Pack) Manifest() PackManifest                { return p.manifest }
func (p *Pack) Variables() formats.FrontendVariables  { return p.variables }
func (p *Pack) SourcePath(path string) (string, bool) { return manifestPath(p.manifest.Files, path) }
func (p *Pack) List() []formats.LevelInfo {
	return append([]formats.LevelInfo(nil), p.manifest.Levels...)
}
func (p *Pack) Load(id string) (formats.Level, error) {
	if level, ok := p.levels[id]; ok {
		return level, nil
	}
	r, err := p.source.Open(filepath.ToSlash(filepath.Join("levels", id+".json")))
	if err != nil {
		return formats.Level{}, err
	}
	defer r.Close()
	var level formats.Level
	if err := json.NewDecoder(r).Decode(&level); err != nil {
		return formats.Level{}, err
	}
	if err := level.Validate(); err != nil {
		return formats.Level{}, err
	}
	for _, info := range p.manifest.Levels {
		if strings.EqualFold(info.ID, id) {
			level.Info = info
			break
		}
	}
	level.WaveBuild = p.manifest.WaveBuild
	p.levels[id] = level
	return level, nil
}
func (p *Pack) LoadScriptLevel(name string, world int) (formats.Level, error) {
	for _, info := range p.manifest.Levels {
		if strings.EqualFold(info.ID, name) || strings.EqualFold(info.BaseFile, name) {
			return p.Load(info.ID)
		}
	}
	key := fmt.Sprintf("%s:%d", strings.ToLower(strings.TrimSpace(name)), world)
	if id, ok := p.manifest.ScriptLevels[key]; ok {
		return p.Load(id)
	}
	return p.Load(name)
}
func (p *Pack) TexturePath(name string) (string, bool) {
	pathName := strings.ToLower(strings.TrimSuffix(filepath.ToSlash(name), filepath.Ext(name)))
	if path, ok := p.manifest.Textures[pathName]; ok {
		return path, true
	}
	baseName := strings.ToLower(strings.TrimSuffix(filepath.Base(name), filepath.Ext(name)))
	path, ok := p.manifest.Textures[baseName]
	if !ok && strings.HasSuffix(pathName, "_sd") {
		_, hasSD := p.SourcePath("Common0/Xml/Common0_Sprites_SD.xml")
		_, hasHD := p.SourcePath("Common0/Xml/Common0_Sprites_HD.xml")
		if !hasSD && hasHD {
			hdName := strings.TrimSuffix(pathName, "_sd") + "_hd"
			path, ok = p.manifest.Textures[hdName]
			if !ok {
				path, ok = p.manifest.Textures[filepath.Base(hdName)]
			}
		}
	}
	return path, ok
}
func (p *Pack) TextureSourceScale(name string) float64 {
	path, ok := p.TexturePath(name)
	if ok && strings.HasSuffix(strings.ToLower(strings.TrimSuffix(path, filepath.Ext(path))), "_hd") {
		return 2
	}
	return 1
}
func (p *Pack) Open(path string) (io.ReadCloser, error) {
	reader, err := p.source.Open(path)
	if err == nil {
		return reader, nil
	}
	if alternate, ok := p.audioFallback(path); ok {
		return p.source.Open(alternate)
	}
	return nil, err
}

// audioFallback maps the audio paths used by the older cache layout
// ("audio/sound/sfx/x.ogg", "audio/music/sound/x.ogg") to the same file in a
// per-package layout ("audio/Common0/Sound/SFX/x.ogg") by case-insensitive name.
func (p *Pack) audioFallback(path string) (string, bool) {
	slashed := filepath.ToSlash(path)
	if !strings.HasPrefix(strings.ToLower(slashed), "audio/") || !strings.EqualFold(filepath.Ext(slashed), ".ogg") {
		return "", false
	}
	p.audioOnce.Do(func() {
		keys := make([]string, 0, len(p.manifest.Files))
		for key := range p.manifest.Files {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		p.audioIndex = map[string]string{}
		for _, key := range keys {
			value := p.manifest.Files[key]
			if !strings.EqualFold(filepath.Ext(value), ".ogg") {
				continue
			}
			base := strings.ToLower(strings.TrimSuffix(filepath.Base(filepath.ToSlash(value)), filepath.Ext(value)))
			if _, taken := p.audioIndex[base]; !taken {
				p.audioIndex[base] = value
			}
		}
	})
	wanted := strings.ToLower(strings.TrimSuffix(filepath.Base(slashed), filepath.Ext(slashed)))
	alternate, ok := p.audioIndex[wanted]
	return alternate, ok && !strings.EqualFold(alternate, slashed)
}
func (p *Pack) Script(path string) (formats.Script, error) {
	name, ok := manifestPath(p.manifest.Files, path)
	if !ok {
		return formats.Script{}, fmt.Errorf("script %q not found", path)
	}
	r, err := p.source.Open(name)
	if err != nil {
		return formats.Script{}, err
	}
	defer r.Close()
	script, err := formats.ParseScript(r)
	if err != nil {
		return formats.Script{}, fmt.Errorf("%s: %w", name, err)
	}
	return script, nil
}
func (p *Pack) ScriptSource(path string) (string, error) {
	name, ok := manifestPath(p.manifest.Files, path)
	if !ok {
		return "", fmt.Errorf("script %q not found", path)
	}
	r, err := p.source.Open(name)
	if err != nil {
		return "", err
	}
	defer r.Close()
	source, err := io.ReadAll(r)
	if err != nil {
		return "", fmt.Errorf("%s: %w", name, err)
	}
	return string(source), nil
}
func (p *Pack) Conversation(world int, name string) (formats.Conversation, error) {
	paths, missing := p.conversationFiles(world)
	for _, path := range paths {
		r, err := p.source.Open(path)
		if err != nil {
			return formats.Conversation{}, err
		}
		conversations, parseErr := formats.ParseDialog(r)
		r.Close()
		if parseErr != nil {
			return formats.Conversation{}, fmt.Errorf("%s: %w", path, parseErr)
		}
		for _, conversation := range conversations {
			if strings.EqualFold(conversation.Name, name) {
				return conversation, nil
			}
		}
	}
	if len(missing) != 0 {
		return formats.Conversation{}, fmt.Errorf("conversation %q for world %d not found; missing declared dialog XML: %s", name, world, strings.Join(missing, ", "))
	}
	return formats.Conversation{}, fmt.Errorf("conversation %q for world %d not found", name, world)
}

func (p *Pack) conversationFiles(world int) ([]string, []string) {
	keys := make([]string, 0, len(p.manifest.Files))
	for key := range p.manifest.Files {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	paths, missing := []string{}, []string{}
	seenNames, seenPaths := map[string]bool{}, map[string]bool{}
	for _, level := range p.manifest.Levels {
		if level.WorldIndex != world {
			continue
		}
		for _, name := range level.ConversationXMLs {
			name = strings.TrimSpace(name)
			if name == "" {
				continue
			}
			base := strings.TrimSuffix(filepath.Base(name), filepath.Ext(name))
			nameKey := strings.ToLower(base)
			if seenNames[nameKey] {
				continue
			}
			seenNames[nameKey] = true
			filename := strings.ToLower(base + ".xml")
			matches := []string{}
			for _, key := range keys {
				parts := strings.Split(strings.ToLower(filepath.ToSlash(key)), "/")
				if len(parts) < 2 || parts[len(parts)-2] != "dialog" || parts[len(parts)-1] != filename {
					continue
				}
				matches = append(matches, p.manifest.Files[key])
			}
			if len(matches) == 0 {
				missing = append(missing, name)
				continue
			}
			for _, path := range matches {
				if !seenPaths[path] {
					paths = append(paths, path)
					seenPaths[path] = true
				}
			}
		}
	}
	return paths, missing
}
func (p *Pack) Weapons() (formats.WeaponCatalog, error) {
	if p.weaponsOK {
		return append(formats.WeaponCatalog(nil), p.weapons...), p.weaponErr
	}
	p.weaponsOK = true
	path, ok := p.SourcePath("Common0/Xml/Common0_Weapons.xml")
	if !ok {
		p.weaponErr = fmt.Errorf("weapon catalog not found")
		return nil, p.weaponErr
	}
	r, err := p.source.Open(path)
	if err != nil {
		p.weaponErr = err
		return nil, err
	}
	defer r.Close()
	p.weapons, p.weaponErr = formats.ParseWeapons(r)
	if p.weaponErr != nil {
		p.weaponErr = fmt.Errorf("%s: %w", path, p.weaponErr)
	}
	if p.weaponErr == nil {
		// Later packages add weapons: Common1 buzzsaw, Common2 sentry guns, DLC1 dual pistol and cow-pat bomb.
		extras := make([]string, 0, 4)
		for key := range p.manifest.Files {
			lower := strings.ToLower(key)
			if strings.HasSuffix(lower, "_weapons.xml") && !strings.HasPrefix(lower, "common0/") {
				extras = append(extras, key)
			}
		}
		sort.Strings(extras)
		for _, key := range extras {
			path, ok := p.SourcePath(key)
			if !ok {
				continue
			}
			rc, err := p.source.Open(path)
			if err != nil {
				continue
			}
			more, err := formats.ParseWeapons(rc)
			rc.Close()
			if err != nil {
				p.weaponErr = fmt.Errorf("%s: %w", path, err)
				break
			}
			for _, weapon := range more {
				if _, exists := p.weapons.Find(weapon.GunType); !exists {
					p.weapons = append(p.weapons, weapon)
				}
			}
		}
	}
	return append(formats.WeaponCatalog(nil), p.weapons...), p.weaponErr
}
func (p *Pack) ZombieWeapons() (formats.ZombieWeaponCatalog, error) {
	if p.zombieWeaponsOK {
		return append(formats.ZombieWeaponCatalog(nil), p.zombieWeapons...), p.zombieWeaponErr
	}
	p.zombieWeaponsOK = true
	keys := make([]string, 0, len(p.manifest.Files))
	for key := range p.manifest.Files {
		if strings.HasSuffix(strings.ToLower(filepath.Base(filepath.ToSlash(key))), "_manifest.xml") {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	seen := map[string]bool{}
	for _, key := range keys {
		path := p.manifest.Files[key]
		r, err := p.source.Open(path)
		if err != nil {
			p.zombieWeaponErr = fmt.Errorf("%s: %w", path, err)
			return nil, p.zombieWeaponErr
		}
		var packageManifest struct {
			XMLInfo []struct {
				Attributes []xml.Attr `xml:",any,attr"`
			} `xml:"XmlInfo"`
		}
		err = xml.NewDecoder(r).Decode(&packageManifest)
		r.Close()
		if err != nil {
			p.zombieWeaponErr = fmt.Errorf("%s: %w", path, err)
			return nil, p.zombieWeaponErr
		}
		for _, info := range packageManifest.XMLInfo {
			for _, attribute := range info.Attributes {
				if !strings.EqualFold(attribute.Name.Local, "zombieWeapon") {
					continue
				}
				name := strings.TrimSpace(attribute.Value)
				if name == "" {
					continue
				}
				if filepath.Ext(name) == "" {
					name += ".xml"
				}
				wanted := filepath.ToSlash(filepath.Join(filepath.Dir(key), name))
				weaponPath, ok := manifestPath(p.manifest.Files, wanted)
				if !ok || seen[strings.ToLower(filepath.ToSlash(weaponPath))] {
					continue
				}
				seen[strings.ToLower(filepath.ToSlash(weaponPath))] = true
				weaponXML, err := p.source.Open(weaponPath)
				if err != nil {
					p.zombieWeaponErr = fmt.Errorf("%s: %w", weaponPath, err)
					return nil, p.zombieWeaponErr
				}
				catalog, parseErr := formats.ParseZombieWeapons(weaponXML)
				weaponXML.Close()
				if parseErr != nil {
					p.zombieWeaponErr = fmt.Errorf("%s: %w", weaponPath, parseErr)
					return nil, p.zombieWeaponErr
				}
				p.zombieWeapons = append(p.zombieWeapons, catalog...)
			}
		}
	}
	return append(formats.ZombieWeaponCatalog(nil), p.zombieWeapons...), nil
}
func (p *Pack) Sprites() (formats.SpriteCatalog, error) {
	if p.spritesOK {
		return p.sprites, p.spriteErr
	}
	p.spritesOK = true
	path, ok := p.SourcePath("Common0/Xml/Common0_Sprites_SD.xml")
	if !ok {
		path, ok = p.SourcePath("Common0/Xml/Common0_Sprites_HD.xml")
	}
	if !ok {
		p.spriteErr = fmt.Errorf("sprite catalog not found")
		return nil, p.spriteErr
	}
	r, err := p.source.Open(path)
	if err != nil {
		p.spriteErr = err
		return nil, err
	}
	defer r.Close()
	p.sprites, p.spriteErr = formats.ParseSprites(r)
	if p.spriteErr != nil {
		p.spriteErr = fmt.Errorf("%s: %w", path, p.spriteErr)
	}
	return p.sprites, p.spriteErr
}
func manifestPath(files map[string]string, wanted string) (string, bool) {
	for key, path := range files {
		if strings.EqualFold(filepath.ToSlash(key), filepath.ToSlash(wanted)) {
			return path, true
		}
	}
	return "", false
}
