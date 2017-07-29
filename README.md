# MiniCD

Package minicd exposes an `http.HandlerFunc` that you can include in your Go web server so it can continuously deliver itself whenever you push to Github. 

This is meant for small personal projects and I wouldn't recommend it for large production apps. For larger scale CI/CD check out [baghdad](https://www.github.com/marwan-at-work/baghdad).

## Usage

```go
package main

import (
    "fmt"
    "net/http"

    "github.com/marwan-at-work/minicd"
)

func main() {
    killSig := make(chan struct{})

    http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
        fmt.Fprintln(w, "hello world")
    })

    http.HandleFunc("/github-hook", minicd.Handler(minicd.Config{
        WebhookSecret: "my_webhook_secret",
        GithubToken: "optional_if_public_repo",
        KillSig: killSig,
    }))

    srv := &http.Server{Addr: ":3000"}

    go func() {
        if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
            log.Fatal(err)
        }
    }()

    <-killSig
    ctx, cancel := context.WithTimeout(context.Background(), time.Second*3)
    defer cancel()

    err := srv.Shutdown(ctx)
    if err != nil {
        log.Fatal(err)
    }

    fmt.Println("see ya on the other updated side")
}

```