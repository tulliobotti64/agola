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

package command

import (
	"context"
	"fmt"
	"path"

	"github.com/sorintlab/agola/internal/services/gateway/common"
	"github.com/sorintlab/agola/internal/services/types"
	"github.com/sorintlab/agola/internal/util"

	"github.com/pkg/errors"
)

type CreateProjectRequest struct {
	Name                string
	ParentID            string
	RemoteSourceName    string
	RepoPath            string
	CurrentUserID       string
	SkipSSHHostKeyCheck bool
}

func (c *CommandHandler) CreateProject(ctx context.Context, req *CreateProjectRequest) (*types.Project, error) {
	if !util.ValidateName(req.Name) {
		return nil, util.NewErrBadRequest(errors.Errorf("invalid project name %q", req.Name))
	}

	user, resp, err := c.configstoreClient.GetUser(ctx, req.CurrentUserID)
	if err != nil {
		return nil, ErrFromRemote(resp, errors.Wrapf(err, "failed to get user %q", req.CurrentUserID))
	}

	rs, resp, err := c.configstoreClient.GetRemoteSourceByName(ctx, req.RemoteSourceName)
	if err != nil {
		c.log.Errorf("err: %+v", err)
		return nil, ErrFromRemote(resp, errors.Wrapf(err, "failed to get remote source %q", req.RemoteSourceName))
	}
	c.log.Infof("rs: %s", util.Dump(rs))
	var la *types.LinkedAccount
	for _, v := range user.LinkedAccounts {
		if v.RemoteSourceID == rs.ID {
			la = v
			break
		}
	}
	c.log.Infof("la: %s", util.Dump(la))
	if la == nil {
		return nil, errors.Errorf("user doesn't have a linked account for remote source %q", rs.Name)
	}

	gitsource, err := common.GetGitSource(rs, la)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create gitsource client")
	}

	repo, err := gitsource.GetRepoInfo(req.RepoPath)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get repository info from gitsource")
	}

	//cloneURL := fmt.Sprintf("git@%s:%s/%s.git", host, repoOwner, repoName)
	sshCloneURL := repo.SSHCloneURL
	c.log.Infof("sshCloneURL: %s", sshCloneURL)

	c.log.Infof("generating ssh key pairs")
	privateKey, _, err := util.GenSSHKeyPair(4096)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to generate ssh key pair")
	}

	parentID := req.ParentID
	if parentID == "" {
		// create project in current user namespace
		parentID = path.Join("user", user.UserName)
	}

	p := &types.Project{
		Name: req.Name,
		Parent: types.Parent{
			Type: types.ConfigTypeProjectGroup,
			ID:   parentID,
		},
		LinkedAccountID:     la.ID,
		RepoPath:            req.RepoPath,
		CloneURL:            sshCloneURL,
		SkipSSHHostKeyCheck: req.SkipSSHHostKeyCheck,
		SSHPrivateKey:       string(privateKey),
	}

	c.log.Infof("creating project")
	p, resp, err = c.configstoreClient.CreateProject(ctx, p)
	if err != nil {
		return nil, ErrFromRemote(resp, errors.Wrapf(err, "failed to create project"))
	}
	c.log.Infof("project %s created, ID: %s", p.Name, p.ID)

	return p, c.SetupProject(ctx, rs, la, &SetupProjectRequest{
		Project:  p,
		RepoPath: req.RepoPath,
	})
}

type SetupProjectRequest struct {
	Project  *types.Project
	RepoPath string
}

func (c *CommandHandler) SetupProject(ctx context.Context, rs *types.RemoteSource, la *types.LinkedAccount, conf *SetupProjectRequest) error {
	gitsource, err := common.GetGitSource(rs, la)
	if err != nil {
		return errors.Wrapf(err, "failed to create gitsource client")
	}

	pubKey, err := util.ExtractPublicKey([]byte(conf.Project.SSHPrivateKey))
	if err != nil {
		return errors.Wrapf(err, "failed to extract public key")
	}

	webhookURL := fmt.Sprintf("%s/webhooks?projectid=%s", c.apiExposedURL, conf.Project.ID)

	// generate deploy keys and webhooks containing the agola project id so we
	// can have multiple projects referencing the same remote repository and this
	// will trigger multiple different runs
	deployKeyName := fmt.Sprintf("agola deploy key - %s", conf.Project.ID)
	c.log.Infof("creating/updating deploy key: %s", string(pubKey))
	if err := gitsource.UpdateDeployKey(conf.RepoPath, deployKeyName, string(pubKey), true); err != nil {
		return errors.Wrapf(err, "failed to create deploy key")
	}
	c.log.Infof("deleting existing webhooks")
	if err := gitsource.DeleteRepoWebhook(conf.RepoPath, webhookURL); err != nil {
		return errors.Wrapf(err, "failed to delete repository webhook")
	}
	c.log.Infof("creating webhook to url: %s", webhookURL)
	if err := gitsource.CreateRepoWebhook(conf.RepoPath, webhookURL, ""); err != nil {
		return errors.Wrapf(err, "failed to create repository webhook")
	}

	return nil
}

func (c *CommandHandler) ReconfigProject(ctx context.Context, projectRef string) error {
	p, resp, err := c.configstoreClient.GetProject(ctx, projectRef)
	if err != nil {
		return ErrFromRemote(resp, errors.Wrapf(err, "failed to get project %q", projectRef))
	}

	user, resp, err := c.configstoreClient.GetUserByLinkedAccount(ctx, p.LinkedAccountID)
	if err != nil {
		return ErrFromRemote(resp, errors.Wrapf(err, "failed to get user with linked account id %q", p.LinkedAccountID))
	}

	la := user.LinkedAccounts[p.LinkedAccountID]
	c.log.Infof("la: %s", util.Dump(la))
	if la == nil {
		return errors.Errorf("linked account %q in user %q doesn't exist", p.LinkedAccountID, user.UserName)
	}

	rs, resp, err := c.configstoreClient.GetRemoteSource(ctx, la.RemoteSourceID)
	if err != nil {
		return ErrFromRemote(resp, errors.Wrapf(err, "failed to get remote source %q", la.RemoteSourceID))
	}

	return c.SetupProject(ctx, rs, la, &SetupProjectRequest{
		Project:  p,
		RepoPath: p.RepoPath,
	})
}
