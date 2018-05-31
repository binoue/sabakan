package web

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/cybozu-go/sabakan"
)

// Server is the sabakan server.
type Server struct {
	Model        sabakan.Model
	MyURL        *url.URL
	IPXEFirmware string
}

// Handler implements http.Handler
func (s Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/api/v1/") {
		s.handleAPIV1(w, r)
		return
	}

	renderError(r.Context(), w, APIErrNotFound)
}

func (s Server) handleAPIV1(w http.ResponseWriter, r *http.Request) {
	p := r.URL.Path[len("/api/v1/"):]

	switch {
	case p == "config/dhcp":
		s.handleConfigDHCP(w, r)
		return
	case p == "config/ipam":
		s.handleConfigIPAM(w, r)
		return
	case strings.HasPrefix(p, "crypts/"):
		s.handleCrypts(w, r)
		return
	case p == "boot/ipxe.efi":
		http.ServeFile(w, r, s.IPXEFirmware)
		return
	case strings.HasPrefix(p, "boot/coreos/"):
		s.handleCoreOS(w, r)
		return
	case strings.HasPrefix(p, "boot/ignitions/"):
		s.handleIgnitions(w, r)
		return
	case strings.HasPrefix(p, "machines"):
		s.handleMachines(w, r)
		return
	case p == "images/coreos" || strings.HasPrefix(p, "images/coreos/"):
		s.handleImages(w, r)
		return
	}

	renderError(r.Context(), w, APIErrNotFound)
}
