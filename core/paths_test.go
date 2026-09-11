package core

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestPreparePrivateDirPermissions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "parent", "data")
	got, err := preparePrivateDir(path)
	if err != nil || got != path {
		t.Fatalf("preparePrivateDir = %q, %v", got, err)
	}
	assertWritablePrivateDir(t, path)
	if _, err := preparePrivateDir(filepath.Join(path, "probe", "invalid")); err == nil {
		t.Fatal("accepted a regular file as a parent directory")
	}
}

func assertWritablePrivateDir(t *testing.T, path string) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0700 {
		t.Fatalf("%s permissions = %04o, want 0700", path, info.Mode().Perm())
	}
	if err := os.WriteFile(filepath.Join(path, "probe"), []byte("writable"), 0600); err != nil {
		t.Fatal(err)
	}
}

func TestResolveResourceDir(t *testing.T) {
	root := t.TempDir()
	install := filepath.Join(root, "bin")
	cwd := filepath.Join(root, "work")
	gp1, gp2 := filepath.Join(root, "go1"), filepath.Join(root, "go2")
	gopath := strings.Join([]string{gp1, gp2}, string(os.PathListSeparator))
	mkdir := func(p string) {
		t.Helper()
		if err := os.MkdirAll(p, 0755); err != nil {
			t.Fatal(err)
		}
	}
	check := func(want string) {
		t.Helper()
		if got := resolveResourceDir(install, cwd, gopath, ""); got != want {
			t.Fatalf("resource directory = %q, want %q", got, want)
		}
	}
	check(install)
	mkdir(filepath.Join(gp2, "bin", "icon"))
	check(filepath.Join(gp2, "bin"))
	mkdir(filepath.Join(cwd, "icon"))
	check(cwd)
	mkdir(filepath.Join(install, "icon"))
	check(install)
	bundle := filepath.Join(root, "App.app", "Contents", "Resources")
	mkdir(bundle)
	install = filepath.Join(filepath.Dir(bundle), "MacOS")
	check(bundle)
	override := filepath.Join(root, "explicit assets")
	if got := resolveResourceDir(install, cwd, gopath, override); got != override {
		t.Fatalf("explicit override ignored: %s", got)
	}
}

// Run a fresh consumer process: core.init executes before TestMain/t.Setenv,
// so an in-process environment change cannot test startup path isolation.
func TestSDKReadOnlyInstallation(t *testing.T) {
	for _, portable := range []bool{false, true} {
		name := "per-user"
		if portable {
			name = "explicit-portable"
		}
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			install := filepath.Join(root, "read-only-install")
			if err := os.Mkdir(install, 0755); err != nil {
				t.Fatal(err)
			}
			bin := filepath.Join(install, "sdk-consumer")
			if runtime.GOOS == "windows" {
				bin += ".exe"
			}
			self, err := os.Executable()
			if err != nil {
				t.Fatal(err)
			}
			if err := copyFileContents(bin, self); err != nil {
				t.Fatal(err)
			}
			if err := os.Chmod(bin, 0555); err != nil {
				t.Fatal(err)
			}
			if err := os.Chmod(install, 0555); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = os.Chmod(install, 0755) })
			cmd := exec.Command(bin, "-test.run=^TestSDKPathsChild$", "-test.v")
			cmd.Dir = root
			for _, e := range os.Environ() {
				key, _, _ := strings.Cut(e, "=")
				switch strings.ToUpper(key) {
				case "HOME", "APPDATA", "LOCALAPPDATA", "XDG_CONFIG_HOME", "SILK_DATA_DIR", "SILK_WORKSPACE_DIR", "SILK_RESOURCE_DIR", "SILK_PATHS_CHILD", "SILK_DEBUG":
					continue
				}
				cmd.Env = append(cmd.Env, e)
			}
			home := filepath.Join(root, "home")
			cmd.Env = append(cmd.Env, "HOME="+home, "APPDATA="+home, "LOCALAPPDATA="+home,
				"XDG_CONFIG_HOME="+home, "SILK_RESOURCE_DIR="+install, "SILK_PATHS_CHILD=1", "SILK_DEBUG=false")
			if portable {
				cmd.Env = append(cmd.Env, "SILK_DATA_DIR="+filepath.Join(root, "portable", "data"))
			}
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("consumer: %v\n%s", err, out)
			}
			entries, err := os.ReadDir(install)
			if err != nil || len(entries) != 1 {
				t.Fatalf("SDK wrote into installation: %v, %v", entries, err)
			}
		})
	}
}

func TestSDKPathsChild(t *testing.T) {
	if os.Getenv("SILK_PATHS_CHILD") != "1" {
		t.Skip("subprocess helper")
	}
	defer Close()
	want := os.Getenv("SILK_DATA_DIR")
	if want == "" {
		base, err := os.UserConfigDir()
		if err != nil {
			t.Fatal(err)
		}
		want = filepath.Join(base, "silk", "sdk-consumer")
	}
	if LocalDataDir() != want || ResourceDir() != os.Getenv("SILK_RESOURCE_DIR") {
		t.Fatalf("unexpected paths: data=%q resources=%q", LocalDataDir(), ResourceDir())
	}
	assertWritablePrivateDir(t, LocalDataDir())
	assertWritablePrivateDir(t, WorkspaceDir())
	assertWritablePrivateDir(t, TempDir())
	defer os.RemoveAll(TempDir())
	ini := UserIni()
	if err := ini.WriteAttr("consumer_value", 42); err != nil {
		t.Fatal(err)
	}
	ini.Sync()
	doc, err := LoadTDocFile(filepath.Join(LocalDataDir(), "ini.cml"))
	if err != nil {
		t.Fatal(err)
	}
	var value int
	if err := doc.ReadAttr("consumer_value", &value); err != nil || value != 42 {
		t.Fatalf("user configuration did not persist: %d, %v", value, err)
	}
	Close()
	if _, err := os.Stat(TempDir()); !os.IsNotExist(err) {
		t.Fatalf("process-private temporary directory survived Close: %v", err)
	}
	if _, err := os.Stat(LocalDataDir()); err != nil {
		t.Fatalf("Close removed persistent user data: %v", err)
	}
}

func TestReadOnlyIniDoesNotCreateMissingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing.cml")
	ini := LoadTDocIni(path, true)
	var value int
	_ = ini.ReadAttr("unknown", &value)
	ini.Sync()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("loading a missing read-only ini wrote a file: %v", err)
	}
	if err := ini.WriteAttr("unknown", 1); err != errIniReadOnly {
		t.Fatalf("read-only write = %v", err)
	}
}
