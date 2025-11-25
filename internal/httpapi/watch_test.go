package httpapi

import (
	"context"
	"fmt"
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
	var called []string
	handler := WatchHandler{
		AllowedNamespaces: []string{"allowed"},
		Watcher: stubWatchProvider{
			watchFn: func(ctx context.Context, ns string) (kube.InstanceWatchStream, error) {
				_ = ctx
				called = append(called, ns)
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
	if len(called) != 1 || called[0] != "allowed" {
		t.Fatalf("expected watch to scope to allowed namespace, got %v", called)
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

func TestWatchHandlerScansAllAllowedWhenNoNamespaceProvided(t *testing.T) {
	events1 := make(chan kube.InstanceWatchEvent, 1)
	events2 := make(chan kube.InstanceWatchEvent, 1)
	stream1 := &stubWatchStream{ch: events1}
	stream2 := &stubWatchStream{ch: events2}

	var seen []string
	handler := WatchHandler{
		AllowedNamespaces: []string{"ns1", "ns2"},
		Watcher: stubWatchProvider{
			watchFn: func(ctx context.Context, ns string) (kube.InstanceWatchStream, error) {
				_ = ctx
				seen = append(seen, ns)
				if ns == "ns1" {
					return stream1, nil
				}
				if ns == "ns2" {
					return stream2, nil
				}
				return nil, fmt.Errorf("unexpected namespace %s", ns)
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

	events2 <- kube.InstanceWatchEvent{
		Type: "ADDED",
		Object: v1alpha1.ConvexInstance{
			ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "ns2"},
			Spec:       v1alpha1.ConvexInstanceSpec{Environment: "dev", Version: "1.0.0"},
		},
	}
	close(events1)
	close(events2)
	<-done

	if len(seen) != 2 || !contains(seen, "ns1") || !contains(seen, "ns2") {
		t.Fatalf("expected watches for both allowed namespaces, got %v", seen)
	}
	if body := rec.Body.String(); !strings.Contains(body, `"namespace":"ns2"`) {
		t.Fatalf("expected streamed event for ns2, got %q", body)
	}
}

func contains(list []string, val string) bool {
	for _, v := range list {
		if v == val {
			return true
		}
	}
	return false
}
