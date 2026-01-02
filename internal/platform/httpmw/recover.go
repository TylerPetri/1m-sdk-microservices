package httpmw

import (
	"net/http"
	"runtime/debug"

	"go.uber.org/zap"
)

func Recover(log *zap.Logger, next http.Handler) http.Handler {
	if log == nil {
		log = zap.NewNop()
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if v := recover(); v != nil {
				log.Error("panic recovered",
					zap.Any("panic", v),
					zap.ByteString("stack", debug.Stack()),
				)
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			}
		}()

		next.ServeHTTP(w, r)
	})
}
