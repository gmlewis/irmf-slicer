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
	"net/http"
	"regexp"
	"strings"
)

// IRMF represents an IRMF shader.
type IRMF struct {
	Author    string          `json:"author"`
	Copyright string          `json:"copyright"`
	Date      string          `json:"date"`
	Encoding  *string         `json:"encoding,omitempty"`
	IRMF      string          `json:"irmf"`
	Materials []string        `json:"materials"`
	Max       []float32       `json:"max"`
	Min       []float32       `json:"min"`
	Notes     string          `json:"notes"`
	Options   json.RawMessage `json:"options"`
	Title     string          `json:"title"`
	Units     string          `json:"units"`
	Version   string          `json:"version"`

	Shader string `json:"-"`
}

var (
	jsonKeys = []string{
		"author",
		"copyright",
		"date",
		"irmf",
		"materials",
		"max",
		"min",
		"notes",
		"options",
		"title",
		"units",
		"version",
	}
	trailingCommaRE = regexp.MustCompile(`,[\s\n]*}`)
	arrayRE         = regexp.MustCompile(`\[([^\]]+)\]`)
	whitespaceRE    = regexp.MustCompile(`[\s\n]+`)
)

// newModel parses the IRMF source file and returns a new IRMF struct.
func newModel(src []byte) (*IRMF, error) {
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
		if _, err := io.Copy(buf, zr); err != nil {
			return err
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
		jsonBlob.Shader = string(shaderSrcBuf)
	}

	jsonBlob.Shader = processIncludes(jsonBlob.Shader)

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
	if i.IRMF != "1.0" {
		return findKeyLine(jsonBlobStr, "irmf"), fmt.Errorf("unsupported IRMF version: %v", i.IRMF)
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

	if len(i.Materials) <= 4 && strings.Index(shaderSrc, "mainModel4") < 0 {
		return findKeyLine(jsonBlobStr, "materials"), fmt.Errorf("Found %v materials, but missing 'mainModel4' function", len(i.Materials))
	}

	if len(i.Materials) > 4 && len(i.Materials) <= 9 && strings.Index(shaderSrc, "mainModel9") < 0 {
		return findKeyLine(jsonBlobStr, "materials"), fmt.Errorf("Found %v materials, but missing 'mainModel9' function", len(i.Materials))
	}

	if len(i.Materials) > 9 && len(i.Materials) <= 16 && strings.Index(shaderSrc, "mainModel16") < 0 {
		return findKeyLine(jsonBlobStr, "materials"), fmt.Errorf("Found %v materials, but missing 'mainModel16' function", len(i.Materials))
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

func (i *IRMF) format(shaderSrc string) (string, error) {
	buf, err := json.MarshalIndent(i, "", "  ")
	if err != nil {
		return "", fmt.Errorf("unable to format IRMF shader: %v", err)
	}

	jsonBlob := string(buf)

	// Clean up the JSON.
	jsonBlob = strings.Replace(jsonBlob, `"options": null,`, `"options": {},`, 1)
	jsonBlob = arrayRE.ReplaceAllStringFunc(jsonBlob, func(s string) string {
		return whitespaceRE.ReplaceAllString(s, "")
	})

	return fmt.Sprintf("/*%v*/\n%v", jsonBlob, shaderSrc), nil
}

func curl(url string) ([]byte, error) {
	resp, err := http.Get(url)
	if err != nil {
		log.Printf("Unable to download source from: %v", url)
		return nil, nil
	}
	defer resp.Body.Close()

	buf, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("Unable to read response body.")
		return nil, nil
	}
	log.Printf("Read %v bytes from %v", len(buf), url)

	return buf, nil
}

var (
	includeRE = regexp.MustCompile(`^#include\s+"([^"]+)"`)
)

const (
	githubRawPrefix = "https://raw.githubusercontent.com/"
	lygiaBaseURL    = "https://lygia.xyz"
	prefix1         = "lygia.xyz/"
	prefix2         = "lygia/"
	prefix3         = "github.com/"
)

func parseIncludeURL(trimmed string) string {
	m := includeRE.FindStringSubmatch(trimmed)
	if len(m) < 2 {
		return ""
	}

	inc := m[1]
	if !strings.HasSuffix(inc, ".glsl") {
		return ""
	}

	switch {
	case strings.HasPrefix(inc, prefix1):
		return fmt.Sprintf("%v/%v", lygiaBaseURL, inc[len(prefix1):])
	case strings.HasPrefix(inc, prefix2):
		return fmt.Sprintf("%v/%v", lygiaBaseURL, inc[len(prefix2):])
	case strings.HasPrefix(inc, prefix3):
		location := inc[len(prefix3):]
		location = strings.Replace(location, "/blob/", "/", 1)
		return githubRawPrefix + location
	default:
		return ""
	}
}

// processIncludes converts "#include" lines (with recognized prefixes)
// into their actual source by retrieving them from the internet.
// Note that multiline comments ("/*" and "*/") are currently not supported.
// It is recommended that an ignored "#include" statement should be commented-out
// with single-line comments ("//...").
func processIncludes(source string) string {
	lines := strings.Split(source, "\n")
	var result []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if url := parseIncludeURL(trimmed); url != "" {
			if buf, err := curl(url); err == nil {
				result = append(result, string(buf))
			}
			continue
		}

		result = append(result, line)
	}

	return strings.Join(result, "\n")
}
