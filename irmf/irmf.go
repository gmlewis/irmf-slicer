// Package irmf parses and validates IRMF shader files.
package irmf

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	neturl "net/url"
	"regexp"
	"strings"
	"time"
	"unicode"
)

const (
	// Limits for untrusted input. Remote includes are fetched code, so cap the
	// number and size of downloads and the amount of work a model can request.
	maxIRMFFileBytes      = 64 << 20 // 64 MiB raw .irmf file
	maxShaderBodyBytes    = 16 << 20 // 16 MiB decoded shader body
	maxIncludeBytes       = 1 << 20  // 1 MiB per remote #include
	maxIncludes           = 16       // remote #include directives per model
	maxSlicePixels        = 64 << 20 // samples in one rendered slice
	maxSlices             = 1 << 20  // slices along one axis
	maxModelExtent        = 1 << 20  // model units along one axis
	includeRequestTimeout = 15 * time.Second
)

// modelOptions controls parsing of untrusted model files.
type modelOptions struct {
	// AllowRemoteIncludes permits fetching #include URLs from lygia.xyz and
	// raw.githubusercontent.com. It is off by default: remote includes are
	// fetched code and can change a model without changing the file.
	AllowRemoteIncludes bool
	// AllowMutableIncludes permits GitHub includes that name a branch or tag
	// instead of a 40-hex commit. This weakens reproducibility: the code can
	// change after the model was reviewed. It is off by default and only has
	// an effect when AllowRemoteIncludes is also set.
	AllowMutableIncludes bool
}

// IRMF represents an IRMF shader.
type IRMF struct {
	Author      string          `json:"author,omitempty"`
	License     string          `json:"license,omitempty"`
	Date        string          `json:"date,omitempty"`
	Encoding    *string         `json:"encoding,omitempty"`
	IRMFVersion string          `json:"irmf"`
	GLSLVersion string          `json:"glslVersion,omitempty"`
	Language    string          `json:"language"`
	Materials   []string        `json:"materials"`
	Max         []float32       `json:"max"`
	Min         []float32       `json:"min"`
	Notes       string          `json:"notes,omitempty"`
	Options     json.RawMessage `json:"options,omitempty"`
	Title       string          `json:"title,omitempty"`
	TSLVersion  string          `json:"tslVersion,omitempty"`
	Units       string          `json:"units"`
	Version     string          `json:"version,omitempty"`

	Shader string `json:"-"`
}

var (
	jsonKeys = []string{
		"author",
		"license",
		"date",
		"encoding",
		"irmf",
		"glslVersion",
		"language",
		"materials",
		"max",
		"min",
		"notes",
		"options",
		"title",
		"tslVersion",
		"units",
		"version",
	}
	trailingCommaRE = regexp.MustCompile(`,[\s\n]*}`)
)

// newModel parses the IRMF source file with remote includes disabled.
func newModel(src []byte) (*IRMF, error) {
	return newModelWithOptions(src, modelOptions{})
}

