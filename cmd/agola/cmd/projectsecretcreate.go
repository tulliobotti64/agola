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

package cmd

import (
	"context"
	"net/url"

	"github.com/pkg/errors"
	"github.com/sorintlab/agola/internal/services/gateway/api"
	"github.com/spf13/cobra"
)

var cmdProjectSecretCreate = &cobra.Command{
	Use:   "create",
	Short: "create a project secret",
	Run: func(cmd *cobra.Command, args []string) {
		if err := projectSecretCreate(cmd, args); err != nil {
			log.Fatalf("err: %v", err)
		}
	},
}

type projectSecretCreateOptions struct {
	projectID string
	name      string
}

var projectSecretCreateOpts projectSecretCreateOptions

func init() {
	flags := cmdProjectSecretCreate.Flags()

	flags.StringVar(&projectSecretCreateOpts.projectID, "project", "", "project id or full path)")
	flags.StringVarP(&projectSecretCreateOpts.name, "name", "n", "", "secret name")

	cmdProjectSecretCreate.MarkFlagRequired("project")
	cmdProjectSecretCreate.MarkFlagRequired("name")

	cmdProjectSecret.AddCommand(cmdProjectSecretCreate)
}

func projectSecretCreate(cmd *cobra.Command, args []string) error {
	gwclient := api.NewClient(gatewayURL, token)

	req := &api.CreateSecretRequest{
		Name: projectSecretCreateOpts.name,
	}

	log.Infof("creating project secret")
	secret, _, err := gwclient.CreateProjectSecret(context.TODO(), url.PathEscape(projectSecretCreateOpts.projectID), req)
	if err != nil {
		return errors.Wrapf(err, "failed to create project secret")
	}
	log.Infof("project secret %q created, ID: %q", secret.Name, secret.ID)

	return nil
}
