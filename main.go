package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os/signal"
	"syscall"
)

const (
	boreServer = "bore.digital"
	borePort   = 2200
)

func main() {
	// signal.NotifyContext — graceful shutdown ctx-ით
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := run_bore_digital(ctx); err != nil {
		log.Fatal(err)
	}
	log.Println("გამოირთო.")
}

func run_bore_digital(ctx context.Context) error {
	cfg := Config{
		RemoteServer: boreServer,
		RemotePort:   borePort,
		ID:           "alex751",
		KeepAlive:    true,
	}

	bc, err := NewBoreClient(cfg)
	if err != nil {
		return err
	}

	ln, err := bc.Listen(ctx)
	if err != nil {
		return err
	}
	defer ln.Close()

	log.Println("bore tunnel გაიხსნა!")

	mux := http.NewServeMux()
	mux.HandleFunc("/", homeHandler)

	return http.Serve(ln, mux)
}

func homeHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprintf(w, `<!DOCTYPE html>
<html>
<head><title>bore</title></head>
<body>
	<h1>გამარჯობა bore-დან! 👋</h1>
	<p>localhost პორტი არ გახსნილა.</p>
</body>
</html>`)
}
