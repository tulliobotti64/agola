// Copyright 2019 Sorint.lab
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied
// See the License for the specific language governing permissions and
// limitations under the License.

package api

import (
	"encoding/json"
	"net/http"
	"net/url"

	"github.com/gorilla/mux"
	"github.com/pkg/errors"
	"github.com/sorintlab/agola/internal/services/types"
	"github.com/sorintlab/agola/internal/util"
)

type ErrorResponse struct {
	Message string `json:"message"`
}

func ErrorResponseFromError(err error) *ErrorResponse {
	switch {
	case util.IsErrBadRequest(err):
		fallthrough
	case util.IsErrNotFound(err):
		return &ErrorResponse{Message: err.Error()}
	}

	// on generic error return an generic message to not leak the real error
	return &ErrorResponse{Message: "internal server error"}
}

func httpError(w http.ResponseWriter, err error) bool {
	if err == nil {
		return false
	}

	response := ErrorResponseFromError(err)
	resj, merr := json.Marshal(response)
	if merr != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return true
	}
	switch {
	case util.IsErrBadRequest(err):
		w.WriteHeader(http.StatusBadRequest)
		w.Write(resj)
	case util.IsErrNotFound(err):
		w.WriteHeader(http.StatusNotFound)
		w.Write(resj)
	default:
		w.WriteHeader(http.StatusInternalServerError)
		w.Write(resj)
	}
	return true
}

func httpResponse(w http.ResponseWriter, code int, res interface{}) error {
	w.Header().Set("Content-Type", "application/json")

	if res != nil {
		resj, err := json.Marshal(res)
		if err != nil {
			httpError(w, err)
			return err
		}
		w.WriteHeader(code)
		_, err = w.Write(resj)
		return err
	}

	w.WriteHeader(code)
	return nil
}

func GetConfigTypeRef(r *http.Request) (types.ConfigType, string, error) {
	vars := mux.Vars(r)
	projectRef, err := url.PathUnescape(vars["projectref"])
	if err != nil {
		return "", "", util.NewErrBadRequest(errors.Wrapf(err, "wrong projectref %q", vars["projectref"]))
	}
	if projectRef != "" {
		return types.ConfigTypeProject, projectRef, nil
	}

	projectGroupRef, err := url.PathUnescape(vars["projectgroupref"])
	if err != nil {
		return "", "", util.NewErrBadRequest(errors.Wrapf(err, "wrong projectgroupref %q", vars["projectgroupref"]))
	}
	if projectGroupRef != "" {
		return types.ConfigTypeProjectGroup, projectGroupRef, nil
	}

	return "", "", util.NewErrBadRequest(errors.Errorf("cannot get project or projectgroup ref"))
}
