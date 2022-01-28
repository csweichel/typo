package main

import (
	"io"
	"io/ioutil"
	"log"
	"math/rand"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/creack/pty"
	"github.com/fatih/color"
)

func main() {
	if len(os.Args) < 2 {
		log.Fatalf("usage: %s <script.txt>", os.Args[0])
	}
	fc, err := ioutil.ReadFile(os.Args[1])
	if err != nil {
		log.Fatal(err)
	}
	lines := strings.Split(string(fc), "\n")

	var (
		cmds []Command
		lc   int
	)
	for _, line := range lines {
		lc++
		line = strings.TrimSpace(line)
		if len(line) == 0 {
			continue
		}
		segs := strings.Split(line, " ")
		switch segs[0] {
		case "RUN":
			cmds = append(cmds, RunCommand{
				wait: true,
				cmd:  strings.TrimPrefix(line, "RUN "),
			})
		case "RUNI":
			cmds = append(cmds, RunCommand{
				wait: false,
				cmd:  strings.TrimPrefix(line, "RUNI "),
			})
		case "ECHO":
			cmds = append(cmds, EchoCommand(strings.TrimPrefix(line, "ECHO ")))
		case "CLEAR":
			cmds = append(cmds, ClearCommand{})
		case "TERM":
			cmds = append(cmds, TermCommand(strings.TrimPrefix(line, "TERM ")))
		case "SLEEP":
			dur, err := time.ParseDuration(strings.TrimPrefix(line, "SLEEP "))
			if err != nil {
				log.Fatalf("line %d: %v", lc-1, err)
			}
			cmds = append(cmds, SleepCommand(dur))
		default:
			log.Fatalf("unknown command in line %d: %s", lc-1, line)
		}
	}

	enterRawMode()
	defer leaveRawMode()

	done := make(chan struct{})
	cmd := exec.Command("bash")
	ptmx, err := pty.Start(cmd)
	// Make sure to close the pty at the end.
	defer func() { _ = ptmx.Close() }() // Best effort.

	// Handle pty size.
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGWINCH)
	go func() {
		for range ch {
			if err := pty.InheritSize(os.Stdin, ptmx); err != nil {
				log.Printf("error resizing pty: %s", err)
			}
		}
	}()
	ch <- syscall.SIGWINCH                        // Initial resize.
	defer func() { signal.Stop(ch); close(ch) }() // Cleanup signals when done.

	go func() {
		defer close(done)
		b := make([]byte, 1)

		for i, c := range cmds {
			if i > 0 && cmds[i-1].Wait() {
				os.Stdin.Read(b)
			}

			c.Execute(ptmx)
		}
	}()
	go io.Copy(os.Stdout, ptmx)
	<-done

	// hack to let the last command finish
	time.Sleep(5 * time.Second)
}

func enterRawMode() {
	// disable input buffering
	exec.Command("stty", "-F", "/dev/tty", "cbreak", "min", "1").Run()
	// do not display entered characters on the screen
	exec.Command("stty", "-F", "/dev/tty", "-echo").Run()
}

func leaveRawMode() {
	// disable input buffering
	exec.Command("stty", "-F", "/dev/tty", "-cbreak").Run()
	// do not display entered characters on the screen
	exec.Command("stty", "-F", "/dev/tty", "echo").Run()
}

type Command interface {
	Execute(term *os.File) error
	Wait() bool
}

type RunCommand struct {
	wait bool
	cmd  string
}

func (c RunCommand) Execute(f *os.File) error {
	l := c.cmd
	for _, c := range []byte(l) {
		if rand.Intn(100) < 10 {
			_, err := f.Write([]byte{[]byte(l)[rand.Intn(len(l))]})
			if err != nil {
				return err
			}
			time.Sleep(time.Duration(70+rand.Intn(50)) * time.Millisecond)
			_, err = f.Write([]byte{8})
			if err != nil {
				return err
			}
		}

		_, err := f.Write([]byte{c})
		if err != nil {
			return err
		}
		time.Sleep(time.Duration(30+rand.Intn(50)) * time.Millisecond)
	}
	time.Sleep(time.Duration(30+rand.Intn(50)) * time.Millisecond)
	_, err := f.WriteString("\n")
	if err != nil {
		return err
	}
	return nil
}
func (c RunCommand) Wait() bool { return c.wait }

type EchoCommand string

func (c EchoCommand) Execute(f *os.File) error {
	color.HiGreen("\n" + string(c) + "\n")
	f.WriteString("\n")
	return nil
}

func (c EchoCommand) Wait() bool { return false }

type ClearCommand struct{}

func (c ClearCommand) Execute(f *os.File) error {
	f.WriteString("\nclear\n")
	return nil
}

func (c ClearCommand) Wait() bool { return true }

type TermCommand string

func (c TermCommand) Execute(f *os.File) error {
	leaveRawMode()
	defer enterRawMode()

	cmd := exec.Command(string(c))
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	err := cmd.Run()

	f.WriteString("\nclear\n")

	return err
}

func (c TermCommand) Wait() bool { return true }

type SleepCommand time.Duration

func (c SleepCommand) Execute(f *os.File) error {
	time.Sleep(time.Duration(c))
	return nil
}

func (c SleepCommand) Wait() bool { return true }
