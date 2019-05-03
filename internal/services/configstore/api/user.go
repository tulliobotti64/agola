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
	"time"

	"github.com/pkg/errors"
	"github.com/sorintlab/agola/internal/db"
	"github.com/sorintlab/agola/internal/services/configstore/command"
	"github.com/sorintlab/agola/internal/services/configstore/readdb"
	"github.com/sorintlab/agola/internal/services/types"
	"github.com/sorintlab/agola/internal/util"

	"github.com/gorilla/mux"
	"go.uber.org/zap"
)

type UserHandler struct {
	log    *zap.SugaredLogger
	readDB *readdb.ReadDB
}

func NewUserHandler(logger *zap.Logger, readDB *readdb.ReadDB) *UserHandler {
	return &UserHandler{log: logger.Sugar(), readDB: readDB}
}

func (h *UserHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	userRef := vars["userref"]

	var user *types.User
	err := h.readDB.Do(func(tx *db.Tx) error {
		var err error
		user, err = h.readDB.GetUser(tx, userRef)
		return err
	})
	if err != nil {
		h.log.Errorf("err: %+v", err)
		httpError(w, err)
		return
	}

	if user == nil {
		httpError(w, util.NewErrNotFound(errors.Errorf("user %q doesn't exist", userRef)))
		return
	}

	if err := httpResponse(w, http.StatusOK, user); err != nil {
		h.log.Errorf("err: %+v", err)
	}
}

type CreateUserRequest struct {
	UserName string `json:"user_name"`

	CreateUserLARequest *CreateUserLARequest `json:"create_user_la_request"`
}

type CreateUserHandler struct {
	log *zap.SugaredLogger
	ch  *command.CommandHandler
}

func NewCreateUserHandler(logger *zap.Logger, ch *command.CommandHandler) *CreateUserHandler {
	return &CreateUserHandler{log: logger.Sugar(), ch: ch}
}

func (h *CreateUserHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var req *CreateUserRequest
	d := json.NewDecoder(r.Body)
	if err := d.Decode(&req); err != nil {
		httpError(w, util.NewErrBadRequest(err))
		return
	}

	creq := &command.CreateUserRequest{
		UserName: req.UserName,
	}
	if req.CreateUserLARequest != nil {
		creq.CreateUserLARequest = &command.CreateUserLARequest{
			RemoteSourceName:           req.CreateUserLARequest.RemoteSourceName,
			RemoteUserID:               req.CreateUserLARequest.RemoteUserID,
			RemoteUserName:             req.CreateUserLARequest.RemoteUserName,
			UserAccessToken:            req.CreateUserLARequest.UserAccessToken,
			Oauth2AccessToken:          req.CreateUserLARequest.Oauth2AccessToken,
			Oauth2RefreshToken:         req.CreateUserLARequest.Oauth2RefreshToken,
			Oauth2AccessTokenExpiresAt: req.CreateUserLARequest.Oauth2AccessTokenExpiresAt,
		}
	}

	user, err := h.ch.CreateUser(ctx, creq)
	if httpError(w, err) {
		h.log.Errorf("err: %+v", err)
		return
	}

	if err := httpResponse(w, http.StatusCreated, user); err != nil {
		h.log.Errorf("err: %+v", err)
	}
}

type UpdateUserRequest struct {
	UserName string `json:"user_name"`
}

type UpdateUserHandler struct {
	log *zap.SugaredLogger
	ch  *command.CommandHandler
}

func NewUpdateUserHandler(logger *zap.Logger, ch *command.CommandHandler) *UpdateUserHandler {
	return &UpdateUserHandler{log: logger.Sugar(), ch: ch}
}

func (h *UpdateUserHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	vars := mux.Vars(r)
	userRef := vars["userref"]

	var req *UpdateUserRequest
	d := json.NewDecoder(r.Body)
	if err := d.Decode(&req); err != nil {
		httpError(w, util.NewErrBadRequest(err))
		return
	}

	creq := &command.UpdateUserRequest{
		UserRef:  userRef,
		UserName: req.UserName,
	}

	user, err := h.ch.UpdateUser(ctx, creq)
	if httpError(w, err) {
		h.log.Errorf("err: %+v", err)
		return
	}

	if err := httpResponse(w, http.StatusCreated, user); err != nil {
		h.log.Errorf("err: %+v", err)
	}
}

type DeleteUserHandler struct {
	log *zap.SugaredLogger
	ch  *command.CommandHandler
}

func NewDeleteUserHandler(logger *zap.Logger, ch *command.CommandHandler) *DeleteUserHandler {
	return &DeleteUserHandler{log: logger.Sugar(), ch: ch}
}

func (h *DeleteUserHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	vars := mux.Vars(r)
	userRef := vars["userref"]

	err := h.ch.DeleteUser(ctx, userRef)
	if httpError(w, err) {
		h.log.Errorf("err: %+v", err)
	}
	if err := httpResponse(w, http.StatusNoContent, nil); err != nil {
		h.log.Errorf("err: %+v", err)
	}
}

