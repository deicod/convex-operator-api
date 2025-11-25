package kube

import (
	"context"

	"github.com/deicod/convex-operator/api/v1alpha1"
	"k8s.io/apimachinery/pkg/watch"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// InstanceWatchEvent represents a watch notification for a ConvexInstance.
type InstanceWatchEvent struct {
	Type   string
	Object v1alpha1.ConvexInstance
}

// InstanceWatchStream is a minimal watch stream interface.
type InstanceWatchStream interface {
	ResultChan() <-chan InstanceWatchEvent
	Stop()
}

// InstanceWatcher streams ConvexInstance changes using a watch-capable client.
type InstanceWatcher struct {
	Client client.WithWatch
}

// Watch starts a watch for ConvexInstances, optionally scoped to a namespace.
func (w InstanceWatcher) Watch(ctx context.Context, namespace string) (InstanceWatchStream, error) {
	list := &v1alpha1.ConvexInstanceList{}
	var opts []client.ListOption
	if namespace != "" {
		opts = append(opts, client.InNamespace(namespace))
	}

	wi, err := w.Client.Watch(ctx, list, opts...)
	if err != nil {
		return nil, err
	}

	out := make(chan InstanceWatchEvent, 1)
	go func() {
		defer close(out)
		for ev := range wi.ResultChan() {
			ci, ok := ev.Object.(*v1alpha1.ConvexInstance)
			if !ok || ci == nil {
				continue
			}
			out <- InstanceWatchEvent{
				Type:   string(ev.Type),
				Object: *ci,
			}
		}
	}()

	if ctx.Done() != nil {
		go func() {
			<-ctx.Done()
			wi.Stop()
		}()
	}

	return &instanceWatchAdapter{watcher: wi, ch: out}, nil
}

type instanceWatchAdapter struct {
	watcher watch.Interface
	ch      <-chan InstanceWatchEvent
}

func (a *instanceWatchAdapter) ResultChan() <-chan InstanceWatchEvent { return a.ch }

func (a *instanceWatchAdapter) Stop() {
	a.watcher.Stop()
}
