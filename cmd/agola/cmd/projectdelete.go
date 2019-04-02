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

	"github.com/sorintlab/agola/internal/services/gateway/api"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var cmdProjectDelete = &cobra.Command{
	Use:   "delete",
	Short: "delete a project",
	Run: func(cmd *cobra.Command, args []string) {
		if err := projectDelete(cmd, args); err != nil {
			log.Fatalf("err: %v", err)
		}
	},
}

type projectDeleteOptions struct {
	projectRef string
}

var projectDeleteOpts projectDeleteOptions

func init() {
	flags := cmdProjectDelete.Flags()

	flags.StringVar(&projectDeleteOpts.projectRef, "project", "", "project id or full path")

	cmdProjectDelete.MarkFlagRequired("project")

	cmdProject.AddCommand(cmdProjectDelete)
}

func projectDelete(cmd *cobra.Command, args []string) error {
	gwclient := api.NewClient(gatewayURL, token)

	log.Infof("deleting project")

	if _, err := gwclient.DeleteProject(context.TODO(), projectDeleteOpts.projectRef); err != nil {
		return errors.Wrapf(err, "failed to delete project")
	}

	return nil
}