// newModelWithOptions parses the IRMF source file and returns a new IRMF struct.
func newModelWithOptions(src []byte, opts modelOptions) (*IRMF, error) {
	if len(src) > maxIRMFFileBytes {
		return nil, fmt.Errorf("IRMF file is too large (%d bytes; limit is %d)", len(src), maxIRMFFileBytes)
	}
	if bytes.Index(src, []byte("/*{")) != 0 {
		return nil, errors.New(`Unable to find leading "/*{"`)
	}
	endJSON := bytes.Index(src, []byte("\n}*/\n"))
	if endJSON < 0 {
		return nil, errors.New(`Unable to find trailing "}*/"`)
	}

	jsonBlobStr := string(src[2 : endJSON+2])
	jsonBlob, err := parseJSON(jsonBlobStr)
	if err != nil {
		return nil, fmt.Errorf("unable to parse JSON blob: %v", err)
	}

	shaderSrcBuf := src[endJSON+5:]
	unzip := func(data []byte) error {
		zr, err := gzip.NewReader(bytes.NewReader(data))
		if err != nil {
			return err
		}
		buf := &bytes.Buffer{}
		// Cap decompression so a small file cannot expand into a memory bomb.
		if _, err := io.Copy(buf, io.LimitReader(zr, maxShaderBodyBytes+1)); err != nil {
			return err
		}
		if buf.Len() > maxShaderBodyBytes {
			return fmt.Errorf("decoded shader exceeds %d bytes", maxShaderBodyBytes)
		}
		if err := zr.Close(); err != nil {
			return err
		}
		jsonBlob.Shader = buf.String()
		return nil
	}

	if jsonBlob.Encoding != nil && *jsonBlob.Encoding == "gzip+base64" {
		data, err := base64.RawStdEncoding.DecodeString(string(shaderSrcBuf))
		if err != nil {
			return nil, fmt.Errorf("uudecode error: %v", err)
		}
		if err := unzip(data); err != nil {
			return nil, fmt.Errorf("unzip: %v", err)
		}
		jsonBlob.Encoding = nil
	} else if jsonBlob.Encoding != nil && *jsonBlob.Encoding == "gzip" {
		if err := unzip(shaderSrcBuf); err != nil {
			return nil, fmt.Errorf("unzip: %v", err)
		}
		jsonBlob.Encoding = nil
	} else {
		if len(shaderSrcBuf) > maxShaderBodyBytes {
			return nil, fmt.Errorf("shader body is too large (%d bytes; limit is %d)", len(shaderSrcBuf), maxShaderBodyBytes)
		}
		jsonBlob.Shader = string(shaderSrcBuf)
	}

	if hasIncludeDirective(jsonBlob.Shader) {
		switch {
		case jsonBlob.Language == "tsl":
			// TSL is JavaScript: expanding an include would fetch remote text
			// straight into executable code.
			return nil, errors.New("TSL bodies must not contain #include; inline the included code")
		case opts.AllowRemoteIncludes:
			shader, err := processIncludes(jsonBlob.Shader, opts.AllowMutableIncludes)
			if err != nil {
				return nil, err
			}
			jsonBlob.Shader = shader
		default:
			return nil, errors.New("shader contains #include but remote includes are disabled; " +
				"pass --remote-includes to fetch them, or inline the included code")
		}
	}

	if lineNum, err := jsonBlob.validate(jsonBlobStr, jsonBlob.Shader); err != nil {
		return nil, fmt.Errorf("invalid JSON blob on line %v: %v", lineNum, err)
	}

	return jsonBlob, nil
}

func parseJSON(s string) (*IRMF, error) {
	result := &IRMF{}

	// Avoid the trailing comma silliness in JavaScript:
	s = trailingCommaRE.ReplaceAllString(s, "}")

	if err := json.Unmarshal([]byte(s), result); err != nil {
		// This allows the user to omit quotes around keys:
		for _, key := range jsonKeys {
			s = strings.Replace(s, key+":", fmt.Sprintf("%q:", key), 1)
		}
		if err := json.Unmarshal([]byte(s), result); err != nil {
			return nil, err
		}
	}
	return result, nil
}

