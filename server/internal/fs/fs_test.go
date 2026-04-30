package filesystem_test

import (
	"testing"

	filesystem "github.com/krteke/River/internal/fs"
)

func TestPathClean(t *testing.T) {
	p1 := "/../path"
	p2 := "."
	p3 := ""
	p4 := "//path"
	p5 := "path"

	cleaned1 := filesystem.CleanPath(p1)
	cleaned2 := filesystem.CleanPath(p2)
	cleaned3 := filesystem.CleanPath(p3)
	cleaned4 := filesystem.CleanPath(p4)
	cleaned5 := filesystem.CleanPath(p5)

	if cleaned1 != "/path" {
		t.Errorf("expected /path, got %s", cleaned1)
	}
	if cleaned2 != "/" {
		t.Errorf("expected /, got %s", cleaned2)
	}
	if cleaned3 != "/" {
		t.Errorf("expected /, got %s", cleaned3)
	}
	if cleaned4 != "/path" {
		t.Errorf("expected /path, got %s", cleaned4)
	}
	if cleaned5 != "/path" {
		t.Errorf("expected /path, got %s", cleaned5)
	}
}

func TestJoinReal(t *testing.T) {
	base1 := "/path"
	name1 := "file.txt"
	expected1 := "/path/file.txt"
	result1 := filesystem.JoinReal(base1, name1)

	base2 := "/"
	name2 := "file.txt"
	expected2 := "/file.txt"
	result2 := filesystem.JoinReal(base2, name2)

	base3 := "/"
	name3 := "/file.txt"
	expected3 := "/file.txt"
	result3 := filesystem.JoinReal(base3, name3)

	if result1 != expected1 {
		t.Errorf("expected %s, got %s", expected1, result1)
	}
	if result2 != expected2 {
		t.Errorf("expected %s, got %s", expected2, result2)
	}
	if result3 != expected3 {
		t.Errorf("expected %s, got %s", expected3, result3)
	}
}
