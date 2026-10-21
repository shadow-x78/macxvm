// MacXVM - Embedded Resources (MIT)
// https://github.com/shadow-x78/macxvm

// Package resources embeds the guest-side scripts and data used by MacXVM.
//
// The files in this directory are the single source of truth; no Go string
// constant duplicates their content.
package resources

import _ "embed"

//go:embed slim-level1.sh
var Slim1 string

//go:embed slim-level2.sh
var Slim2 string

//go:embed slim-labels.txt
var SlimLabels string

//go:embed install-clt.sh
var InstallCLT string