func (i *IRMF) validate(jsonBlobStr, shaderSrc string) (int, error) {
	if i.IRMFVersion != "1.0" {
		return findKeyLine(jsonBlobStr, "irmf"), fmt.Errorf("unsupported IRMF version: %v", i.IRMFVersion)
	}
	if len(i.Materials) < 1 {
		return findKeyLine(jsonBlobStr, "materials"), errors.New("must list at least one material name")
	}
	if len(i.Materials) > 16 {
		return findKeyLine(jsonBlobStr, "materials"), fmt.Errorf("IRMF 1.0 only supports up to 16 materials, found %v", len(i.Materials))
	}
	if len(i.Max) != 3 {
		return findKeyLine(jsonBlobStr, "max"), fmt.Errorf("max must have only 3 values, found %v", len(i.Max))
	}
	if len(i.Min) != 3 {
		return findKeyLine(jsonBlobStr, "min"), fmt.Errorf("min must have only 3 values, found %v", len(i.Min))
	}
	if i.Units == "" {
		return findKeyLine(jsonBlobStr, "units"), errors.New("units are required by IRMF 1.0 (even though the irmf-editor ignores the units)")
	}
	if i.Min[0] >= i.Max[0] {
		return findKeyLine(jsonBlobStr, "max"), fmt.Errorf("min.x (%v) must be strictly less than max.x (%v)", i.Min[0], i.Max[0])
	}
	if i.Min[1] >= i.Max[1] {
		return findKeyLine(jsonBlobStr, "max"), fmt.Errorf("min.y (%v) must be strictly less than max.y (%v)", i.Min[1], i.Max[1])
	}
	if i.Min[2] >= i.Max[2] {
		return findKeyLine(jsonBlobStr, "max"), fmt.Errorf("min.z (%v) must be strictly less than max.z (%v)", i.Min[2], i.Max[2])
	}
	if line, err := validateBounds(jsonBlobStr, i.Min, i.Max); err != nil {
		return line, err
	}

	switch i.Language {
	case "glsl", "":
		if len(i.Materials) <= 4 && !strings.Contains(shaderSrc, "mainModel4") {
			return findKeyLine(jsonBlobStr, "materials"), fmt.Errorf("Found %v materials, but missing 'mainModel4' function", len(i.Materials))
		}

		if len(i.Materials) > 4 && len(i.Materials) <= 9 && !strings.Contains(shaderSrc, "mainModel9") {
			return findKeyLine(jsonBlobStr, "materials"), fmt.Errorf("Found %v materials, but missing 'mainModel9' function", len(i.Materials))
		}

		if len(i.Materials) > 9 && len(i.Materials) <= 16 && !strings.Contains(shaderSrc, "mainModel16") {
			return findKeyLine(jsonBlobStr, "materials"), fmt.Errorf("Found %v materials, but missing 'mainModel16' function", len(i.Materials))
		}
	case "tsl":
		// TSL is JavaScript: expanding #include would fetch remote text into
		// executable code, so the format forbids it.
		if hasIncludeDirective(shaderSrc) {
			return findKeyLine(jsonBlobStr, "#include"), errors.New("TSL bodies must not contain #include; inline the included code")
		}
		// TSL entry points are JavaScript callables named mainModelN.
		if len(i.Materials) <= 4 && !strings.Contains(shaderSrc, "mainModel4") {
			return findKeyLine(jsonBlobStr, "materials"), fmt.Errorf("Found %v materials, but missing 'mainModel4' function", len(i.Materials))
		}
		if len(i.Materials) > 4 && len(i.Materials) <= 9 && !strings.Contains(shaderSrc, "mainModel9") {
			return findKeyLine(jsonBlobStr, "materials"), fmt.Errorf("Found %v materials, but missing 'mainModel9' function", len(i.Materials))
		}
		if len(i.Materials) > 9 && len(i.Materials) <= 16 && !strings.Contains(shaderSrc, "mainModel16") {
			return findKeyLine(jsonBlobStr, "materials"), fmt.Errorf("Found %v materials, but missing 'mainModel16' function", len(i.Materials))
		}
	case "wgsl":
		if len(i.Materials) <= 4 && !strings.Contains(shaderSrc, "fn mainModel4") {
			return findKeyLine(jsonBlobStr, "materials"), fmt.Errorf("Found %v materials, but missing 'fn mainModel4' function", len(i.Materials))
		}

		if len(i.Materials) > 4 && len(i.Materials) <= 9 && !strings.Contains(shaderSrc, "fn mainModel9") {
			return findKeyLine(jsonBlobStr, "materials"), fmt.Errorf("Found %v materials, but missing 'fn mainModel9' function", len(i.Materials))
		}

		if len(i.Materials) > 9 && len(i.Materials) <= 16 && !strings.Contains(shaderSrc, "fn mainModel16") {
			return findKeyLine(jsonBlobStr, "materials"), fmt.Errorf("Found %v materials, but missing 'fn mainModel16' function", len(i.Materials))
		}
	default:
		return findKeyLine(jsonBlobStr, "language"), fmt.Errorf("unsupported language: %v", i.Language)
	}

	if i.Encoding != nil && *i.Encoding != "" && *i.Encoding != "gzip" && *i.Encoding != "gzip+base64" {
		return findKeyLine(jsonBlobStr, "encoding"), errors.New("Unsupported encoding. Possible values are 'gzip' or 'gzip+base64'")
	}

	return 0, nil
}

func findKeyLine(s, key string) int {
	if i := strings.Index(s, fmt.Sprintf("%q:", key)); i >= 0 {
		return indexToLineNum(s, i)
	}
	if i := strings.Index(s, fmt.Sprintf("%v:", key)); i >= 0 {
		return indexToLineNum(s, i)
	}
	if i := strings.Index(s, key); i >= 0 {
		return indexToLineNum(s, i)
	}
	return 2 // Fall back to top of json blob.
}

func indexToLineNum(s string, offset int) int {
	s = s[:offset]
	return strings.Count(s, "\n") + 1
}

// allowedIncludeHosts is the set of hosts an #include may be fetched from.
// Redirects are checked against it as well, so a trusted host cannot bounce a
// request to an arbitrary one.
var allowedIncludeHosts = map[string]bool{
	"lygia.xyz":                 true,
	"raw.githubusercontent.com": true,
}

