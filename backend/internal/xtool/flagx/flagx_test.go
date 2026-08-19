package flagx

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGetEnv(t *testing.T) {
	os.Setenv("FX_TEST", "v")
	if GetEnv("FX_TEST", "x") != "v" {
		t.Fatal("env")
	}
	if GetEnv("FX_MISSING", "d") != "d" {
		t.Fatal("def")
	}
	os.Unsetenv("FX_TEST")
}

func TestGetEnvInt(t *testing.T) {
	os.Setenv("FX_N", "42")
	if GetEnvInt("FX_N", 1) != 42 {
		t.Fatal("int")
	}
	if GetEnvInt("FX_X", 7) != 7 {
		t.Fatal("def")
	}
	os.Setenv("FX_BAD", "x")
	if GetEnvInt("FX_BAD", 8) != 8 {
		t.Fatal("bad")
	}
	os.Unsetenv("FX_N")
	os.Unsetenv("FX_BAD")
}

func TestGetEnvBool(t *testing.T) {
	os.Setenv("FX_B", "true")
	if !GetEnvBool("FX_B", false) {
		t.Fatal("bool")
	}
	if GetEnvBool("FX_X", true) != true {
		t.Fatal("def")
	}
	os.Unsetenv("FX_B")
}

func TestFileOrDefault(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "x")
	os.WriteFile(path, []byte("hello"), 0644)
	if FileOrDefault(path, "d") != "hello" {
		t.Fatal("file")
	}
	if FileOrDefault(filepath.Join(dir, "missing"), "d") != "d" {
		t.Fatal("def")
	}
}

func TestMustEnv_Panic(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("应 panic")
		}
	}()
	MustEnv("FX_NEVER_SET_12345")
}
