package platform

import (
	"cmp"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
)

// umuGameID turns off umu-launcher's per-game fixes: the ones for the
// Steam version of the game (umu-443110) do not suit this client.
const umuGameID = "umu-default"

var protonChoice struct {
	sync.Mutex
	path string
}

// SetProton picks the Proton build the game runs with; an empty path picks the newest one found.
func SetProton(path string) {
	protonChoice.Lock()
	protonChoice.path = path
	protonChoice.Unlock()
}

func chosenProton() string {
	protonChoice.Lock()
	defer protonChoice.Unlock()
	return protonChoice.path
}

// Protons lists the Proton builds of Steam and Heroic, best first.
func Protons() []Proton { return findProtons(HomeDir()) }

type runner struct {
	name   string
	argv   []string
	env    []string
	prefix string
}

func steamRoots(home string) []string {
	var roots []string
	for _, rel := range []string{".steam/root", ".steam/steam", ".local/share/Steam", ".var/app/com.valvesoftware.Steam/data/Steam", ".var/app/com.valvesoftware.Steam/.local/share/Steam"} {
		roots = appendDir(roots, filepath.Join(home, rel))
	}
	return roots
}

func appendDir(dirs []string, dir string) []string {
	real, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return dirs
	}
	if st, err := os.Stat(real); err != nil || !st.IsDir() || slices.Contains(dirs, real) {
		return dirs
	}
	return append(dirs, real)
}

