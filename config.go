package sabakan

import (
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"path"

	"github.com/coreos/etcd/clientv3"
	"github.com/gorilla/mux"
)

// Config is structure of the sabakan option
type Config struct {
	NodeIPv4Offset string `json:"node-ipv4-offset"`
	NodeRackShift  uint   `json:"node-rack-shift"`
	BMCIPv4Offset  string `json:"bmc-ipv4-offset"`
	BMCRackShift   uint   `json:"bmc-rack-shift"`
	NodeIPPerNode  uint   `json:"node-ip-per-node"`
	BMCIPPerNode   uint   `json:"bmc-ip-per-node"`
}

func (c *Config) validate() error {
	if _, _, err := net.ParseCIDR(c.NodeIPv4Offset); err != nil {
		return errors.New("node-ipv4-offset: " + ErrorInvalidValue)
	}
	if c.NodeRackShift == 0 {
		return errors.New("node-rack-shift: " + ErrorValueNotFound)
	}
	if _, _, err := net.ParseCIDR(c.BMCIPv4Offset); err != nil {
		return errors.New("bmc-ipv4-offset: " + ErrorInvalidValue)
	}
	if c.BMCRackShift == 0 {
		return errors.New("bmc-rack-shift: " + ErrorValueNotFound)
	}
	if c.NodeIPPerNode == 0 {
		return errors.New("node-ip-per-node: " + ErrorValueNotFound)
	}
	if c.BMCIPPerNode == 0 {
		return errors.New("bmc-ip-per-node: " + ErrorValueNotFound)
	}
	return nil
}

// EtcdClient is etcd3 client object
type EtcdClient struct {
	Client *clientv3.Client
	Prefix string
}

const (
	// EtcdKeyConfig is etcd key name for sabakan option
	EtcdKeyConfig = "/config"
	// EtcdKeyMachines is etcd key name for machines management
	EtcdKeyMachines = "/machines"
	// EtcdKeyCrypts is etcd key name for crypts management
	EtcdKeyCrypts = "/crypts"

	// ErrorInvalidValue is an error message when a target value is invalid
	ErrorInvalidValue = "invalid value"
	// ErrorValueNotFound is an error message when a target value is not found
	ErrorValueNotFound = "value not found"
	// ErrorMachinesExist is an error message when /machines key exists in etcd.
	ErrorMachinesExist = "machines already exist"
	// ErrorValueAlreadyExists is an error message when a target value already exists
	ErrorValueAlreadyExists = "value already exists"
)

// InitConfig is initialization of the sabakan API /config
func InitConfig(r *mux.Router, e *EtcdClient) {
	e.initConfigFunc(r)
}

func (e *EtcdClient) initConfigFunc(r *mux.Router) {
	r.HandleFunc("/config", e.handleGetConfig).Methods("GET")
	r.HandleFunc("/config", e.handlePostConfig).Methods("POST")
}

func (e *EtcdClient) handleGetConfig(w http.ResponseWriter, r *http.Request) {
	key := path.Join(e.Prefix, EtcdKeyConfig)
	resp, err := e.Client.Get(r.Context(), key)
	if err != nil {
		renderError(w, err, http.StatusInternalServerError)
		return
	}
	if resp == nil {
		renderError(w, errors.New(ErrorValueNotFound), http.StatusNotFound)
		return
	}
	if len(resp.Kvs) == 0 {
		renderError(w, errors.New(ErrorValueNotFound), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, err = w.Write(resp.Kvs[0].Value)
	if err != nil {
		renderError(w, err, http.StatusInternalServerError)
		return
	}
}

func (e *EtcdClient) handlePostConfig(w http.ResponseWriter, r *http.Request) {
	key := path.Join(e.Prefix, EtcdKeyMachines)
	resp, err := e.Client.Get(r.Context(), key, clientv3.WithPrefix())
	if err != nil {
		renderError(w, err, http.StatusInternalServerError)
		return
	}
	if resp.Count != 0 {
		renderError(w, errors.New(ErrorMachinesExist), http.StatusForbidden)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	var sc Config
	err = json.NewDecoder(r.Body).Decode(&sc)
	if err != nil {
		renderError(w, err, http.StatusBadRequest)
		return
	}
	err = sc.validate()
	if err != nil {
		renderError(w, err, http.StatusBadRequest)
		return
	}

	j, err := json.Marshal(sc)
	if err != nil {
		renderError(w, err, http.StatusInternalServerError)
		return
	}

	// Put config
	key = path.Join(e.Prefix, EtcdKeyConfig)
	_, err = e.Client.Put(r.Context(), key, string(j))
	if err != nil {
		renderError(w, err, http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}
