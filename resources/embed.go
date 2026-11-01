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

//go:embed args-template.txt
var ArgsTemplate string

//go:embed cpu-models.txt
var CPUModels string

//go:embed boot-assets.txt
var BootAssets string

//go:embed args-template-vfio-install.txt
var ArgsTemplateVFIOInstall string

//go:embed args-template-vfio-daily.txt
var ArgsTemplateVFIODaily string

//go:embed args-template-vfio-recovery.txt
var ArgsTemplateVFIORecovery string

//go:embed args-template-reims-install.txt
var ArgsTemplateReimsInstall string

//go:embed args-template-reims-daily.txt
var ArgsTemplateReimsDaily string

//go:embed args-template-reims-recovery.txt
var ArgsTemplateReimsRecovery string

//go:embed opencore-mxgpu/config-install.plist
var OpenCoreMXGPUInstall string

//go:embed opencore-mxgpu/config-daily.plist
var OpenCoreMXGPUDaily string

//go:embed opencore-mxgpu/config-recovery.plist
var OpenCoreMXGPURecovery string

//go:embed opencore-reims/config-install.plist
var OpenCoreReimsInstall string

//go:embed opencore-reims/config-daily.plist
var OpenCoreReimsDaily string

//go:embed opencore-reims/config-recovery.plist
var OpenCoreReimsRecovery string

//go:embed mxgpu-release-public-key.txt
var MXGPUReleasePublicKey string
