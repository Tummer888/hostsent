package auth

import "strings"

type PermissionSet map[string]struct{}

func NewPermissionSet(items []string) PermissionSet {
	set := make(PermissionSet, len(items))
	for _, item := range items {
		set[strings.TrimSpace(item)] = struct{}{}
	}
	return set
}

func (s PermissionSet) Has(permission string) bool {
	_, ok := s[permission]
	return ok
}
