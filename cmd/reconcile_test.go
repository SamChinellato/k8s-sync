package cmd

import (
	"bytes"
	"fmt"
	"k8s.io/apimachinery/pkg/util/yaml"
	"reflect"
	"testing"
)

func TestFileCheck(t *testing.T) {
	// arrange
	file := "../tests/test-manifests/test.yaml"
	//act
	got := reflect.TypeOf(FileCheck(file)).String()
	want := "*os.fileStat"
	//assert
	if got != want {
		t.Errorf("got %q want %q", got, want)
	}
}

func TestFileToBytes(t *testing.T) {
	// arrange
	file := "../tests/test-manifests/test.yaml"
	//act
	got, _ := FileToBytes(file)
	gotType := reflect.TypeOf(got).String()
	wantType := "[]uint8"
	//assert
	if gotType != wantType {
		t.Errorf("got %q want %q", gotType, wantType)
	}
}

func TestAddTrailingSlash(t *testing.T) {
	//arrange
	filename := "./test-manifests"
	//act
	got := AddTrailingSlash(filename)
	want := "./test-manifests/"
	//assert
	if got != want {
		t.Errorf("got %q want %q", got, want)
	}
}

func TestDirToBytes(t *testing.T) {
	//arrange
	dir := "../tests/test-manifests/"
	//act
	got, _ := DirToBytes(dir)
	gotType := reflect.TypeOf(got).String()
	wantType := "[][]uint8"
	gotLen := len(got)
	wantLen := 4
	//assert
	if gotType != wantType {
		t.Errorf("got %q want %q", gotType, wantType)
	}
	if gotLen != wantLen {
		t.Errorf("got %q want %q", gotLen, wantLen)
	}
}

func TestFileBytesToUnstructuredObjGVKMap(t *testing.T) {
	filename := "../tests/test-manifests/test.yaml"
	fileBytes, _ := FileToBytes(filename)
	fmt.Println(fileBytes)
	decoder := yaml.NewYAMLOrJSONDecoder(bytes.NewReader(fileBytes), 100)

	got := FileBytesToUnstructuredObjGVKMap(decoder, fileBytes)

	gotType := reflect.TypeOf(got).String()
	wantType := "map[*schema.GroupVersionKind]*unstructured.Unstructured"
	if gotType != wantType {
		t.Errorf("got %q want %q", gotType, wantType)
	}
	gotLen := len(got)
	fmt.Println(got)
	wantLen := 4
	if gotLen != wantLen {
		t.Errorf("got %q want %q", gotLen, wantLen)
	}
}
