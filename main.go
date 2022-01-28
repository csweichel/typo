package main

import (
	"io"
	"io/ioutil"
	"log"
	"math/rand"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/creack/pty"
)

func main() {
	fc, err := ioutil.ReadFile(os.Args[1])
	if err != nil {
		log.Fatal(err)
	}
	lines := strings.Split(string(fc), "\n")

	// disable input buffering
	exec.Command("stty", "-F", "/dev/tty", "cbreak", "min", "1").Run()
	// do not display entered characters on the screen
	exec.Command("stty", "-F", "/dev/tty", "-echo").Run()
	defer func() {
		// disable input buffering
		exec.Command("stty", "-F", "/dev/tty", "-cbreak").Run()
		// do not display entered characters on the screen
		exec.Command("stty", "-F", "/dev/tty", "echo").Run()
	}()

	done := make(chan struct{})
	cmd := exec.Command("bash")
	f, err := pty.Start(cmd)
	go func() {
		defer close(done)
		b := make([]byte, 1)
		for _, l := range lines {
			os.Stdin.Read(b)

			for _, c := range []byte(l) {
				if rand.Intn(100) < 10 {
					f.Write([]byte{[]byte(l)[rand.Intn(len(l))]})
					time.Sleep(time.Duration(70+rand.Intn(50)) * time.Millisecond)
					f.Write([]byte{8})
				}

				f.Write([]byte{c})
				time.Sleep(time.Duration(30+rand.Intn(50)) * time.Millisecond)
			}
			f.WriteString("\n")
		}
	}()
	go io.Copy(os.Stdout, f)
	<-done

	// hack to let the last command finish
	time.Sleep(5 * time.Second)
}
