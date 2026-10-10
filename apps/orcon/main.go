package main

import (
 "embed"
 "flag"
 "fmt"
 "io/fs"
 "log"
 "net/http"
 "os"
 "strings"

 "github.com/wirvii/orcon/packages/go/config"
)

//go:embed ui/dist
var embeddedUI embed.FS

func main() {
 role := flag.String("role", "standalone", "standalone|api|scheduler|worker")
 addr := flag.String("addr", ":8080", "HTTP listen address")
 flag.Parse()
 selected := config.Role(strings.ToLower(*role))
 if !selected.Valid() { fmt.Fprintln(os.Stderr, "invalid role:", *role); os.Exit(2) }
 switch selected {
 case config.RoleStandalone, config.RoleAPI:
  assets, err := fs.Sub(embeddedUI, "ui/dist"); if err != nil { log.Fatal(err) }
  mux := http.NewServeMux()
  mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK); _, _ = w.Write([]byte("ok")) })
  mux.Handle("/", http.FileServer(http.FS(assets)))
  log.Printf("orcon role=%s listening=%s", selected, *addr)
  log.Fatal(http.ListenAndServe(*addr, mux))
 default:
  log.Printf("orcon role=%s scaffold: processing not implemented", selected)
 }
}
