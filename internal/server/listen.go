package server

import (
	"errors"
	"fmt"
	"net"
	"strconv"
	"syscall"
)

// listen ouvre le port du serveur.
//
// Pour "localhost", Go n'écoute que sur la première adresse résolue,
// 127.0.0.1. Un autre programme pouvait alors tenir le même port en IPv6
// (Docker publie sur *:port) sans que rien ne le signale : le navigateur, qui
// essaie ::1 d'abord, tombait sur lui et recevait un 404 qui ne venait pas de
// Lorag. On écoute donc sur les deux, et un port pris d'un côté ou de l'autre
// arrête le démarrage avec un message clair. Une machine sans IPv6 n'écoute
// qu'en IPv4.
func listen(host string, port int) ([]net.Listener, error) {
	p := strconv.Itoa(port)
	if host != "" && host != "localhost" {
		l, err := net.Listen("tcp", net.JoinHostPort(host, p))
		if err != nil {
			return nil, portError(net.JoinHostPort(host, p), err)
		}
		return []net.Listener{l}, nil
	}

	v4, err := net.Listen("tcp4", net.JoinHostPort("127.0.0.1", p))
	if err != nil {
		return nil, portError("127.0.0.1:"+p, err)
	}
	v6, err := net.Listen("tcp6", net.JoinHostPort("::1", p))
	if err != nil {
		if errors.Is(err, syscall.EADDRINUSE) {
			v4.Close()
			return nil, portError("[::1]:"+p, err)
		}
		// Pas d'IPv6 sur cette machine : l'IPv4 suffit.
		return []net.Listener{v4}, nil
	}
	return []net.Listener{v4, v6}, nil
}

func portError(addr string, err error) error {
	if errors.Is(err, syscall.EADDRINUSE) {
		return fmt.Errorf("le port de %s est déjà pris par un autre programme (lsof -nP -iTCP:%s -sTCP:LISTEN pour le trouver) ; choisissez-en un autre avec `lorag server -p <port>` ou LORAG_PORT",
			addr, portOf(addr))
	}
	return fmt.Errorf("écoute sur %s : %w", addr, err)
}

func portOf(addr string) string {
	_, port, err := net.SplitHostPort(addr)
	if err != nil {
		return addr
	}
	return port
}
