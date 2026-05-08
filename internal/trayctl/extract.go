package trayctl

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"io"
	"path/filepath"
	"strings"
)

func extractAllFromTarGz(data []byte) (map[string][]byte, error) {
	gz, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer gz.Close()
	files := make(map[string][]byte)
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if hdr.Typeflag != tar.TypeReg {
			continue
		}
		content, err := io.ReadAll(io.LimitReader(tr, maxDownloadBytes))
		if err != nil {
			return nil, err
		}
		files[filepath.Base(hdr.Name)] = content
	}
	return files, nil
}

func extractAllFromZip(data []byte) (map[string][]byte, error) {
	r, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, err
	}
	files := make(map[string][]byte)
	for _, f := range r.File {
		if f.FileInfo().IsDir() {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return nil, err
		}
		content, err := io.ReadAll(io.LimitReader(rc, maxDownloadBytes))
		rc.Close()
		if err != nil {
			return nil, err
		}
		files[filepath.Base(f.Name)] = content
	}
	return files, nil
}

func extractAssets(data []byte, assetFileName string) (map[string][]byte, error) {
	if strings.HasSuffix(assetFileName, ".zip") {
		return extractAllFromZip(data)
	}
	return extractAllFromTarGz(data)
}
