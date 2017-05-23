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
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/pkg/apis/extensions/v1beta1"
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
	tprs := []*v1beta1.ThirdPartyResource{
		{
			ObjectMeta: meta.ObjectMeta{
				Name: tprStorageStatus,
			},
			Versions: []v1beta1.APIVersion{
				{Name: TPRVersion},
			},
			Description: "Status reports from Quartermaster managed storage",
		},
		{
			ObjectMeta: meta.ObjectMeta{
				Name: tprStorageNode,
			},
			Versions: []v1beta1.APIVersion{
				{Name: TPRVersion},
			},
			Description: "Managed storage nodes via Quartermaster",
		},
		{
			ObjectMeta: meta.ObjectMeta{
				Name: tprStorageCluster,
			},
			Versions: []v1beta1.APIVersion{
				{Name: TPRVersion},
			},
			Description: "Managed storage clusters via Quartermaster",
		},
	}
	tprClient := c.kclient.ExtensionsV1beta1().ThirdPartyResources()
	for _, tpr := range tprs {
		_, err := tprClient.Create(tpr)
		if apierrors.IsAlreadyExists(err) {
			logger.Debug("%v already registered", tpr.GetName())
		} else if err != nil {
			return err
		} else {
			logger.Info("%v TPR created", tpr.GetName())
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
