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
	"strconv"

	"github.com/pkg/errors"
	csapi "github.com/sorintlab/agola/internal/services/configstore/api"
	"github.com/sorintlab/agola/internal/services/gateway/command"
	"github.com/sorintlab/agola/internal/services/types"
	"github.com/sorintlab/agola/internal/util"
	"go.uber.org/zap"

	"github.com/gorilla/mux"
)

type CreateOrgRequest struct {
	Name string `json:"name"`
}

type CreateOrgHandler struct {
	log *zap.SugaredLogger
	ch  *command.CommandHandler
}

func NewCreateOrgHandler(logger *zap.Logger, ch *command.CommandHandler) *CreateOrgHandler {
	return &CreateOrgHandler{log: logger.Sugar(), ch: ch}
}

func (h *CreateOrgHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var userID string
	userIDVal := ctx.Value("userid")
	if userIDVal != nil {
		userID = userIDVal.(string)
	}
	h.log.Infof("userID: %q", userID)

	var req CreateOrgRequest
	d := json.NewDecoder(r.Body)
	if err := d.Decode(&req); err != nil {
		httpError(w, util.NewErrBadRequest(err))
		return
	}

	creq := &command.CreateOrgRequest{
		Name:          req.Name,
		CreatorUserID: userID,
	}

	org, err := h.ch.CreateOrg(ctx, creq)
	if httpError(w, err) {
		h.log.Errorf("err: %+v", err)
		return
	}

	res := createOrgResponse(org)
	if err := httpResponse(w, http.StatusCreated, res); err != nil {
		h.log.Errorf("err: %+v", err)
	}
}

type DeleteOrgHandler struct {
	log               *zap.SugaredLogger
	configstoreClient *csapi.Client
}

func NewDeleteOrgHandler(logger *zap.Logger, configstoreClient *csapi.Client) *DeleteOrgHandler {
	return &DeleteOrgHandler{log: logger.Sugar(), configstoreClient: configstoreClient}
}

func (h *DeleteOrgHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	vars := mux.Vars(r)
	orgRef := vars["orgref"]

	resp, err := h.configstoreClient.DeleteOrg(ctx, orgRef)
	if httpErrorFromRemote(w, resp, err) {
		h.log.Errorf("err: %+v", err)
		return
	}

	if err := httpResponse(w, http.StatusNoContent, nil); err != nil {
		h.log.Errorf("err: %+v", err)
	}
}

type OrgHandler struct {
	log               *zap.SugaredLogger
	configstoreClient *csapi.Client
}

func NewOrgHandler(logger *zap.Logger, configstoreClient *csapi.Client) *OrgHandler {
	return &OrgHandler{log: logger.Sugar(), configstoreClient: configstoreClient}
}

func (h *OrgHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	vars := mux.Vars(r)
	orgRef := vars["orgref"]

	org, resp, err := h.configstoreClient.GetOrg(ctx, orgRef)
	if httpErrorFromRemote(w, resp, err) {
		h.log.Errorf("err: %+v", err)
		return
	}

	res := createOrgResponse(org)
	if err := httpResponse(w, http.StatusOK, res); err != nil {
		h.log.Errorf("err: %+v", err)
	}
}

type OrgResponse struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func createOrgResponse(o *types.Organization) *OrgResponse {
	org := &OrgResponse{
		ID:   o.ID,
		Name: o.Name,
	}
	return org
}

type OrgsHandler struct {
	log               *zap.SugaredLogger
	configstoreClient *csapi.Client
}

func NewOrgsHandler(logger *zap.Logger, configstoreClient *csapi.Client) *OrgsHandler {
	return &OrgsHandler{log: logger.Sugar(), configstoreClient: configstoreClient}
}

func (h *OrgsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	query := r.URL.Query()

	limitS := query.Get("limit")
	limit := DefaultRunsLimit
	if limitS != "" {
		var err error
		limit, err = strconv.Atoi(limitS)
		if err != nil {
			httpError(w, util.NewErrBadRequest(errors.Wrapf(err, "cannot parse limit")))
			return
		}
	}
	if limit < 0 {
		httpError(w, util.NewErrBadRequest(errors.Errorf("limit must be greater or equal than 0")))
		return
	}
	if limit > MaxRunsLimit {
		limit = MaxRunsLimit
	}
	asc := false
	if _, ok := query["asc"]; ok {
		asc = true
	}

	start := query.Get("start")

	csorgs, resp, err := h.configstoreClient.GetOrgs(ctx, start, limit, asc)
	if httpErrorFromRemote(w, resp, err) {
		h.log.Errorf("err: %+v", err)
		return
	}

	orgs := make([]*OrgResponse, len(csorgs))
	for i, p := range csorgs {
		orgs[i] = createOrgResponse(p)
	}
	if err := httpResponse(w, http.StatusOK, orgs); err != nil {
		h.log.Errorf("err: %+v", err)
	}
}
