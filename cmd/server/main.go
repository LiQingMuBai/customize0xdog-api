package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"customize-teldog-api/internal/config"
	"customize-teldog-api/internal/server"
)

var buddhaASCII = strings.Join([]string{
	"                       _oo0oo_",
	"                      o8888888o",
	"                      88\" . \"88",
	"                      (| -_- |)",
	"                      0\\  =  /0",
	"                    ___/`---'\\___",
	"                  .' \\\\|     |// '.",
	"                 / \\\\|||  :  |||// \\",
	"                / _||||| -:- |||||- \\",
	"               |   | \\\\\\  -  /// |   |",
	"               | \\_|  ''\\---/''  |_/ |",
	"               \\  .-\\__  '-'  ___/-. /",
	"             ___'. .'  /--.--\\  `. .'___",
	"          .\"\" '<  `.___\\_<|>_/___.' >' \"\".",
	"         | | :  `- \\`.;`\\ _ /`;.`/ - `  : | |",
	"         \\  \\ `_.   \\_ __\\ /__ _/   .-` /  /",
	"     =====`-.____`.___ \\_____/___.`____.-'=====",
	"                       `=---='",
	"",
	"                 佛祖保佑   永无 BUG",
	"",
}, "\n")

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config error: %v", err)
	}

	srv, err := server.New(cfg)
	if err != nil {
		log.Fatalf("server init error: %v", err)
	}

	httpSrv := &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		log.Print(buddhaASCII)
		log.Printf("佛祖保佑, listening on %s", cfg.ListenAddr)
		errCh <- httpSrv.ListenAndServe()
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-sigCh:
		log.Printf("shutdown signal=%s", sig.String())
	case err := <-errCh:
		if err != nil && err != http.ErrServerClosed {
			log.Printf("http server stopped: %v", err)
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = httpSrv.Shutdown(ctx)
}
