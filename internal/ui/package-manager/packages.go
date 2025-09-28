package packages

import (
	"fmt"
	"os"
)

// NOTE: These are ZephyrRTOS packages. Refactor if a need for other platforms arises

var packagesLinux = []string{
	"git",
	"cmake",
	"ninja-build",
	"gperf",
	"ccache",
	"dfu-util",
	"device-tree-compiler",
	"wget",
	"python3-dev",
	"python3-venv",
	"python3-tk",
	"xz-utils",
	"file",
	"make",
	"gcc",
	"gcc-multilib",
	"g++-multilib",
	"libsdl2-dev",
	"libmagic1",
}

var packagesLinuxARM = []string{
	"git",
	"cmake",
	"ninja-build",
	"gperf",
	"ccache",
	"dfu-util",
	"device-tree-compiler",
	"wget",
	"python3-dev",
	"python3-venv",
	"python3-tk",
	"xz-utils",
	"file",
	"make",
	"gcc",
	"libsdl2-dev",
	"libmagic1",
}

var packagesWindows = []string{
	"Kitware.CMake",
	"Ninja-build.Ninja",
	"oss-winget.gperf",
	"python",
	"Git.Git",
	"oss-winget.dtc",
	"wget",
	"7zip.7zip",
}

var packagesMacOS = []string{
	"cmake",
	"ninja",
	"gperf",
	"python3",
	"python-tk",
	"ccache",
	"qemu",
	"dtc",
	"libmagic",
	"wget",
	"openocd",
}

// func decidePackages() []string {
//TODO: return packages based on user OS
// }

// func getPackages() []string {
//	os := os.getOS() // or whatever it is in golang
// 	pkgs := packages
// 	copy(pkgs, packages)
//
// 	for k := range pkgs {
// 		pkgs[k] += fmt.Sprintf("-%d.%d.%d", 10, 10, 10)
// 	}
// 	return pkgs
// }
