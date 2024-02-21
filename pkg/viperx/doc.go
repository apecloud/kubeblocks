/*
Copyright (C) 2022-2024 ApeCloud Co., Ltd

This file is part of KubeBlocks project

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program.  If not, see <http://www.gnu.org/licenses/>.
*/

// Package viperx is a thread-safe extension for the Viper configuration library.
//
// # Overview
//
// The viperx package provides a set of utilities and enhancements to the Viper library
// that ensure thread-safety when working with configuration settings in concurrent
// environments.
//
// # Features
//
//   - Thread-safe access: All operations on the configuration settings are designed to be
//     safe for concurrent access from multiple goroutines.
//
//   - Locking mechanism: The package utilizes a mutex lock to protect concurrent read
//     and write operations on the underlying Viper configuration.
//
// # Usage
//
// To use viperx, import the package and call viperx.XXX where supposed to call viper.XXX.
// Or, import the package and set the alias to viper, all viper calls exist should work well.
//
// Example:
//
//	import "github.com/apecloud/kubeblocks/pkg/viperx"
//
//	func main() {
//	    // Set a configuration value
//	    viperx.Set("key", "value")
//
//	    // Get a configuration value
//	    val := viperx.Get("key")
//	    fmt.Println(val)
//	}
//
// Note: While ViperX ensures thread-safety for the configuration settings, it does not
// modify the underlying behavior of Viper itself. It is still important to follow the
// guidelines and best practices provided by Viper for configuration management.
//
// For more information on Viper, refer to the official Viper documentation at:
// https://github.com/spf13/viper
package viperx
