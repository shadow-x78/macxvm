// MacXVM - Embedded Resources (MIT)
// https://github.com/shadow-x78/macxvm

package resources

import _ "embed"

//go:embed slim-level1.sh
var Slim1 string

//go:embed slim-level2.sh
var Slim2 string

//go:embed slim-labels.txt
var SlimLabels string

//go:embed apps-default.txt
var AppsDefault string

//go:embed install-clt.sh
var InstallCLT string
