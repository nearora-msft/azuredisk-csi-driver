/*
Copyright The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Code generated by informer-gen. DO NOT EDIT.

package v1beta2

import (
	"context"
	time "time"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
	watch "k8s.io/apimachinery/pkg/watch"
	cache "k8s.io/client-go/tools/cache"
	azurediskv1beta2 "sigs.k8s.io/azuredisk-csi-driver/pkg/apis/azuredisk/v1beta2"
	versioned "sigs.k8s.io/azuredisk-csi-driver/pkg/apis/client/clientset/versioned"
	internalinterfaces "sigs.k8s.io/azuredisk-csi-driver/pkg/apis/client/informers/externalversions/internalinterfaces"
	v1beta2 "sigs.k8s.io/azuredisk-csi-driver/pkg/apis/client/listers/azuredisk/v1beta2"
)

// AzVolumeInformer provides access to a shared informer and lister for
// AzVolumes.
type AzVolumeInformer interface {
	Informer() cache.SharedIndexInformer
	Lister() v1beta2.AzVolumeLister
}

type azVolumeInformer struct {
	factory          internalinterfaces.SharedInformerFactory
	tweakListOptions internalinterfaces.TweakListOptionsFunc
	namespace        string
}

// NewAzVolumeInformer constructs a new informer for AzVolume type.
// Always prefer using an informer factory to get a shared informer instead of getting an independent
// one. This reduces memory footprint and number of connections to the server.
func NewAzVolumeInformer(client versioned.Interface, namespace string, resyncPeriod time.Duration, indexers cache.Indexers) cache.SharedIndexInformer {
	return NewFilteredAzVolumeInformer(client, namespace, resyncPeriod, indexers, nil)
}

// NewFilteredAzVolumeInformer constructs a new informer for AzVolume type.
// Always prefer using an informer factory to get a shared informer instead of getting an independent
// one. This reduces memory footprint and number of connections to the server.
func NewFilteredAzVolumeInformer(client versioned.Interface, namespace string, resyncPeriod time.Duration, indexers cache.Indexers, tweakListOptions internalinterfaces.TweakListOptionsFunc) cache.SharedIndexInformer {
	return cache.NewSharedIndexInformer(
		&cache.ListWatch{
			ListFunc: func(options v1.ListOptions) (runtime.Object, error) {
				if tweakListOptions != nil {
					tweakListOptions(&options)
				}
				return client.DiskV1beta2().AzVolumes(namespace).List(context.TODO(), options)
			},
			WatchFunc: func(options v1.ListOptions) (watch.Interface, error) {
				if tweakListOptions != nil {
					tweakListOptions(&options)
				}
				return client.DiskV1beta2().AzVolumes(namespace).Watch(context.TODO(), options)
			},
		},
		&azurediskv1beta2.AzVolume{},
		resyncPeriod,
		indexers,
	)
}

func (f *azVolumeInformer) defaultInformer(client versioned.Interface, resyncPeriod time.Duration) cache.SharedIndexInformer {
	return NewFilteredAzVolumeInformer(client, f.namespace, resyncPeriod, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc}, f.tweakListOptions)
}

func (f *azVolumeInformer) Informer() cache.SharedIndexInformer {
	return f.factory.InformerFor(&azurediskv1beta2.AzVolume{}, f.defaultInformer)
}

func (f *azVolumeInformer) Lister() v1beta2.AzVolumeLister {
	return v1beta2.NewAzVolumeLister(f.Informer().GetIndexer())
}
