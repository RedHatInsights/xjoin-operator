package utils

/*
 * Returns a copy of the given map with the given keys left out
 */
func Omit(value map[string]string, ignoredKeys ...string) map[string]string {
	copy := make(map[string]string, len(value))

	for k, v := range value {
		ignored := false

		for _, ignoredKey := range ignoredKeys {
			if k == ignoredKey {
				ignored = true
			}
		}

		if !ignored {
			copy[k] = v
		}

	}

	return copy
}

func ContainsString(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}
