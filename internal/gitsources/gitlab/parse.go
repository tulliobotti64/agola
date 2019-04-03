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

package gitlab

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/sorintlab/agola/internal/services/types"

	"github.com/pkg/errors"
)

const (
	hookEvent = "X-Gitlab-Event"

	hookPush        = "Push Hook"
	hookTagPush     = "Tag Push Hook"
	hookPullRequest = "Merge Request Hook"

	prStateOpen = "open"

	prActionOpen = "opened"
	prActionSync = "synchronized"
)

func parseWebhook(r *http.Request) (*types.WebhookData, error) {
	switch r.Header.Get(hookEvent) {
	case hookPush:
		return parsePushHook(r.Body)
	case hookTagPush:
		return parsePushHook(r.Body)
	case hookPullRequest:
		return parsePullRequestHook(r.Body)
	default:
		return nil, errors.Errorf("unknown webhook event type: %q", r.Header.Get(hookEvent))
	}
}

func parsePush(r io.Reader) (*pushHook, error) {
	push := new(pushHook)
	err := json.NewDecoder(r).Decode(push)
	return push, err
}

func parsePullRequest(r io.Reader) (*pullRequestHook, error) {
	pr := new(pullRequestHook)
	err := json.NewDecoder(r).Decode(pr)
	return pr, err
}

func parsePushHook(payload io.Reader) (*types.WebhookData, error) {
	push, err := parsePush(payload)
	if err != nil {
		return nil, err
	}

	// skip push events with 0 commits. i.e. a tag deletion.
	if len(push.Commits) == 0 {
		return nil, nil
	}

	return webhookDataFromPush(push)
}

func parsePullRequestHook(payload io.Reader) (*types.WebhookData, error) {
	prhook, err := parsePullRequest(payload)
	if err != nil {
		return nil, err
	}

	//	// skip non open pull requests
	//	if prhook.PullRequest.State != prStateOpen {
	//		return nil, nil
	//	}
	//	// only accept actions that have new commits
	//	if prhook.Action != prActionOpen && prhook.Action != prActionSync {
	//		return nil, nil
	//	}

	return webhookDataFromPullRequest(prhook), nil
}

func webhookDataFromPush(hook *pushHook) (*types.WebhookData, error) {
	sender := hook.UserName
	if sender == "" {
		sender = hook.UserUsername
	}

	// common data
	whd := &types.WebhookData{
		CommitSHA:  hook.After,
		Ref:        hook.Ref,
		CommitLink: hook.Commits[0].URL,
		//CompareLink: hook.Compare,
		//CommitLink: fmt.Sprintf("%s/commit/%s", hook.Repo.URL, hook.After),
		Sender: sender,

		Repo: types.WebhookDataRepo{
			Path:   hook.Project.PathWithNamespace,
			WebURL: hook.Project.WebURL,
		},
	}

	switch {
	case strings.HasPrefix(hook.Ref, "refs/heads/"):
		whd.Event = types.WebhookEventPush
		whd.Branch = strings.TrimPrefix(hook.Ref, "refs/heads/")
		whd.BranchLink = fmt.Sprintf("%s/tree/%s", hook.Project.WebURL, whd.Branch)
		if len(hook.Commits) > 0 {
			whd.Message = hook.Commits[0].Message
		}
	case strings.HasPrefix(hook.Ref, "refs/tags/"):
		whd.Event = types.WebhookEventTag
		whd.Tag = strings.TrimPrefix(hook.Ref, "refs/tags/")
		whd.TagLink = fmt.Sprintf("%s/tree/%s", hook.Project.WebURL, whd.Tag)
		whd.Message = fmt.Sprintf("Tag %s", whd.Tag)
	default:
		// ignore received webhook since it doesn't have a ref we're interested in
		return nil, fmt.Errorf("unsupported webhook ref %q", hook.Ref)
	}

	return whd, nil
}

// helper function that extracts the Build data from a Gitea pull_request hook
func webhookDataFromPullRequest(hook *pullRequestHook) *types.WebhookData {
	// TODO(sgotti) Use PR opener username or last commit user name?
	sender := hook.User.Name
	if sender == "" {
		sender = hook.User.Username
	}
	//sender := hook.ObjectAttributes.LastCommit.Author.Name
	//if sender == "" {
	//	sender := hook.ObjectAttributes.LastCommit.Author.UserName
	//}
	build := &types.WebhookData{
		Event:     types.WebhookEventPullRequest,
		CommitSHA: hook.ObjectAttributes.LastCommit.ID,
		Ref:       fmt.Sprintf("refs/merge-requests/%d/head", hook.ObjectAttributes.Iid),
		//CommitLink:      fmt.Sprintf("%s/commit/%s", hook.Repo.URL, hook.PullRequest.Head.Sha),
		CommitLink:      hook.ObjectAttributes.LastCommit.URL,
		Branch:          hook.ObjectAttributes.SourceBranch,
		Message:         hook.ObjectAttributes.Title,
		Sender:          sender,
		PullRequestID:   strconv.Itoa(hook.ObjectAttributes.Iid),
		PullRequestLink: hook.ObjectAttributes.URL,

		Repo: types.WebhookDataRepo{
			Path:   hook.Project.PathWithNamespace,
			WebURL: hook.Project.WebURL,
		},
	}
	return build
}
