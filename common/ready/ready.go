package ready

import (
	"net/http"
	"sentioxyz/sentio-core/common/utils"
)

type server struct {
	name   string
	probe  func() error
	ready  bool
	reason error
}

var servers []*server

func Registry(name string, probe func() error) {
	servers = append(servers, &server{
		name:  name,
		probe: probe,
	})
}

func checkReady() (map[string]string, bool) {
	status := make(map[string]string)
	ready := true
	for _, svr := range servers {
		status[svr.name] = ""
		if svr.ready {
			continue
		}
		if svr.reason = svr.probe(); svr.reason == nil {
			svr.ready = true
		} else {
			status[svr.name] = svr.reason.Error()
			ready = false
		}
	}
	return status, ready
}

func ServeHTTP(w http.ResponseWriter, r *http.Request) {
	status, ready := checkReady()
	w.Header().Add("Content-Type", "application/json")
	w.WriteHeader(utils.Select(ready, http.StatusOK, http.StatusServiceUnavailable))
	_, _ = w.Write([]byte(utils.MustJSONMarshal(status)))
}
