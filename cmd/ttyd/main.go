package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/user"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/WeidiDeng/ttyd-go"
)

var (
	address       = flag.String("addr", "127.0.0.1:7681", "address to listen on. use port 0 to select a random port")
	socketAddress = flag.String("socket", "", "unix socket to listen on. this takes precedence over -addr")
	basicAuth     = flag.String("basic", "", "basic auth credential (user:password)")
	writable      = flag.Bool("writable", false, "enable writable mode")
	compress      = flag.Bool("compress", false, "enable compression")
	cert          = flag.String("cert", "", "path to the tls certificate file")
	key           = flag.String("key", "", "path to the tls key file")
	uid           = flag.Int("uid", 0, "run as user id")
	gid           = flag.Int("gid", 0, "run as group id")
	cwd           = flag.String("cwd", "", "current working directory for the process. calling process's cwd is used if not provided")
)

func customError(msg string) {
	_, _ = fmt.Fprintln(flag.CommandLine.Output(), msg)
	flag.Usage()
	os.Exit(2)
}

func init() {
	flag.Usage = func() {
		_, _ = fmt.Fprintf(flag.CommandLine.Output(), "Usage of %s:\n", os.Args[0])
		flag.PrintDefaults()
		_, _ = fmt.Fprintln(flag.CommandLine.Output(), "arguments without flag are treated as the command to run")
		_, _ = fmt.Fprintf(flag.CommandLine.Output(), "Example: %s /bin/bash -c 'echo hello world'\n", os.Args[0])
	}
	flag.Parse()
	if len(flag.Args()) == 0 {
		customError("no command specified")
	}
	if *basicAuth != "" && strings.Count(*basicAuth, ":") != 1 {
		customError("invalid basic auth. format user:password")
	}
	if *cert == "" && *key != "" || *cert != "" && *key == "" {
		customError("both cert and key must be provided")
	}
}

func main() {
	var (
		uidSet bool
		gidSet bool

		trueUid int
		trueGid int
	)
	flag.Visit(func(f *flag.Flag) {
		switch f.Name {
		case "uid":
			uidSet = true
		case "gid":
			gidSet = true
		}
	})
	if runtime.GOOS != "windows" && (uidSet || gidSet) {
		u, err := user.Current()
		if err != nil {
			log.Fatalln("failed to get current user:", err)
		}
		userUid, err := strconv.Atoi(u.Uid)
		if err != nil {
			log.Fatalln("failed to parse current user id:", err)
		}
		userGid, err := strconv.Atoi(u.Gid)
		if err != nil {
			log.Fatalln("failed to parse current group id:", err)
		}

		if uidSet {
			trueUid = *uid
		} else {
			trueUid = userUid
		}

		if gidSet {
			trueGid = *gid
		} else {
			trueGid = userGid
		}
	}

	cmdFunc := func() *exec.Cmd {
		cmd := exec.Command(flag.Args()[0], flag.Args()[1:]...)
		if runtime.GOOS != "windows" && (uidSet || gidSet) {
			setCredential(cmd, trueUid, trueGid)
		}
		cmd.Dir = *cwd
		return cmd
	}
	now := time.Now()
	var authFunc func(w http.ResponseWriter, r *http.Request) bool
	if *basicAuth != "" {
		user, pass, _ := strings.Cut(*basicAuth, ":")
		authFunc = func(w http.ResponseWriter, r *http.Request) bool {
			u, p, ok := r.BasicAuth()
			if !ok || u != user || p != pass {
				w.Header().Set("WWW-Authenticate", `Basic realm="ttyd"`)
				http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
				return false
			}
			return true
		}
	} else {
		authFunc = func(w http.ResponseWriter, r *http.Request) bool {
			return true
		}
	}
	http.HandleFunc("/", func(writer http.ResponseWriter, request *http.Request) {
		if !authFunc(writer, request) {
			return
		}

		http.ServeContent(writer, request, "index.html", now, strings.NewReader(ttyd.DefaultHTML))
	})
	http.HandleFunc("/token", func(writer http.ResponseWriter, request *http.Request) {
		if !authFunc(writer, request) {
			return
		}

		ttyd.DefaultTokenHandlerFunc(writer, request)
	})

	http.HandleFunc("/ws", func(writer http.ResponseWriter, request *http.Request) {
		if !authFunc(writer, request) {
			return
		}

		var handlerOptions []ttyd.HandlerOption
		if *writable {
			handlerOptions = append(handlerOptions, ttyd.EnableClientInput())
		}
		if *compress {
			handlerOptions = append(handlerOptions, ttyd.EnableCompressionWithContextTakeover())
		}
		ttyd.NewHandler(cmdFunc(), handlerOptions...).ServeHTTP(writer, request)
	})
	var (
		network   string
		addr      string
		printAddr bool
	)
	if *socketAddress != "" {
		network = "unix"
		addr = *socketAddress
		_ = os.Remove(addr)
	} else {
		network = "tcp"
		addr = *address
		if strings.HasSuffix(addr, ":0") {
			printAddr = true
		}
	}
	l, err := net.Listen(network, addr)
	if err != nil {
		log.Fatalln("failed to listen:")
	}

	if printAddr {
		log.Println("listening on", l.Addr().String())
	}
	if *cert != "" {
		log.Fatal(http.ServeTLS(l, nil, *cert, *key))
	}
	log.Fatal(http.Serve(l, nil))
}