var (
	vdfPath     = regexp.MustCompile(`"path"\s+"((?:[^"\\]|\\.)*)"`)
	vdfToolID   = regexp.MustCompile(`"require_tool_appid"\s+"(\d+)"`)
	vdfInstall  = regexp.MustCompile(`"installdir"\s+"((?:[^"\\]|\\.)*)"`)
	vdfUnescape = strings.NewReplacer(`\\`, `\`, `\"`, `"`)
)

func steamLibraries(roots []string) []string {
	var libs []string
	for _, root := range roots {
		libs = appendDir(libs, root)
		for _, file := range []string{"steamapps/libraryfolders.vdf", "config/libraryfolders.vdf"} {
			data, err := os.ReadFile(filepath.Join(root, file))
			if err != nil {
				continue
			}
			for _, m := range vdfPath.FindAllSubmatch(data, -1) {
				libs = appendDir(libs, vdfUnescape.Replace(string(m[1])))
			}
		}
	}
	return libs
}

type protonBuild struct {
	Proton
	rank    int
	version []int
}

var digits = regexp.MustCompile(`\d+`)

var provenProtons = []string{"Proton 10.0"}

func newProtonBuild(name, dir string) protonBuild {
	lower := strings.ToLower(name)
	// Builds the game is known to run on come first, then Valve's numbered
	// builds: with GE-Proton the game crashes when the garage loads, and with
	// Proton 11.0 it does not start.
	rank := 2
	switch {
	case slices.Contains(provenProtons, name):
		rank = -1
	case strings.Contains(lower, "experimental") || strings.Contains(lower, "hotfix"):
		rank = 3
	case strings.HasPrefix(lower, "proton "):
		rank = 0
	case strings.HasPrefix(lower, "ge-proton") || strings.HasPrefix(lower, "umu-proton") || strings.HasPrefix(lower, "proton-ge") || strings.HasSuffix(lower, "-ge") || strings.Contains(lower, "-ge-"):
		rank = 1
	}
	b := protonBuild{Proton: Proton{Name: name, Path: dir}, rank: rank}
	for _, d := range digits.FindAllString(name, -1) {
		n, _ := strconv.Atoi(d)
		b.version = append(b.version, n)
	}
	return b
}

func findProtons(home string) []Proton {
	roots := steamRoots(home)
	var dirs []string
	for _, root := range roots {
		dirs = append(dirs, filepath.Join(root, "compatibilitytools.d", "*"))
	}
	for _, lib := range steamLibraries(roots) {
		dirs = append(dirs, filepath.Join(lib, "steamapps", "common", "Proton*"))
	}
	for _, rel := range []string{".config/heroic/tools/proton/*", ".var/app/com.heroicgameslauncher.hgl/config/heroic/tools/proton/*"} {
		dirs = append(dirs, filepath.Join(home, rel))
	}
	var builds []protonBuild
	var seen []string
	for _, pattern := range dirs {
		matches, _ := filepath.Glob(pattern)
		for _, dir := range matches {
			name := filepath.Base(dir)
			if !isFile(filepath.Join(dir, "proton")) || slices.Contains(seen, name) {
				continue
			}
			seen = append(seen, name)
			builds = append(builds, newProtonBuild(name, dir))
		}
	}
	slices.SortStableFunc(builds, func(a, b protonBuild) int {
		if c := cmp.Compare(a.rank, b.rank); c != 0 {
			return c
		}
		if c := slices.Compare(b.version, a.version); c != 0 {
			return c
		}
		return strings.Compare(a.Name, b.Name)
	})
	protons := make([]Proton, len(builds))
	for i, b := range builds {
		protons[i] = b.Proton
	}
	return protons
}

func isFile(path string) bool {
	st, err := os.Stat(path)
	return err == nil && st.Mode().IsRegular()
}

// steamRuntime finds the Steam Linux Runtime container a Proton build asks for in its toolmanifest.vdf.
func steamRuntime(proton string, libs []string) string {
	data, err := os.ReadFile(filepath.Join(proton, "toolmanifest.vdf"))
	if err != nil {
		return ""
	}
	m := vdfToolID.FindSubmatch(data)
	if m == nil {
		return ""
	}
	id := string(m[1])
	for _, lib := range libs {
		data, err := os.ReadFile(filepath.Join(lib, "steamapps", "appmanifest_"+id+".acf"))
		if err != nil {
			continue
		}
		if dir := vdfInstall.FindSubmatch(data); dir != nil {
			entry := filepath.Join(lib, "steamapps", "common", vdfUnescape.Replace(string(dir[1])), "_v2-entry-point")
			if isFile(entry) {
				return entry
			}
		}
	}
	known := map[string]string{"1628350": "SteamLinuxRuntime_sniper", "1391110": "SteamLinuxRuntime_soldier"}[id]
	for _, lib := range libs {
		if entry := filepath.Join(lib, "steamapps", "common", known, "_v2-entry-point"); known != "" && isFile(entry) {
			return entry
		}
	}
	return ""
}

func findUmu(home string, lookPath func(string) (string, error)) []string {
	if path, err := lookPath("umu-run"); err == nil {
		return []string{path}
	}
	for _, rel := range []string{
		".local/bin/umu-run",
		".local/share/lutris/runtime/umu/umu-run",
		".var/app/net.lutris.Lutris/data/lutris/runtime/umu/umu-run",
		".config/heroic/tools/runtimes/umu/umu_run.py",
		".var/app/com.heroicgameslauncher.hgl/config/heroic/tools/runtimes/umu/umu_run.py",
	} {
		path := filepath.Join(home, rel)
		if !isFile(path) {
			continue
		}
		if strings.HasSuffix(path, ".py") {
			python, err := lookPath("python3")
			if err != nil {
				continue
			}
			return []string{python, path}
		}
		return []string{path}
	}
	return nil
}

var unsafeName = regexp.MustCompile(`[^A-Za-z0-9._-]+`)

// gamePrefix is WINEPREFIX, or a prefix of the launcher's own: one for each
// Proton build, since a prefix that another Proton version has upgraded can
// crash the game.
func gamePrefix(proton *Proton) (string, error) {
	if prefix := os.Getenv("WINEPREFIX"); prefix != "" {
		return prefix, os.MkdirAll(prefix, 0755)
	}
	dir, err := DataDir()
	if err != nil {
		return "", err
	}
	prefix := filepath.Join(dir, "prefix")
	if proton != nil {
		prefix = filepath.Join(dir, "prefixes", strings.Trim(unsafeName.ReplaceAllString(proton.Name, "-"), "-."))
	}
	return prefix, os.MkdirAll(prefix, 0755)
}

// linkPfx lets Proton, which keeps its prefix in pfx, umu-launcher and Wine share one folder.
func linkPfx(prefix string) error {
	pfx := filepath.Join(prefix, "pfx")
	if _, err := os.Lstat(pfx); err == nil {
		return nil
	}
	return os.Symlink(".", pfx)
}

func setDefault(env []string, key, value string) []string {
	if os.Getenv(key) != "" {
		return env
	}
	return append(env, key+"="+value)
}

func joinPaths(key string, paths ...string) string {
	if old := os.Getenv(key); old != "" {
		paths = append([]string{old}, paths...)
	}
	return key + "=" + strings.Join(paths, ":")
}

// gameRunner picks how to run a Windows program from dir: AWLAUNCHER_RUNNER,
// umu-launcher, Proton from Steam or Heroic (in its Steam Linux Runtime when
// that is installed), then Wine.
func gameRunner(home, dir string, lookPath func(string) (string, error)) (runner, error) {
	if custom := strings.Fields(os.Getenv("AWLAUNCHER_RUNNER")); len(custom) > 0 {
		prefix, err := gamePrefix(nil)
		if err != nil {
			return runner{}, err
		}
		return runner{name: filepath.Base(custom[0]), argv: custom, env: setDefault(os.Environ(), "WINEPREFIX", prefix), prefix: prefix}, nil
	}
	protons := findProtons(home)
	var proton *Proton
	for i := range protons {
		if protons[i].Path == chosenProton() {
			proton = &protons[i]
		}
	}
	if proton == nil && len(protons) > 0 {
		proton = &protons[0]
	}
	umu := findUmu(home, lookPath)
	used := proton
	if umu != nil && os.Getenv("PROTONPATH") != "" {
		used = nil
	}
	prefix, err := gamePrefix(used)
	if err != nil {
		return runner{}, err
	}
	env := setDefault(os.Environ(), "WINEPREFIX", prefix)
	if umu != nil {
		if err := linkPfx(prefix); err != nil {
			return runner{}, err
		}
		env = setDefault(env, "GAMEID", umuGameID)
		env = setDefault(env, "STORE", "none")
		name := "umu-launcher"
		if proton != nil && os.Getenv("PROTONPATH") == "" {
			env = append(env, "PROTONPATH="+proton.Path)
			name += " with " + proton.Name
		}
		return runner{name: name, argv: umu, env: env, prefix: prefix}, nil
	}
	if proton != nil {
		if err := linkPfx(prefix); err != nil {
			return runner{}, err
		}
		roots := steamRoots(home)
		client := prefix
		if len(roots) > 0 {
			client = roots[0]
		}
		env = setDefault(env, "STEAM_COMPAT_DATA_PATH", prefix)
		env = setDefault(env, "STEAM_COMPAT_CLIENT_INSTALL_PATH", client)
		env = setDefault(env, "STEAM_COMPAT_INSTALL_PATH", dir)
		argv := []string{filepath.Join(proton.Path, "proton"), "waitforexitandrun"}
		name := proton.Name
		if entry := steamRuntime(proton.Path, steamLibraries(roots)); entry != "" {
			env = append(env, joinPaths("STEAM_COMPAT_TOOL_PATHS", proton.Path, filepath.Dir(entry)), joinPaths("STEAM_COMPAT_MOUNTS", dir, prefix))
			argv = append([]string{entry, "--verb=waitforexitandrun", "--"}, argv...)
			name += " in " + filepath.Base(filepath.Dir(entry))
		}
		return runner{name: name, argv: argv, env: env, prefix: prefix}, nil
	}
	if wine, err := lookPath("wine"); err == nil {
		return runner{name: "Wine", argv: []string{wine}, env: env, prefix: prefix}, nil
	}
	return runner{}, errors.New("nothing can run the Windows game here: install Proton in Steam (or GE-Proton), umu-launcher or Wine, or set AWLAUNCHER_RUNNER")
}
