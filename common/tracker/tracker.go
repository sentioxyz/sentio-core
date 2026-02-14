package tracker

import (
	"bytes"
	"encoding/json"
	"net/http"
	"sync"
)

type TrackedObject interface {
	Snapshot() interface{}
}

type Tracker struct {
	TrackedObjects map[string]TrackedObject
	mutex          sync.Mutex
}

func NewTracker() *Tracker {
	return &Tracker{
		TrackedObjects: make(map[string]TrackedObject),
	}
}

func (t *Tracker) TrackObject(name string, object TrackedObject) {
	t.mutex.Lock()
	defer t.mutex.Unlock()
	t.TrackedObjects[name] = object
}

func (t *Tracker) Snapshot() map[string]interface{} {
	t.mutex.Lock()
	defer t.mutex.Unlock()
	snapshot := make(map[string]interface{})
	for k, v := range t.TrackedObjects {
		snapshot[k] = v.Snapshot()
	}
	return snapshot
}

var GlobalTracker *Tracker = NewTracker()

func AddOrReplaceTrackedObject(name string, object TrackedObject) {
	GlobalTracker.TrackObject(name, object)
}

type wrappedObject struct {
	fn func() any
}

func (w *wrappedObject) Snapshot() interface{} {
	return w.fn()
}

func AddOrReplaceTrackedFunc(name string, fn func() any) {
	AddOrReplaceTrackedObject(name, &wrappedObject{fn: fn})
}

func SnapshotTrackedObjects() map[string]interface{} {
	return GlobalTracker.Snapshot()
}

func HTTPHandler(w http.ResponseWriter, r *http.Request) {
	var body bytes.Buffer
	err := json.NewEncoder(&body).Encode(SnapshotTrackedObjects())
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(err.Error()))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(body.Bytes())
}
