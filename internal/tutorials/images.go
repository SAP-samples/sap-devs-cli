package tutorials

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
