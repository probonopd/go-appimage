package helpers

import "strings"

// ContainsString returns true if slice contains element that ends with the given string
func ContainsString(slice []string, element string) bool {

	for _, item := range slice {
		if strings.HasSuffix(item, element) == true {
			return true
		}
	}

	return false
}
