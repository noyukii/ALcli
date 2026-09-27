package api

import "strings"

// DocumentKind returns the leading GraphQL operation kind used by the CLI and
// MCP command surfaces. AniList still validates the full document.
func DocumentKind(document string) string {
	for {
		document = strings.TrimSpace(document)
		if !strings.HasPrefix(document, "#") {
			break
		}
		if i := strings.IndexByte(document, '\n'); i >= 0 {
			document = document[i+1:]
		} else {
			return ""
		}
	}
	if strings.HasPrefix(document, "mutation") && (len(document) == 8 || !isNameByte(document[8])) {
		return "mutation"
	}
	if strings.HasPrefix(document, "query") && (len(document) == 5 || !isNameByte(document[5])) || strings.HasPrefix(document, "{") {
		return "query"
	}
	return ""
}

func isNameByte(b byte) bool {
	return b == '_' || b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z' || b >= '0' && b <= '9'
}
