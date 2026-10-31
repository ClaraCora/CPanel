package httpapi

import (
	"net/http"
	"time"
)

const requestBodyTimeout = 15 * time.Second

// Limit the time spent receiving request bodies without imposing a write
// timeout on WebSocket sessions or streaming responses.
func requestBodyDeadline(next http.Handler) http.Handler {
	return withRequestBodyDeadline(requestBodyTimeout, next)
}

func withRequestBodyDeadline(timeout time.Duration, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Body != nil && r.Body != http.NoBody {
			controller := http.NewResponseController(w)
			if err := controller.SetReadDeadline(time.Now().Add(timeout)); err == nil {
				defer func() {
					// Close/drain while the deadline is still in force. Clearing it
					// first would let net/http drain an unfinished slow body forever.
					_ = r.Body.Close()
					_ = controller.SetReadDeadline(time.Time{})
				}()
			}
		}
		next.ServeHTTP(w, r)
	})
}
