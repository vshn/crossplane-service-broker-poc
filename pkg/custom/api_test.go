package custom_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"broker/pkg/crossplane"
	"broker/pkg/custom"

	"code.cloudfoundry.org/lager"
	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
)

func newTestAPI() (func(), string) {
	logger := lager.NewLogger("testing")
	router := mux.NewRouter()
	cp := &crossplane.Crossplane{}
	handler := custom.NewAPIHandler(cp, logger)

	custom.NewAPI(router, handler, logger)

	ts := httptest.NewServer(router)
	return ts.Close, ts.URL
}

// func TestAPI_Endpoints(t *testing.T) {
// 	close, url := newTestAPI()
// 	defer close()

// 	res, err := http.Get(url + "/custom/service_instances/test/endpoint")
// 	assert.NoError(t, err)
// 	assert.Equal(t, http.StatusOK, res.StatusCode)
// 	assert.Equal(t, res.Header.Get("Content-Type"), "application/json")

// 	endpoints := []struct{}{}
// 	err = json.NewDecoder(res.Body).Decode(&endpoints)
// 	assert.NoError(t, err)
// 	res.Body.Close()

// 	assert.Len(t, endpoints, 0)
// }
func TestAPI_NotImplementedEndpoint(t *testing.T) {
	close, url := newTestAPI()
	defer close()

	res, err := http.Get(url + "/custom/service_instances/test/usage")
	assert.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, res.StatusCode)
	assert.Equal(t, res.Header.Get("Content-Type"), "application/json")

	notImplemented := struct {
		Error       string
		Description string
	}{}
	err = json.NewDecoder(res.Body).Decode(&notImplemented)
	assert.NoError(t, err)
	res.Body.Close()

	assert.Equal(t, notImplemented.Error, "API not implemented")
	assert.Equal(t, notImplemented.Description, "API not implemented")
}
