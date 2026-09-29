package main

import (
	"slices"
	"strings"

	"github.com/WilsonSousajr/omatty/internal/golist"
)

// check applies ADR 0001's layer table to every production import of pkgs.
func check(module string, pkgs []golist.Package) []Finding {
	var out []Finding
	for _, p := range pkgs {
		out = append(out, checkPackage(module, p)...)
	}
	return out
}

func checkPackage(module string, p golist.Package) []Finding {
	from := short(p.ImportPath, module)
	layer := layerOf(p.ImportPath, module)
	if layer == Unlayered {
		return []Finding{{From: from, Rule: "unlayered"}}
	}
	var out []Finding
	for _, imp := range p.Imports {
		if f, bad := checkImport(module, layer, imp); bad {
			f.From = from
			out = append(out, f)
		}
	}
	return out
}

// checkImport says whether a package in layer may import imp.
func checkImport(module string, layer Layer, imp string) (Finding, bool) {
	if strings.HasPrefix(imp, module+"/") {
		to := layerOf(imp, module)
		rule := string(layer) + " -> " + string(to)
		return Finding{To: short(imp, module), Rule: rule}, slices.Contains(mayNotReach[layer], to)
	}
	rule := string(layer) + " may not import " + imp
	if layer == Domain && !stdlib(imp) {
		return Finding{To: imp, Rule: rule}, true
	}
	for _, banned := range mayNotImport[layer] {
		if imp == banned || (strings.HasSuffix(banned, "/") && strings.HasPrefix(imp, banned)) {
			return Finding{To: imp, Rule: rule}, true
		}
	}
	return Finding{}, false
}

// stdlib reports whether imp is a standard-library path: its first element has
// no dot, which every module path's host does.
func stdlib(imp string) bool {
	first, _, _ := strings.Cut(imp, "/")
	return !strings.Contains(first, ".")
}

func short(importPath, module string) string {
	return strings.TrimPrefix(importPath, module+"/")
}
