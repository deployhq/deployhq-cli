package commands

import (
	"fmt"
	"strconv"
)

// identifiable is any resource that has both a numeric ID and a UUID identifier.
type identifiable interface {
	NumericID() int
	UUID() string
}

// resolveID checks if the given argument is a numeric ID and, if so, resolves
// it to the UUID identifier by scanning the provided list. This lets users and
// agents pass either form (e.g. "429" or "eec5201d-...") and have it work.
func resolveID[T identifiable](arg string, list []T) (string, error) {
	n, err := strconv.Atoi(arg)
	if err != nil {
		// Not numeric — assume it's already a UUID identifier.
		return arg, nil
	}
	for _, item := range list {
		if item.NumericID() == n {
			return item.UUID(), nil
		}
	}
	return "", fmt.Errorf("no resource found with numeric ID %d — use the UUID identifier instead", n)
}