const (
	DefaultUsersLimit = 10
	MaxUsersLimit     = 20
)

type UsersHandler struct {
	log    *zap.SugaredLogger
	readDB *readdb.ReadDB
}

func NewUsersHandler(logger *zap.Logger, readDB *readdb.ReadDB) *UsersHandler {
	return &UsersHandler{log: logger.Sugar(), readDB: readDB}
}

func (h *UsersHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()

	limitS := query.Get("limit")
	limit := DefaultUsersLimit
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
	if limit > MaxUsersLimit {
		limit = MaxUsersLimit
	}
	asc := false
	if _, ok := query["asc"]; ok {
		asc = true
	}

	start := query.Get("start")

	// handle special queries, like get user by token
	queryType := query.Get("query_type")
	h.log.Infof("query_type: %s", queryType)

	var users []*types.User
	switch queryType {
	case "bytoken":
		token := query.Get("token")
		var user *types.User
		err := h.readDB.Do(func(tx *db.Tx) error {
			var err error
			user, err = h.readDB.GetUserByTokenValue(tx, token)
			return err
		})
		h.log.Infof("user: %s", util.Dump(user))
		if err != nil {
			h.log.Errorf("err: %+v", err)
			httpError(w, err)
			return
		}
		if user == nil {
			httpError(w, util.NewErrNotFound(errors.Errorf("user with required token doesn't exist")))
			return
		}
		users = []*types.User{user}
	case "bylinkedaccount":
		linkedAccountID := query.Get("linkedaccountid")
		var user *types.User
		err := h.readDB.Do(func(tx *db.Tx) error {
			var err error
			user, err = h.readDB.GetUserByLinkedAccount(tx, linkedAccountID)
			return err
		})
		h.log.Infof("user: %s", util.Dump(user))
		if err != nil {
			h.log.Errorf("err: %+v", err)
			httpError(w, err)
			return
		}
		if user == nil {
			httpError(w, util.NewErrNotFound(errors.Errorf("user with linked account %q token doesn't exist", linkedAccountID)))
			return
		}
		users = []*types.User{user}
	case "byremoteuser":
		remoteUserID := query.Get("remoteuserid")
		remoteSourceID := query.Get("remotesourceid")
		var user *types.User
		err := h.readDB.Do(func(tx *db.Tx) error {
			var err error
			user, err = h.readDB.GetUserByLinkedAccountRemoteUserIDandSource(tx, remoteUserID, remoteSourceID)
			return err
		})
		h.log.Infof("user: %s", util.Dump(user))
		if err != nil {
			h.log.Errorf("err: %+v", err)
			httpError(w, err)
			return
		}
		if user == nil {
			httpError(w, util.NewErrNotFound(errors.Errorf("user with remote user %q for remote source %q token doesn't exist", remoteUserID, remoteSourceID)))
			return
		}
		users = []*types.User{user}
	default:
		// default query
		err := h.readDB.Do(func(tx *db.Tx) error {
			var err error
			users, err = h.readDB.GetUsers(tx, start, limit, asc)
			return err
		})
		if err != nil {
			h.log.Errorf("err: %+v", err)
			httpError(w, err)
			return
		}
	}

	if err := httpResponse(w, http.StatusOK, users); err != nil {
		h.log.Errorf("err: %+v", err)
	}
}

type CreateUserLARequest struct {
	RemoteSourceName           string    `json:"remote_source_name"`
	RemoteUserID               string    `json:"remote_user_id"`
	RemoteUserName             string    `json:"remote_user_name"`
	UserAccessToken            string    `json:"user_access_token"`
	Oauth2AccessToken          string    `json:"oauth2_access_token"`
	Oauth2RefreshToken         string    `json:"oauth2_refresh_token"`
	Oauth2AccessTokenExpiresAt time.Time `json:"oauth_2_access_token_expires_at"`
}

type CreateUserLAHandler struct {
	log *zap.SugaredLogger
	ch  *command.CommandHandler
}

func NewCreateUserLAHandler(logger *zap.Logger, ch *command.CommandHandler) *CreateUserLAHandler {
	return &CreateUserLAHandler{log: logger.Sugar(), ch: ch}
}

func (h *CreateUserLAHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	vars := mux.Vars(r)
	userRef := vars["userref"]

	var req CreateUserLARequest
	d := json.NewDecoder(r.Body)
	if err := d.Decode(&req); err != nil {
		httpError(w, util.NewErrBadRequest(err))
		return
	}

	creq := &command.CreateUserLARequest{
		UserRef:                    userRef,
		RemoteSourceName:           req.RemoteSourceName,
		RemoteUserID:               req.RemoteUserID,
		RemoteUserName:             req.RemoteUserName,
		UserAccessToken:            req.UserAccessToken,
		Oauth2AccessToken:          req.Oauth2AccessToken,
		Oauth2RefreshToken:         req.Oauth2RefreshToken,
		Oauth2AccessTokenExpiresAt: req.Oauth2AccessTokenExpiresAt,
	}
	user, err := h.ch.CreateUserLA(ctx, creq)
	if httpError(w, err) {
		h.log.Errorf("err: %+v", err)
		return
	}

	if err := httpResponse(w, http.StatusCreated, user); err != nil {
		h.log.Errorf("err: %+v", err)
	}
}

