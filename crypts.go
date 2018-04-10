package sabakan

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"path"

	"github.com/asaskevich/govalidator"
	"github.com/coreos/etcd/clientv3"
	"github.com/coreos/etcd/clientv3/concurrency"
	"github.com/gorilla/mux"
)

type (
	cryptEntity struct {
		Path string `json:"path"`
		Key  string `json:"key"`
	}

	deleteResponseEntity struct {
		Path string `json:"path"`
	}

	deleteResponse []deleteResponseEntity
)

// InitCrypts initialize the handle functions for crypts
func InitCrypts(r *mux.Router, c *clientv3.Client, p string) {
	e := &etcdClient{c, p}
	e.initCryptsFunc(r)
}

func (e *etcdClient) initCryptsFunc(r *mux.Router) {
	r.HandleFunc("/crypts/{serial}/{path}", e.handleCryptsGet).Methods("GET")
	r.HandleFunc("/crypts/{serial}", e.handleCryptsPost).Methods("POST")
	r.HandleFunc("/crypts/{serial}", e.handleCryptsDelete).Methods("DELETE")
}

func makeDeleteResponse(gresp *clientv3.GetResponse, serial string) (deleteResponse, error) {
	entities := deleteResponse{}
	for _, ev := range gresp.Kvs {
		entities = append(entities, deleteResponseEntity{Path: string(ev.Key)})
	}
	return entities, nil
}

func (e *etcdClient) handleCryptsGet(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	serial := vars["serial"]
	diskPath := vars["path"]

	target := path.Join(e.prefix, "crypts", serial, diskPath)
	resp, err := e.client.Get(r.Context(), target)
	if err != nil {
		respError(w, err, http.StatusInternalServerError)
		return
	}
	if resp.Count == 0 {
		respError(w, fmt.Errorf("target %v not found", target), http.StatusNotFound)
		return
	}

	ev := resp.Kvs[0]
	entity := &cryptEntity{Path: string(diskPath), Key: string(ev.Value)}
	err = respWriter(w, entity, http.StatusOK)
	if err != nil {
		respError(w, err, http.StatusInternalServerError)
	}
}

func (e *etcdClient) handleCryptsPost(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	serial := vars["serial"]
	var received cryptEntity
	err := json.NewDecoder(r.Body).Decode(&received)
	if err != nil {
		respError(w, err, http.StatusInternalServerError)
		return
	}

	// Validation
	diskPath := received.Path
	key := received.Key
	if govalidator.IsNull(diskPath) {
		respError(w, errors.New("`diskPath` should not be empty"), http.StatusBadRequest)
		return
	}
	if govalidator.IsNull(key) {
		respError(w, errors.New("`key` should not be empty"), http.StatusBadRequest)
		return
	}

	//  Start mutex
	s, err := concurrency.NewSession(e.client)
	defer s.Close()
	if err != nil {
		respError(w, err, http.StatusInternalServerError)
		return
	}
	m := concurrency.NewMutex(s, "/sabakan-post-crypts-lock/")
	if err := m.Lock(context.TODO()); err != nil {
		respError(w, err, http.StatusInternalServerError)
		return
	}

	// Prohibit overwriting
	target := path.Join(e.prefix, "crypts", serial, diskPath)
	prev, err := e.client.Get(r.Context(), target)
	if err != nil {
		w.Write([]byte(err.Error() + "\n"))
		return
	}
	if prev.Count == 1 {
		respError(w, fmt.Errorf("target %v exists", target), http.StatusBadRequest)
		return
	}

	// Put crypts on etcd
	_, err = e.client.Txn(r.Context()).
		If(clientv3.Compare(clientv3.CreateRevision(target), "=", 0)).
		Then(clientv3.OpPut(target, key)).
		Else().
		Commit()
	if err != nil {
		respError(w, err, http.StatusInternalServerError)
		return
	}

	// Close mutex
	if err := m.Unlock(context.TODO()); err != nil {
		respError(w, err, http.StatusInternalServerError)
		return
	}

	entity := &cryptEntity{Path: string(diskPath), Key: string(key)}
	err = respWriter(w, entity, http.StatusCreated)
	if err != nil {
		respError(w, err, http.StatusInternalServerError)
	}
}

func (e *etcdClient) handleCryptsDelete(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	serial := vars["serial"]

	// GET current crypts
	gresp, err := e.client.Get(r.Context(),
		fmt.Sprintf("/crypts/%v", serial),
		clientv3.WithPrefix())
	if err != nil {
		respError(w, err, http.StatusInternalServerError)
		return
	}
	if len(gresp.Kvs) == 0 {
		respError(w, fmt.Errorf("target not found"), http.StatusNotFound)
		return
	}

	// DELETE
	dresp, err := e.client.Delete(r.Context(),
		fmt.Sprintf("/crypts/%v", serial),
		clientv3.WithPrefix())
	if err != nil {
		respError(w, err, http.StatusInternalServerError)
		return
	}
	if dresp.Deleted <= 0 {
		respError(w, fmt.Errorf("failed to delete"), http.StatusInternalServerError)
		return
	}

	entities, err := makeDeleteResponse(gresp, serial)
	if err != nil {
		respError(w, err, http.StatusInternalServerError)
	}

	err = respWriter(w, entities, http.StatusOK)
	if err != nil {
		respError(w, err, http.StatusInternalServerError)
	}
}
