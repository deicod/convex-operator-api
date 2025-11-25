package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/deicod/convex-operator-api/internal/kube"
	"github.com/deicod/convex-operator/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type stubWatchProvider struct {
	watchFn func(ctx context.Context, namespace string) (kube.InstanceWatchStream, error)
}

func (s stubWatchProvider) Watch(ctx context.Context, namespace string) (kube.InstanceWatchStream, error) {
	return s.watchFn(ctx, namespace)
}

type stubWatchStream struct {
	ch         <-chan kube.InstanceWatchEvent
	stopCalled bool
}

func (s *stubWatchStream) ResultChan() <-chan kube.InstanceWatchEvent { return s.ch }

func (s *stubWatchStream) Stop() {
	s.stopCalled = true
}

type flushingResponseRecorder struct {
	*httptest.ResponseRecorder
}

func (f *flushingResponseRecorder) Flush() {}

func TestWatchHandlerStreamsEvents(t *testing.T) {
	events := make(chan kube.InstanceWatchEvent, 1)
	stream := &stubWatchStream{ch: events}

	handler := WatchHandler{
		Watcher: stubWatchProvider{
			watchFn: func(ctx context.Context, ns string) (kube.InstanceWatchStream, error) {
				_ = ctx
				if ns != "" {
					t.Fatalf("expected empty namespace, got %q", ns)
				}
				return stream, nil
			},
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/instances/watch", nil)
	rec := &flushingResponseRecorder{httptest.NewRecorder()}

	done := make(chan struct{})
	go func() {
		handler.ServeHTTP(rec, req)
		close(done)
	}()

	events <- kube.InstanceWatchEvent{
		Type: "ADDED",
		Object: v1alpha1.ConvexInstance{
			ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "ns1"},
			Spec:       v1alpha1.ConvexInstanceSpec{Environment: "dev", Version: "1.0.0"},
			Status:     v1alpha1.ConvexInstanceStatus{Phase: "Ready"},
		},
	}
	close(events)
	<-done

	body := rec.Body.String()
	if !strings.Contains(body, "event: ADDED") {
		t.Fatalf("expected SSE event line, got %q", body)
	}
	if !strings.Contains(body, `"name":"demo"`) || !strings.Contains(body, `"namespace":"ns1"`) {
		t.Fatalf("expected instance summary in payload, got %q", body)
	}
	if !stream.stopCalled {
		t.Fatalf("expected stream to be stopped")
	}
}

func TestWatchHandlerFiltersNamespaces(t *testing.T) {
	events := make(chan kube.InstanceWatchEvent, 1)
	stream := &stubWatchStream{ch: events}

	handler := WatchHandler{
		AllowedNamespaces: []string{"allowed"},
		Watcher: stubWatchProvider{
			watchFn: func(ctx context.Context, ns string) (kube.InstanceWatchStream, error) {
				_ = ctx
				if ns != "" {
					t.Fatalf("unexpected namespace parameter %q", ns)
				}
				return stream, nil
			},
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/instances/watch", nil)
	rec := &flushingResponseRecorder{httptest.NewRecorder()}

	done := make(chan struct{})
	go func() {
		handler.ServeHTTP(rec, req)
		close(done)
	}()

	events <- kube.InstanceWatchEvent{
		Type: "ADDED",
		Object: v1alpha1.ConvexInstance{
			ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "denied"},
		},
	}
	close(events)
	<-done

	if body := rec.Body.String(); body != "" {
		t.Fatalf("expected filtered events to produce no output, got %q", body)
	}
}

func TestWatchHandlerHonorsNamespaceQuery(t *testing.T) {
	stream := &stubWatchStream{ch: make(chan kube.InstanceWatchEvent)}
	handler := WatchHandler{
		AllowedNamespaces: []string{"ns1"},
		Watcher: stubWatchProvider{
			watchFn: func(ctx context.Context, ns string) (kube.InstanceWatchStream, error) {
				_ = ctx
				if ns != "ns1" {
					t.Fatalf("expected namespace parameter to propagate, got %q", ns)
				}
				return stream, nil
			},
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	req := httptest.NewRequest(http.MethodGet, "/api/v1/instances/watch?namespace=ns1", nil).WithContext(ctx)
	rec := &flushingResponseRecorder{httptest.NewRecorder()}

	done := make(chan struct{})
	go func() {
		handler.ServeHTTP(rec, req)
		close(done)
	}()

	time.Sleep(10 * time.Millisecond)
	cancel()
	<-done
	if !stream.stopCalled {
		t.Fatalf("expected stream to be stopped on client disconnect")
	}
}
