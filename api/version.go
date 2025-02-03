// Copyright 2023 Northern.tech AS
//
//	Licensed under the Apache License, Version 2.0 (the "License");
//	you may not use this file except in compliance with the License.
//	You may obtain a copy of the License at
//
//	    http://www.apache.org/licenses/LICENSE-2.0
//
//	Unless required by applicable law or agreed to in writing, software
//	distributed under the License is distributed on an "AS IS" BASIS,
//	WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//	See the License for the specific language governing permissions and
//	limitations under the License.
package api

import (
	"fmt"
	"runtime"
)

const (
	VersionUnknown = "unknown"
)

var (
	// Version information of current build
	Version string
)

func VersionString() string {
	if Version != "" {
		return Version
	}
	return VersionUnknown
}

func ShowVersion() string {
	return fmt.Sprintf("%s\truntime: %s",
		VersionString(), runtime.Version())
}

func UserAgent() string {
	return fmt.Sprintf("nt-connect/%s (%s; %s; %s)",
		VersionString(), runtime.Version(),
		runtime.GOOS, runtime.GOARCH)
}
