package httpsem

import "net/http"

// StatusAllowsBody reports whether a response status permits a message body.
func StatusAllowsBody(status int) bool {
	if status >= 100 && status < 200 {
		return false
	}
	switch status {
	case http.StatusNoContent, http.StatusResetContent, http.StatusNotModified:
		return false
	default:
		return true
	}
}
