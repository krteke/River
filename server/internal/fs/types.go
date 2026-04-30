package filesystem

import "os"

type Root struct {
	ID       string
	Name     string
	Path     string
	RealPath string
}

type ResolvedPath struct {
	Root    Root
	RelPath string
	AbsPath string
	Info    os.FileInfo
}

type ListResponse struct {
	RootID string     `json:"root_id"`
	Path   string     `json:"path"`
	Parent string     `json:"parent"`
	Items  []ListItem `json:"items"`
}

type ListItem struct {
	Name  string `json:"name"`
	Path  string `json:"path"`
	Type  string `json:"type"`
	Size  int64  `json:"size"`
	MTime int64  `json:"mtime"`
}
