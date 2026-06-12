package filesystem_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/krteke/River/internal/config"
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

func TestListReturnsRelativePathsAndFileTypes(t *testing.T) {
	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "movie.mp4"), "video")
	mustWriteFile(t, filepath.Join(root, "notes.md"), "text")
	mustWriteFile(t, filepath.Join(root, "archive.bin"), "other")
	if err := os.Mkdir(filepath.Join(root, "folder"), 0o755); err != nil {
		t.Fatal(err)
	}

	service := newService(t, root)
	list, err := service.List("media", "/")
	if err != nil {
		t.Fatal(err)
	}

	got := make(map[string]filesystem.ListItem, len(list.Items))
	for _, item := range list.Items {
		got[item.Name] = item
	}
	if got["movie.mp4"].Path != "/movie.mp4" || got["movie.mp4"].Type != filesystem.TypeVideo {
		t.Fatalf("unexpected video item: %+v", got["movie.mp4"])
	}
	if got["notes.md"].Type != filesystem.TypeText {
		t.Fatalf("unexpected text type: %q", got["notes.md"].Type)
	}
	if got["archive.bin"].Type != filesystem.TypeOther {
		t.Fatalf("unexpected other type: %q", got["archive.bin"].Type)
	}
	if got["folder"].Type != filesystem.TypeDirectory {
		t.Fatalf("unexpected directory type: %q", got["folder"].Type)
	}
}

func TestResolveRejectsParentTraversalAndExternalSymlink(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	mustWriteFile(t, filepath.Join(outside, "secret.txt"), "secret")
	if err := os.Symlink(outside, filepath.Join(root, "outside")); err != nil {
		t.Fatal(err)
	}

	service := newService(t, root)
	if _, _, err := service.File("media", "/../secret.txt"); !errors.Is(err, filesystem.ErrPathForbidden) {
		t.Fatalf("expected parent traversal to be forbidden, got %v", err)
	}
	if _, _, err := service.File("media", "/outside/secret.txt"); !errors.Is(err, filesystem.ErrPathForbidden) {
		t.Fatalf("expected external symlink to be forbidden, got %v", err)
	}
}

func TestResolveAllowsSymlinkWithinRoot(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "target"), 0o755); err != nil {
		t.Fatal(err)
	}
	mustWriteFile(t, filepath.Join(root, "target", "notes.txt"), "notes")
	if err := os.Symlink(filepath.Join(root, "target"), filepath.Join(root, "link")); err != nil {
		t.Fatal(err)
	}

	service := newService(t, root)
	file, _, err := service.File("media", "/link/notes.txt")
	if err != nil {
		t.Fatal(err)
	}
	file.Close()
}

func newService(t *testing.T, root string) *filesystem.Service {
	t.Helper()
	service, err := filesystem.NewService([]config.RootConfig{{ID: "media", Path: root}})
	if err != nil {
		t.Fatal(err)
	}
	return service
}

func mustWriteFile(t *testing.T, name, content string) {
	t.Helper()
	if err := os.WriteFile(name, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