var includeClient = &http.Client{
	Timeout: includeRequestTimeout,
	CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= 5 {
			return errors.New("too many redirects")
		}
		if req.URL.Scheme != "https" || !allowedIncludeHosts[req.URL.Hostname()] {
			return fmt.Errorf("redirect to disallowed include host %q", req.URL.Host)
		}
		return nil
	},
}

func fetchInclude(rawURL string) ([]byte, error) {
	u, err := neturl.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("invalid include URL %q: %v", rawURL, err)
	}
	if u.Scheme != "https" || !allowedIncludeHosts[u.Hostname()] || u.User != nil {
		return nil, fmt.Errorf("include URL is not from an allowed https host: %q", rawURL)
	}

	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "irmf-slicer/1.0 (+https://github.com/gmlewis/irmf-slicer)")

	resp, err := includeClient.Do(req)
	if err != nil {
		log.Printf("Unable to download source from: %q", rawURL)
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unable to download %q: HTTP %v", rawURL, resp.StatusCode)
	}

	// Cap the response so a hostile host cannot exhaust memory.
	buf, err := io.ReadAll(io.LimitReader(resp.Body, maxIncludeBytes+1))
	if err != nil {
		return nil, err
	}
	if len(buf) > maxIncludeBytes {
		return nil, fmt.Errorf("include %v exceeds %d bytes", rawURL, maxIncludeBytes)
	}
	log.Printf("Read %v bytes from %q", len(buf), rawURL)
	return buf, nil
}

var (
	includeRE = regexp.MustCompile(`^#include\s+"([^"]+)"`)
	// GitHub includes must name an immutable commit (40 hex characters).
	// Branch and tag names can be moved by the repository owner, which would
	// change the code a model executes without changing the model file.
	githubIncludeRE = regexp.MustCompile(
		`^github\.com/([^/]+)/([^/]+)/(?:(?:blob|raw)/)?([0-9a-fA-F]{40})/(.+)$`)
	// Only used when the caller explicitly allows mutable includes.
	githubMutableIncludeRE = regexp.MustCompile(
		`^github\.com/([^/]+)/([^/]+)/(?:blob|raw)/([^/]+)/(.+)$`)
)

const (
	githubRawPrefix = "https://raw.githubusercontent.com/"
	lygiaBaseURL    = "https://lygia.xyz"
	prefix1         = "lygia.xyz/"
	prefix2         = "lygia/"
	prefix3         = "github.com/"
)

// SafeMaterialName converts an untrusted material name into a safe filename
// component. Material names come from the IRMF header, so they must never
// contain path separators, control characters, characters that are invalid on
// common filesystems, or unbounded length.
//
// Unicode is preserved so that material names in any language remain usable.
func SafeMaterialName(name string) string {
	var b strings.Builder
	lastDash := false
	for _, r := range name {
		// Keep printable Unicode (letters, digits, combining marks, CJK,
		// punctuation) but never path separators or control characters.
		safe := !unicode.IsControl(r) && !unicode.IsSpace(r) &&
			!strings.ContainsRune(`/\:*?"<>|`, r) && r != 0
		if safe {
			b.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash {
			b.WriteByte('-')
			lastDash = true
		}
	}
	s := strings.Trim(b.String(), ".-_")
	if s == "" {
		s = "material"
	}
	// Cap by runes (so a multi-byte character is never split) and by bytes
	// (so the filename stays within common filesystem limits).
	runes := []rune(s)
	if len(runes) > 64 {
		s = string(runes[:64])
	}
	for len(s) > 200 {
		runes = []rune(s)
		s = string(runes[:len(runes)-1])
	}
	return s
}

func hasIncludeDirective(src string) bool {
	for _, line := range strings.Split(src, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "#include") {
			return true
		}
	}
	return false
}

// parseIncludeURL resolves an #include line, requiring a pinned commit for
// GitHub URLs.
func parseIncludeURL(trimmed string) string {
	return parseIncludeURLWithOptions(trimmed, false)
}

