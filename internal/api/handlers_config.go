package api

import (
	"net/http"

	"github.com/evilsocket/opensnitch-web/internal/ws"
)

func (a *API) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := a.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}

	client := ws.NewClient(a.hub, conn)
	a.hub.Register <- client

	go client.WritePump()
	go client.ReadPump()
}