type DeleteUserLAHandler struct {
	log *zap.SugaredLogger
	ch  *command.CommandHandler
}

func NewDeleteUserLAHandler(logger *zap.Logger, ch *command.CommandHandler) *DeleteUserLAHandler {
	return &DeleteUserLAHandler{log: logger.Sugar(), ch: ch}
}

func (h *DeleteUserLAHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	vars := mux.Vars(r)
	userRef := vars["userref"]
	laID := vars["laid"]

	err := h.ch.DeleteUserLA(ctx, userRef, laID)
	if httpError(w, err) {
		h.log.Errorf("err: %+v", err)
	}
	if err := httpResponse(w, http.StatusNoContent, nil); err != nil {
		h.log.Errorf("err: %+v", err)
	}
}

type UpdateUserLARequest struct {
	RemoteUserID               string    `json:"remote_user_id"`
	RemoteUserName             string    `json:"remote_user_name"`
	UserAccessToken            string    `json:"user_access_token"`
	Oauth2AccessToken          string    `json:"oauth2_access_token"`
	Oauth2RefreshToken         string    `json:"oauth2_refresh_token"`
	Oauth2AccessTokenExpiresAt time.Time `json:"oauth_2_access_token_expires_at"`
}

type UpdateUserLAHandler struct {
	log *zap.SugaredLogger
	ch  *command.CommandHandler
}

func NewUpdateUserLAHandler(logger *zap.Logger, ch *command.CommandHandler) *UpdateUserLAHandler {
	return &UpdateUserLAHandler{log: logger.Sugar(), ch: ch}
}

func (h *UpdateUserLAHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	vars := mux.Vars(r)
	userRef := vars["userref"]
	linkedAccountID := vars["laid"]

	var req UpdateUserLARequest
	d := json.NewDecoder(r.Body)
	if err := d.Decode(&req); err != nil {
		httpError(w, util.NewErrBadRequest(err))
		return
	}

	creq := &command.UpdateUserLARequest{
		UserRef:                    userRef,
		LinkedAccountID:            linkedAccountID,
		RemoteUserID:               req.RemoteUserID,
		RemoteUserName:             req.RemoteUserName,
		UserAccessToken:            req.UserAccessToken,
		Oauth2AccessToken:          req.Oauth2AccessToken,
		Oauth2RefreshToken:         req.Oauth2RefreshToken,
		Oauth2AccessTokenExpiresAt: req.Oauth2AccessTokenExpiresAt,
	}
	user, err := h.ch.UpdateUserLA(ctx, creq)
	if httpError(w, err) {
		h.log.Errorf("err: %+v", err)
		return
	}

	if err := httpResponse(w, http.StatusOK, user); err != nil {
		h.log.Errorf("err: %+v", err)
	}
}

type CreateUserTokenRequest struct {
	TokenName string `json:"token_name"`
}

type CreateUserTokenResponse struct {
	Token string `json:"token"`
}

type CreateUserTokenHandler struct {
	log *zap.SugaredLogger
	ch  *command.CommandHandler
}

func NewCreateUserTokenHandler(logger *zap.Logger, ch *command.CommandHandler) *CreateUserTokenHandler {
	return &CreateUserTokenHandler{log: logger.Sugar(), ch: ch}
}

func (h *CreateUserTokenHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	vars := mux.Vars(r)
	userRef := vars["userref"]

	var req CreateUserTokenRequest
	d := json.NewDecoder(r.Body)
	if err := d.Decode(&req); err != nil {
		httpError(w, util.NewErrBadRequest(err))
		return
	}

	token, err := h.ch.CreateUserToken(ctx, userRef, req.TokenName)
	if httpError(w, err) {
		h.log.Errorf("err: %+v", err)
		return
	}

	resp := &CreateUserTokenResponse{
		Token: token,
	}
	if err := httpResponse(w, http.StatusCreated, resp); err != nil {
		h.log.Errorf("err: %+v", err)
	}
}

type DeleteUserTokenHandler struct {
	log *zap.SugaredLogger
	ch  *command.CommandHandler
}

func NewDeleteUserTokenHandler(logger *zap.Logger, ch *command.CommandHandler) *DeleteUserTokenHandler {
	return &DeleteUserTokenHandler{log: logger.Sugar(), ch: ch}
}

func (h *DeleteUserTokenHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	vars := mux.Vars(r)
	userRef := vars["userref"]
	tokenName := vars["tokenname"]

	err := h.ch.DeleteUserToken(ctx, userRef, tokenName)
	if httpError(w, err) {
		h.log.Errorf("err: %+v", err)
	}
	if err := httpResponse(w, http.StatusNoContent, nil); err != nil {
		h.log.Errorf("err: %+v", err)
	}
}