// parseIncludeURLWithOptions resolves an #include line. When allowMutable is
// true, GitHub blob/raw URLs that name a branch or tag are also accepted. That
// is less reproducible and must only be enabled explicitly by the operator.
func parseIncludeURLWithOptions(trimmed string, allowMutable bool) string {
	m := includeRE.FindStringSubmatch(trimmed)
	if len(m) < 2 {
		return ""
	}

	inc := m[1]
	if !strings.HasSuffix(inc, ".glsl") && !strings.HasSuffix(inc, ".wgsl") {
		return ""
	}

	switch {
	case strings.HasPrefix(inc, prefix1):
		return fmt.Sprintf("%v/%v", lygiaBaseURL, inc[len(prefix1):])
	case strings.HasPrefix(inc, prefix2):
		return fmt.Sprintf("%v/%v", lygiaBaseURL, inc[len(prefix2):])
	case strings.HasPrefix(inc, prefix3):
		if m := githubIncludeRE.FindStringSubmatch(inc); m != nil {
			if !safeIncludeRefAndPath(m[3], m[4]) {
				return ""
			}
			return fmt.Sprintf("%v%v/%v/%v/%v", githubRawPrefix, m[1], m[2], m[3], m[4])
		}
		if !allowMutable {
			return ""
		}
		m := githubMutableIncludeRE.FindStringSubmatch(inc)
		if m == nil || !safeIncludeRefAndPath(m[3], m[4]) {
			return ""
		}
		return fmt.Sprintf("%v%v/%v/%v/%v", githubRawPrefix, m[1], m[2], m[3], m[4])
	default:
		return ""
	}
}

// safeIncludeRefAndPath rejects branch names and paths that would change the
// meaning of the constructed URL or traverse the fetched path.
func safeIncludeRefAndPath(ref, path string) bool {
	if ref == "" || ref == "." || ref == ".." ||
		strings.Contains(ref, "..") || strings.ContainsAny(ref, `\?#%`) {
		return false
	}
	if strings.ContainsAny(path, `\?#%`) || strings.ContainsAny(path, "\x00\r\n") {
		return false
	}
	for _, part := range strings.Split(path, "/") {
		if part == "." || part == ".." {
			return false
		}
	}
	return true
}

// processIncludes converts "#include" lines (with recognized prefixes)
// into their actual source by retrieving them from the internet.
//
// Callers must opt in (see modelOptions.AllowRemoteIncludes); TSL bodies never
// reach this function. Failures are returned rather than silently dropping the
// include, so a model can never be rendered with a partially applied shader.
//
// Note that multiline comments ("/*" and "*/") are currently not supported.
// It is recommended that an ignored "#include" statement should be commented-out
// with single-line comments ("//...").
func processIncludes(source string, allowMutable bool) (string, error) {
	lines := strings.Split(source, "\n")
	var result []string
	includeCount := 0
	var totalBytes int64
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		url := parseIncludeURLWithOptions(trimmed, allowMutable)
		if url == "" {
			if strings.HasPrefix(trimmed, "#include") {
				if allowMutable {
					return "", fmt.Errorf("unsupported #include: %v", trimmed)
				}
				return "", fmt.Errorf("unsupported or unpinned #include: %v (pass --allow-mutable-includes to accept a branch or tag)", trimmed)
			}
			result = append(result, line)
			continue
		}
		if allowMutable && parseIncludeURL(trimmed) == "" {
			log.Printf("WARNING: fetching mutable include: %q", url)
		}

		if includeCount >= maxIncludes {
			return "", fmt.Errorf("too many remote includes (limit is %d)", maxIncludes)
		}
		buf, err := fetchInclude(url)
		if err != nil {
			return "", fmt.Errorf("include %q: %v", url, err)
		}
		totalBytes += int64(len(buf))
		if totalBytes > maxIncludes*maxIncludeBytes {
			return "", errors.New("total included source is too large")
		}
		includeCount++
		result = append(result, string(buf))
	}

	return strings.Join(result, "\n"), nil
}

// validateBounds rejects non-finite and absurdly large model bounds. Bounds
// feed directly into slice counts, so a hostile model could otherwise ask the
// slicer for an unbounded amount of work.
func validateBounds(jsonBlobStr string, min, max []float32) (int, error) {
	for axis := 0; axis < 3; axis++ {
		for _, v := range []float32{min[axis], max[axis]} {
			if math.IsNaN(float64(v)) || math.IsInf(float64(v), 0) {
				return findKeyLine(jsonBlobStr, "min"), fmt.Errorf("model bounds must be finite (found %v)", v)
			}
		}
		if extent := float64(max[axis]) - float64(min[axis]); extent > maxModelExtent {
			return findKeyLine(jsonBlobStr, "max"), fmt.Errorf(
				"model extent on axis %d is %v (limit is %v)", axis, extent, maxModelExtent)
		}
	}
	return 0, nil
}
