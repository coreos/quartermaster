// Copyright 2016 The quartermaster Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package operator

import (
	"fmt"
	"net/http"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/pkg/api/v1"
	restclient "k8s.io/client-go/rest"
)

// WaitForTPRReady waits for a third party resource to be available
// for use.
func WaitForTPRReady(restClient restclient.Interface, tprGroup, tprVersion, tprName string) error {
	return wait.Poll(3*time.Second, 30*time.Second, func() (bool, error) {
		res := restClient.Get().AbsPath("apis", tprGroup, tprVersion, tprName).Do()
		err := res.Error()
		if err != nil {
			// RESTClient returns *errors.StatusError for any status codes < 200 or > 206
			// and http.Client.Do errors are returned directly.
			if se, ok := err.(*apierrors.StatusError); ok {
				if se.Status().Code == http.StatusNotFound {
					return false, nil
				}
			}
			return false, err
		}

		var statusCode int
		res.StatusCode(&statusCode)
		if statusCode != http.StatusOK {
			return false, fmt.Errorf("invalid status code: %d", statusCode)
		}

		return true, nil
	})
}

func WaitForDeploymentReady(client kubernetes.Interface, namespace, name string, available int32) error {
	return wait.Poll(3*time.Second, 10*time.Minute, func() (bool, error) {
		deployments := client.Extensions().Deployments(namespace)
		deployment, err := deployments.Get(name, meta.GetOptions{})
		if err != nil {
			return false, err
		}

		if available == deployment.Status.AvailableReplicas {
			return true, nil
		}
		return false, nil
	})
}

// PodRunningAndReady returns whether a pod is running and each container has
// passed it's ready state.
func PodRunningAndReady(pod v1.Pod) (bool, error) {
	switch pod.Status.Phase {
	case v1.PodFailed, v1.PodSucceeded:
		return false, fmt.Errorf("pod completed")
	case v1.PodRunning:
		for _, cond := range pod.Status.Conditions {
			if cond.Type != v1.PodReady {
				continue
			}
			return cond.Status == v1.ConditionTrue, nil
		}
		return false, fmt.Errorf("pod ready condition not found")
	}
	return false, nil
}
