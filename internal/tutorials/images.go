package tutorials

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const maxImageBytes = 10 << 20 // 10 MB

// ImageRef represents a parsed image reference from tutorial markdown.
type ImageRef struct {
	Alt          string `json:"alt"`
	OriginalPath string `json:"original_path"`
	URL          string `json:"url"`
}

// ExtractImageRefs finds all markdown image references and resolves relative
// paths to full GitHub raw URLs. Absolute URLs and path traversals are left as-is.
func ExtractImageRefs(content, repo, branch, slug string) []ImageRef {
	matches := imageRE.FindAllStringSubmatch(content, -1)
	refs := make([]ImageRef, 0, len(matches))
	for _, m := range matches {
		alt, path := m[1], m[2]
		ref := ImageRef{Alt: alt, OriginalPath: path}
		ref.URL = resolveImagePath(path, repo, branch, slug)
		refs = append(refs, ref)
	}
	return refs
}

// FetchedImage holds base64-encoded image data ready for MCP ImageContent.
type FetchedImage struct {
	Alt      string `json:"alt"`
	URL      string `json:"url"`
	Data     string `json:"data"`
	MIMEType string `json:"mime_type"`
}

var imageHTTPClient = &http.Client{Timeout: 15 * time.Second}

// FetchImage downloads an image from url, caches it locally, and returns
// the base64-encoded data with MIME type. Returns cached data on subsequent calls.
func FetchImage(url, cacheDir, slug string) (*FetchedImage, error) {
	filename := filepath.Base(url)
	if filename == "" || filename == "." || filename == "/" {
		return nil, fmt.Errorf("cannot determine filename from URL: %s", url)
	}

	// Hash-prefix the filename to avoid collisions when different URLs share the same basename.
	h := sha256.Sum256([]byte(url))
	cacheFilename := hex.EncodeToString(h[:8]) + "_" + filename

	dir := filepath.Join(cacheDir, "tutorials", "images", slug)
	cached := filepath.Join(dir, cacheFilename)

	if data, err := os.ReadFile(cached); err == nil {
		mimeType := mimeFromExt(filename)
		return &FetchedImage{
			URL:      url,
			Data:     base64.StdEncoding.EncodeToString(data),
			MIMEType: mimeType,
		}, nil
	}

	resp, err := imageHTTPClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("fetch image %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch image %s: HTTP %d", url, resp.StatusCode)
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, maxImageBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read image %s: %w", url, err)
	}
	if len(data) > maxImageBytes {
		return nil, fmt.Errorf("image %s exceeds size limit (%d bytes)", url, maxImageBytes)
	}

	mimeType := resp.Header.Get("Content-Type")
	if mimeType == "" || strings.HasPrefix(mimeType, "application/octet-stream") {
		mimeType = mimeFromExt(filename)
	}
	if mt, _, err := mime.ParseMediaType(mimeType); err == nil {
		mimeType = mt
	}

	if err := os.MkdirAll(dir, 0755); err == nil {
		_ = os.WriteFile(cached, data, 0644)
	}

	return &FetchedImage{
		URL:      url,
		Data:     base64.StdEncoding.EncodeToString(data),
		MIMEType: mimeType,
	}, nil
}

// FetchStepImages fetches all images from the given refs, skipping any that fail.
func FetchStepImages(refs []ImageRef, cacheDir, slug string) []FetchedImage {
	var images []FetchedImage
	for _, ref := range refs {
		img, err := FetchImage(ref.URL, cacheDir, slug)
		if err != nil {
			continue
		}
		img.Alt = ref.Alt
		images = append(images, *img)
	}
	return images
}

func mimeFromExt(filename string) string {
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	case ".svg":
		return "image/svg+xml"
	default:
		return "image/png"
	}
}
