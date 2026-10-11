package feature_test

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	fhttp "github.com/arandu-io/framework/http"
	"github.com/arandu-io/hesape/view"
)

// errViewFailed is what the failing view returns, as a published view whose
// data no longer matches it does.
var errViewFailed = errors.New("swaggertest: the view failed to render")

func init() {
	view.Register("swaggertest.failing", func(io.Writer, any) error { return errViewFailed })
}

// serveRecovering serves target and returns what the handler panicked with,
// which is how the router hands an error it has no status for to the
// application's recovery middleware and its error page.
func serveRecovering(router *fhttp.Router, w http.ResponseWriter, target string) (recovered any) {
	defer func() { recovered = recover() }()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, target, nil))
	return nil
}

func TestUIRouteDoesNotHideARenderErrorOfThePublishedView(t *testing.T) {
	t.Parallel()

	config := enabledConfig()
	config.ViewName = "swaggertest.failing"
	response := httptest.NewRecorder()
	recovered := serveRecovering(mountRendered(t, config), response, "/docs")

	err, _ := recovered.(error)
	if !errors.Is(err, errViewFailed) {
		t.Fatalf("the route answered %d with %d bytes and handed on %v, want the view's error for the error page", response.Code, response.Body.Len(), recovered)
	}
	if response.Body.Len() != 0 {
		t.Errorf("the route wrote a page over the view's error:\n%s", response.Body.String())
	}
}

// failingWriter is a connection that breaks on the first byte of the body,
// after the status has gone out.
type failingWriter struct {
	header       http.Header
	writeHeaders int
	writes       int
}

func (w *failingWriter) Header() http.Header { return w.header }
func (w *failingWriter) WriteHeader(int)     { w.writeHeaders++ }
func (w *failingWriter) Write([]byte) (int, error) {
	w.writes++
	return 0, errors.New("swaggertest: connection reset")
}

func TestUIRouteWritesNoSecondPageAfterTheViewStartedWriting(t *testing.T) {
	t.Parallel()

	writer := &failingWriter{header: http.Header{}}
	recovered := serveRecovering(mountRendered(t, enabledConfig()), writer, "/docs")

	if writer.writeHeaders != 1 || writer.writes != 1 {
		t.Errorf("the route wrote the status %d times and a body %d times, want once each: the view's", writer.writeHeaders, writer.writes)
	}
	if err, _ := recovered.(error); err == nil || !strings.Contains(err.Error(), "connection reset") {
		t.Errorf("the route handed on %v, want the write error", recovered)
	}
}

func TestUIRouteServesTheEmbeddedPageWhenNoViewIsRegistered(t *testing.T) {
	t.Parallel()

	config := enabledConfig()
	config.ViewName = unpublishedView
	withRenderer := mountRendered(t, config)
	withoutRenderer, _ := mount(t, config)
	_, embeddedPolicy := documentedPolicies(t)

	for name, router := range map[string]*fhttp.Router{"with a view layer": withRenderer, "without one": withoutRenderer} {
		response := request(router, "/docs")
		if response.Code != http.StatusOK {
			t.Fatalf("%s: GET /docs answered %d: %s", name, response.Code, response.Body.String())
		}
		body := response.Body.String()
		if !strings.Contains(body, "/docs/assets/5.32.14/swagger-ui-bundle.js") || strings.Contains(body, "arandu-swagger-container") {
			t.Errorf("%s: GET /docs did not answer with the embedded page:\n%s", name, body)
		}
		if got := response.Header().Get("Content-Security-Policy"); got != embeddedPolicy {
			t.Errorf("%s: Content-Security-Policy = %q, want the embedded page's %q", name, got, embeddedPolicy)
		}
	}
}

func TestUIRouteWithARegisteredViewAndNoViewLayerSaysSo(t *testing.T) {
	t.Parallel()

	router := fhttp.NewRouter()
	module := newModule(t, enabledConfig())
	module.Routes(router.ForModule(module.Name()))
	response := httptest.NewRecorder()
	recovered := serveRecovering(router, response, "/docs")

	err, _ := recovered.(error)
	if err == nil {
		t.Fatalf("the route answered %d and handed on %v, want the error ctx.View returned without a view layer", response.Code, recovered)
	}
	if response.Body.Len() != 0 {
		t.Errorf("the route wrote a page over the error:\n%s", response.Body.String())
	}
}
