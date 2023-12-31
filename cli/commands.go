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
package cli

import (
	"github.com/northerntechhq/nt-connect/app"
	"github.com/northerntechhq/nt-connect/config"
)

type runOptionsType struct {
	config         string
	fallbackConfig string
	debug          bool
	trace          bool
}

func initDaemon(config *config.NTConnectConfig) (*app.Daemon, error) {
	return app.NewDaemon(config)
}

func runDaemon(d *app.Daemon) error {
	// Handle user forcing update check.
	return d.Run()
}
