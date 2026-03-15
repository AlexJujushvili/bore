# bore (modified)

დაფუძნებულია [jkuri/bore](https://github.com/jkuri/bore) პროექტზე — Reverse HTTP/TCP proxy SSH tunnel-ით.

ორიგინალ პაკეტს დაემატა **`Listen()` მეთოდი**, რომელიც [ngrok-ის Go SDK](https://github.com/ngrok/ngrok-go)-ს სტილს მიბაძავს.

---

## რა შეიცვალა

ორიგინალი `bore` კლიენტი მუშაობდა ასე:

```
browser → SSH tunnel → bore.digital → net.Dial("tcp", "localhost:PORT") → შენი სერვისი
```

ანუ **სავალდებულო იყო** ლოკალური TCP პორტი გახსნილიყო.

ახალი `Listen()` მეთოდი **`net.Pipe()`-ს** იყენებს — in-memory კავშირი, რომელსაც TCP სტეკი საერთოდ არ სჭირდება:

```
browser → SSH tunnel → bore.digital → net.Pipe() → http.Serve(ln, mux)
```

პორტი არ იხსნება. კავშირი მეხსიერებაში ხდება.

---

## ძირითადი ცვლილებები

### 1. `Listen(ctx context.Context) (net.Listener, error)`

ngrok-ის სტილის API — აბრუნებს `net.Listener`-ს, რომელზეც პირდაპირ `http.Serve` გამოიძახება:

```go
ln, err := bc.Listen(ctx)
http.Serve(ln, mux)
```

### 2. `NewBoreClient` — pointer და error

```go
// ადრე
bc := client.NewBoreClient(cfg)

// ახლა
bc, err := NewBoreClient(cfg)
```

### 3. `context.Context` — graceful shutdown

`Listen(ctx)` იღებს context-ს. `ctx` cancel-ზე SSH listener და in-memory listener ავტომატურად იხურება.

### 4. `sync.Once` — double-close პანიკი აღარ

```go
func (l *boreListener) Close() error {
    l.once.Do(func() { close(l.closed) })
    return nil
}
```

---

## გამოყენება

```go
package main

import (
    "context"
    "log"
    "net/http"
    "os/signal"
    "syscall"
)

func main() {
    ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
    defer stop()

    cfg := Config{
        RemoteServer: "bore.digital",
        RemotePort:   2200,
        ID:           "myapp", // → https://myapp.bore.digital
        KeepAlive:    true,
    }

    bc, err := NewBoreClient(cfg)
    if err != nil {
        log.Fatal(err)
    }

    ln, err := bc.Listen(ctx)
    if err != nil {
        log.Fatal(err)
    }
    defer ln.Close()

    mux := http.NewServeMux()
    mux.HandleFunc("/", homeHandler)

    http.Serve(ln, mux)
}
```

---

## ngrok-თან შედარება

```go
// ngrok
ln, err := agent.Listen(ctx, config)
http.Serve(ln, mux)

// bore (modified)
ln, err := bc.Listen(ctx)
http.Serve(ln, mux)
```

სტილი იდენტურია — განსხვავება მხოლოდ კონფიგურაციაშია.

---

## ორიგინალი API — უცვლელია

`Run()` მეთოდი ამოღებულია ამ ვერსიაში. თუ localhost TCP გადამისამართება გჭირდება, გამოიყენე ორიგინალი [jkuri/bore](https://github.com/jkuri/bore).

---

## ლიცენზია

MIT — იხილე ორიგინალი [jkuri/bore](https://github.com/jkuri/bore/blob/main/LICENSE)
