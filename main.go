package main

import (
	"errors"
	"fmt"
	"github.com/rancher/rancher/pkg/kontainer-engine/types"
	"github.com/waldur/waldur-rancher-cluster-driver/driver"
	"os"
	"strconv"
	"sync"
)

var wg = &sync.WaitGroup{}

func main() {
	if os.Args[1] == "" {
		panic(errors.New("No port provided"))
	}

	port, err := strconv.Atoi(os.Args[1])
	if err != nil {
		panic(fmt.Errorf("Argument is not parsable as int: %v", err))
	}

	addr := make(chan string)
	d := driver.Driver{}
	// TODO: change host
	go types.NewServer(&d, addr).ServeOrDie(fmt.Sprintf("127.0.0.1:%v", port))

	wg.Add(1)
	wg.Wait() // wait forever, we only exit if killed by parent process
}
