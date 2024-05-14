// Copyright 2024 Blues Inc.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

package main

import (
	"fmt"

	"github.com/blues/note-cli/lib"
	"github.com/blues/note-go/note"
	notegoapi "github.com/blues/note-go/notehub/api"
)

type Vars map[string]string

// Load env vars into metadata from a list of devices
func varsGetFromDevices(appMetadata AppMetadata, uids []string, flagVerbose bool) (vars map[string]Vars, err error) {

	vars = map[string]Vars{}

	for _, deviceUID := range uids {
		varsRsp := notegoapi.GetDeviceEnvironmentVariablesResponse{}
		url := fmt.Sprintf("/v1/projects/%s/devices/%s/environment_variables", appMetadata.App.UID, deviceUID)
		err = reqHubV1(flagVerbose, lib.ConfigAPIHub(), "GET", url, nil, &varsRsp)
		if err != nil {
			return
		}
		vars[deviceUID] = varsRsp.EnvironmentVariables
	}

	return

}

// Load env vars into metadata from a list of fleets
func varsGetFromFleets(appMetadata AppMetadata, uids []string, flagVerbose bool) (vars map[string]Vars, err error) {

	vars = map[string]Vars{}

	for _, fleetUID := range uids {
		varsRsp := notegoapi.GetFleetEnvironmentVariablesResponse{}
		url := fmt.Sprintf("/v1/projects/%s/fleets/%s/environment_variables", appMetadata.App.UID, fleetUID)
		err = reqHubV1(flagVerbose, lib.ConfigAPIHub(), "GET", url, nil, &varsRsp)
		if err != nil {
			return
		}
		vars[fleetUID] = varsRsp.EnvironmentVariables
	}

	return
}

// Load env vars into metadata from a list of devices and set their values
func varsSetFromDevices(appMetadata AppMetadata, uids []string, template Vars, flagVerbose bool) (vars map[string]Vars, err error) {

	vars = map[string]Vars{}

	for _, deviceUID := range uids {

		rspGet := notegoapi.GetDeviceEnvironmentVariablesResponse{}
		url := fmt.Sprintf("/v1/projects/%s/devices/%s/environment_variables", appMetadata.App.UID, deviceUID)
		err = reqHubV1(flagVerbose, lib.ConfigAPIHub(), "GET", url, nil, &rspGet)
		if err != nil {
			return
		}

		req := notegoapi.PutDeviceEnvironmentVariablesRequest{}
		req.EnvironmentVariables = rspGet.EnvironmentVariables
		for k, v := range template {
			req.EnvironmentVariables[k] = v
		}

		var reqJSON []byte
		reqJSON, err = note.JSONMarshal(req)
		if err != nil {
			return
		}

		rspPut := notegoapi.PutDeviceEnvironmentVariablesResponse{}
		err = reqHubV1(flagVerbose, lib.ConfigAPIHub(), "PUT", url, reqJSON, &rspPut)
		if err != nil {
			return
		}

		vars[deviceUID] = rspPut.EnvironmentVariables

	}

	return

}

// Load env vars into metadata from a list of fleets and set their values
func varsSetFromFleets(appMetadata AppMetadata, uids []string, template Vars, flagVerbose bool) (vars map[string]Vars, err error) {

	vars = map[string]Vars{}

	for _, fleetUID := range uids {

		rspGet := notegoapi.GetFleetEnvironmentVariablesResponse{}
		url := fmt.Sprintf("/v1/projects/%s/fleets/%s/environment_variables", appMetadata.App.UID, fleetUID)
		err = reqHubV1(flagVerbose, lib.ConfigAPIHub(), "GET", url, nil, &rspGet)
		if err != nil {
			return
		}

		req := notegoapi.PutFleetEnvironmentVariablesRequest{}
		req.EnvironmentVariables = rspGet.EnvironmentVariables
		for k, v := range template {
			req.EnvironmentVariables[k] = v
		}

		var reqJSON []byte
		reqJSON, err = note.JSONMarshal(req)
		if err != nil {
			return
		}

		rspPut := notegoapi.PutFleetEnvironmentVariablesResponse{}
		err = reqHubV1(flagVerbose, lib.ConfigAPIHub(), "PUT", url, reqJSON, &rspPut)
		if err != nil {
			return
		}

		vars[fleetUID] = rspPut.EnvironmentVariables

	}

	return
}
