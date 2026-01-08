package sync

import (
	"sort"
	"strings"
)

func mergeStringList(a, b []string) []string {
	return uniqueSortedStrings(a, b)
}

func mergeUpload(existing []artifactKey, add map[artifactKey]struct{}) []artifactKey {
	set := make(map[artifactKey]struct{}, len(existing)+len(add))
	for _, k := range existing {
		if k.isZero() {
			continue
		}
		set[k] = struct{}{}
	}
	for k := range add {
		if k.isZero() {
			continue
		}
		set[k] = struct{}{}
	}
	out := make([]artifactKey, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Kind == out[j].Kind {
			return out[i].ID < out[j].ID
		}
		return out[i].Kind < out[j].Kind
	})
	return out
}

func removeUpload(list []artifactKey, k artifactKey) []artifactKey {
	out := list[:0]
	for _, it := range list {
		if it.Kind == k.Kind && it.ID == k.ID {
			continue
		}
		out = append(out, it)
	}
	return out
}

func mergeObjects(existing []SyncObject, add []SyncObject) []SyncObject {
	set := make(map[string]SyncObject, len(existing)+len(add))
	for _, o := range existing {
		if o.isZero() {
			continue
		}
		set[o.Kind+"|"+o.ID] = SyncObject{Kind: o.Kind, ID: o.ID}
	}
	for _, o := range add {
		if o.isZero() {
			continue
		}
		set[o.Kind+"|"+o.ID] = SyncObject{Kind: o.Kind, ID: o.ID}
	}
	out := make([]SyncObject, 0, len(set))
	for _, v := range set {
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Kind == out[j].Kind {
			return out[i].ID < out[j].ID
		}
		return out[i].Kind < out[j].Kind
	})
	return out
}

func mergeDeployRemaining(existing []deployTarget, add []deployTarget) []deployTarget {
	set := make(map[string]deployTarget, len(existing)+len(add))
	for _, d := range existing {
		if d.isZero() {
			continue
		}
		set[d.Kind+"|"+d.ID] = deployTarget{Kind: d.Kind, ID: d.ID}
	}
	for _, d := range add {
		if d.isZero() {
			continue
		}
		set[d.Kind+"|"+d.ID] = deployTarget{Kind: d.Kind, ID: d.ID}
	}
	out := make([]deployTarget, 0, len(set))
	for _, v := range set {
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Kind == out[j].Kind {
			return out[i].ID < out[j].ID
		}
		return out[i].Kind < out[j].Kind
	})
	return out
}

func removeDeployTarget(list []deployTarget, d deployTarget) []deployTarget {
	out := list[:0]
	for _, it := range list {
		if it.Kind == d.Kind && it.ID == d.ID {
			continue
		}
		out = append(out, it)
	}
	return out
}

// normalizePathSlash ensures we can compare paths regardless of OS separators.
func normalizePathSlash(p string) string {
	return strings.TrimSpace(strings.ReplaceAll(p, "\\", "/"))
}
