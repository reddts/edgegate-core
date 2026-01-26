package build_shared

import (
	"go/build"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"

	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/rw"
)

var (
	androidSDKPath string
	androidNDKPath string
)

const minAndroidNDKMajor = 26

func FindSDK() {
	searchPath := []string{
		"$ANDROID_HOME",
		"$HOME/Android/Sdk",
		"$HOME/.local/lib/android/sdk",
		"$HOME/Library/Android/sdk",
	}
	for _, path := range searchPath {
		path = os.ExpandEnv(path)
		if rw.FileExists(path + "/licenses/android-sdk-license") {
			androidSDKPath = path
			break
		}
	}
	if androidSDKPath == "" {
		log.Fatal("android SDK not found")
	}
	if !findNDK() {
		log.Fatal("android NDK not found")
	}

	os.Setenv("ANDROID_HOME", androidSDKPath)
	os.Setenv("ANDROID_SDK_HOME", androidSDKPath)
	os.Setenv("ANDROID_NDK_HOME", androidNDKPath)
	os.Setenv("NDK", androidNDKPath)
	os.Setenv("PATH", os.Getenv("PATH")+":"+filepath.Join(androidNDKPath, "toolchains", "llvm", "prebuilt", runtime.GOOS+"-x86_64", "bin"))
}

func findNDK() bool {
	if version := strings.TrimSpace(os.Getenv("ANDROID_NDK_VERSION")); version != "" {
		ndkPath := androidSDKPath + "/ndk/" + version
		if !rw.FileExists(ndkPath) {
			log.Fatalf("android NDK %s not found under %s", version, androidSDKPath+"/ndk")
		}
		if !ensureMinNDKVersion(version) {
			return false
		}
		androidNDKPath = ndkPath
		return true
	}

	if rw.FileExists(androidSDKPath + "/ndk/26.1.10909125") {
		if !ensureMinNDKVersion("26.1.10909125") {
			return false
		}
		androidNDKPath = androidSDKPath + "/ndk/26.1.10909125"
		return true
	}
	ndkVersions, err := os.ReadDir(androidSDKPath + "/ndk")
	if err != nil {
		return false
	}
	versionNames := common.Map(ndkVersions, os.DirEntry.Name)
	if len(versionNames) == 0 {
		return false
	}
	sort.Slice(versionNames, func(i, j int) bool {
		iVersions := strings.Split(versionNames[i], ".")
		jVersions := strings.Split(versionNames[j], ".")
		for k := 0; k < len(iVersions) && k < len(jVersions); k++ {
			iVersion, _ := strconv.Atoi(iVersions[k])
			jVersion, _ := strconv.Atoi(jVersions[k])
			if iVersion != jVersion {
				return iVersion > jVersion
			}
		}
		return true
	})
	for _, versionName := range versionNames {
		if rw.FileExists(androidSDKPath + "/ndk/" + versionName) {
			if !ensureMinNDKVersion(versionName) {
				continue
			}
			androidNDKPath = androidSDKPath + "/ndk/" + versionName
			return true
		}
	}
	return false
}

func ensureMinNDKVersion(versionName string) bool {
	major, ok := parseNDKMajor(versionName)
	if !ok {
		log.Fatalf("unable to parse android NDK version: %s", versionName)
	}
	if major < minAndroidNDKMajor {
		log.Fatalf("android NDK %s is too old; require >= %d", versionName, minAndroidNDKMajor)
	}
	return true
}

func parseNDKMajor(versionName string) (int, bool) {
	if strings.HasPrefix(versionName, "r") {
		versionName = strings.TrimPrefix(versionName, "r")
	}
	digits := strings.Builder{}
	for _, r := range versionName {
		if r < '0' || r > '9' {
			break
		}
		digits.WriteRune(r)
	}
	if digits.Len() == 0 {
		return 0, false
	}
	major, err := strconv.Atoi(digits.String())
	if err != nil {
		return 0, false
	}
	return major, true
}

var GoBinPath string

func FindMobile() {
	goBin := filepath.Join(build.Default.GOPATH, "bin")

	if runtime.GOOS == "windows" {
		if !rw.FileExists(goBin + "/" + "gobind.exe") {
			log.Fatal("missing gomobile.exe installation")
		}
	} else {
		if !rw.FileExists(goBin + "/" + "gobind") {
			log.Fatal("missing gomobile installation")
		}
	}
	GoBinPath = goBin
}
