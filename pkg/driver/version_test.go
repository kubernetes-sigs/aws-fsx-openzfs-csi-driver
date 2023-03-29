package driver

import (
	"fmt"
	"reflect"
	"runtime"
	"testing"
)

func TestGetVersion(t *testing.T) {
	version := GetVersion()

	expected := VersionInfo{
		DriverVersion: "",
		GitCommit:     "",
		BuildDate:     "",
		GoVersion:     runtime.Version(),
		Compiler:      runtime.Compiler,
		Platform:      fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH),
	}

	if !reflect.DeepEqual(version, expected) {
		t.Fatalf("structs not equal\ngot:\n%+v\nexpected: \n%+v", version, expected)
	}
}
func TestGetVersionJSON(t *testing.T) {
	version, err := GetVersionJSON()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := fmt.Sprintf(`{
  "driverVersion": "",
  "gitCommit": "",
  "buildDate": "",
  "goVersion": "%s",
  "compiler": "%s",
  "platform": "%s"
}`, runtime.Version(), runtime.Compiler, fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH))

	if version != expected {
		t.Fatalf("json not equal\ngot:\n%s\nexpected:\n%s", version, expected)
	}
}
