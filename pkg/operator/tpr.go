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
	"k8s.io/kubernetes/pkg/api"
	apierrors "k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/apis/extensions"
)

const (
	TPRGroup   = "storage.coreos.com"
	TPRVersion = "v1alpha1"

	TPRStorageNodeKind    = "storagenode"
	TPRStorageStatusKind  = "storagestatus"
	TPRStorageClusterKind = "storagecluster"

	PluralTPRStorageNodeKind    = TPRStorageNodeKind + "s"
	PluralTPRStorageStatusKind  = TPRStorageStatusKind + "es"
	PluralTPRStorageClusterKind = TPRStorageClusterKind + "s"

	tprStorageNode    = "storage-node." + TPRGroup
	tprStorageStatus  = "storage-status." + TPRGroup
	tprStorageCluster = "storage-cluster." + TPRGroup
)

var (
	tprPluralList = []string{PluralTPRStorageClusterKind,
		PluralTPRStorageNodeKind,
		PluralTPRStorageStatusKind}
)

func (c *Operator) createTPRs() error {
	tprs := []*extensions.ThirdPartyResource{
		{
			ObjectMeta: api.ObjectMeta{
				Name: tprStorageStatus,
			},
			Versions: []extensions.APIVersion{
				{Name: TPRVersion},
			},
			Description: "Status reports from Quartermaster managed storage",
		},
		{
			ObjectMeta: api.ObjectMeta{
				Name: tprStorageNode,
			},
			Versions: []extensions.APIVersion{
				{Name: TPRVersion},
			},
			Description: "Managed storage nodes via Quartermaster",
		},
		{
			ObjectMeta: api.ObjectMeta{
				Name: tprStorageCluster,
			},
			Versions: []extensions.APIVersion{
				{Name: TPRVersion},
			},
			Description: "Managed storage clusters via Quartermaster",
		},
	}
	tprClient := c.kclient.Extensions().ThirdPartyResources()
	for _, tpr := range tprs {
		_, err := tprClient.Create(tpr)
		if apierrors.IsAlreadyExists(err) {
			c.logger.Log("msg", "TPR already registered", "tpr", tpr.Name)
		} else if err != nil {
			return err
		} else {
			c.logger.Log("msg", "TPR created", "tpr", tpr.Name)
		}
	}

	// We have to wait for the TPRs to be ready. Otherwise the initial watch may fail.
	for _, pluralTpr := range tprPluralList {
		err := WaitForTPRReady(c.kclient.Core().RESTClient(),
			TPRGroup,
			TPRVersion,
			pluralTpr)
		if err != nil {
			return err
		}
	}

	return nil
}
