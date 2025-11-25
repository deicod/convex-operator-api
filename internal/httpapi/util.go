package httpapi

func namespaceAllowed(allowed []string, ns string) bool {
	if len(allowed) == 0 {
		return true
	}
	if ns == "" {
		return false
	}
	for _, a := range allowed {
		if ns == a {
			return true
		}
	}
	return false
}
