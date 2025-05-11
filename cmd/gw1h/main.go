package main

import (
	"bufio"
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"sync"
	"time"

	"github.com/ericmitchem/gw1h/gw1h"
)

const (
	gw1hVersion = "0.1.0"
)

func StartGW(ctx context.Context) (*exec.Cmd, error) {
	gwExe := `C:\Program Files (x86)\Guild Wars\Gw.exe`

	cmd := exec.CommandContext(ctx, "wine", gwExe)
	cmd.Env = gw1h.WineEnv()

	// TODO: prefix output with [gw]
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		slog.Error("error starting gw", slog.String("err", err.Error()))
		return nil, err
	}
	slog.Debug("gw started", slog.Int("pid", cmd.Process.Pid))
	return cmd, nil
}

func StartToolbox(ctx context.Context, gwPid int) (*exec.Cmd, error) {
	toolboxExe := `C:\Program Files (x86)\GWToolbox\GWToolbox.exe`

	cmd := exec.CommandContext(ctx, "wine", toolboxExe, "/pid", strconv.Itoa(gwPid))
	cmd.Env = gw1h.WineEnv()

	// TODO: prefix output with [toolbox]
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err := cmd.Start()
	if err != nil {
		slog.Error("error starting toolbox", slog.String("err", err.Error()))
		return nil, err
	}
	slog.Debug("toolbox started", slog.Int("pid", cmd.Process.Pid))
	return cmd, nil
}

func main() {
	slog.SetDefault(gw1h.NewLogger())

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	go func() {
		<-ctx.Done()
		signal.Reset(os.Interrupt)
	}()

	_, startGw := os.LookupEnv("GW1H_GW")
	_, startToolbox := os.LookupEnv("GW1H_TOOLBOX")

	if !startGw && !startToolbox {
		slog.Error("Set 'GW1H_GW' or 'GW1H_TOOLBOX' to start")
		return
	}

	wg := sync.WaitGroup{}
	pidChan := make(chan int, 1)

	slog.Info("gw1h started", slog.String("version", gw1hVersion))
	defer slog.Info("gw1h stopped")

	if startGw {
		wg.Add(1)
		go func() {
			defer wg.Done()
			gw, err := StartGW(ctx)
			if err != nil {
				slog.Error("error starting gw", slog.String("err", err.Error()))
				cancel()
				return
			}

			// Give gw time to initialize
			time.Sleep(3 * time.Second)

			// TODO: Pass unique input to each gw instance and grep for the correct one
			// TODO: ellide pid work if we're not starting toolbox

			// List wine processes
			// Filter for guild wars
			// Extract the pid and convert to decimal
			cmd := exec.CommandContext(ctx, "bash", "-c",
				`winedbg --command "info process" | grep Gw.exe | awk '{print "ibase=16;" $1}' | bc`)
			cmd.Env = gw1h.WineEnv()

			out, err := cmd.StdoutPipe()
			if err != nil {
				slog.Error("error getting stdout pipe", slog.String("err", err.Error()))
				cancel()
				return
			}

			err = cmd.Start()
			if err != nil {
				slog.Error("error starting cmd", slog.String("err", err.Error()))
				cancel()
				return
			}

			go func() {
				var pids []int
				scanner := bufio.NewScanner(out)
				for scanner.Scan() {
					pid, err := strconv.Atoi(scanner.Text())
					if err != nil {
						slog.Error("error parsing pid", slog.String("err", err.Error()))
						cancel()
						return
					}
					slog.Debug("detected gw pid", slog.Int("pid", pid))
					pids = append(pids, pid)
				}

				if err := scanner.Err(); err != nil {
					slog.Error("error reading cmd output", slog.String("err", err.Error()))
					cancel()
					return
				}

				switch {
				case len(pids) < 1:
					slog.Error("no gw pids found")
					cancel()
					return
				case len(pids) > 1:
					slog.Error("multiple gw pids found", slog.String("pids", fmt.Sprintf("%v", pids)))
					cancel()
					return
				default:
					pidChan <- pids[0]
					slog.Debug(fmt.Sprintf("pidChan <- %d", pids[0]))
				}
			}()

			err = cmd.Wait()
			if err != nil {
				slog.Error("error waiting for cmd", slog.String("err", err.Error()))
				cancel()
				return
			}

			err = gw.Wait()
			if err != nil {
				slog.Error("error waiting for gw", slog.String("err", err.Error()))
				return
			}
			slog.Debug("gw exited", slog.Int("pid", gw.Process.Pid))
		}()
	}

	if startToolbox {
		wg.Add(1)
		go func() {
			defer wg.Done()

			if !startGw {
				gwPidStr, ok := os.LookupEnv("GW1H_GW_PID")
				if !ok {
					slog.Error("GW1H_GW_PID not set")
					return
				}

				pid, err := strconv.Atoi(gwPidStr)
				if err != nil {
					slog.Error("error parsing gw pid", slog.String("err", err.Error()))
					return
				}
				pidChan <- pid
			}

			var gwPid int
			select {
			case gwPid = <-pidChan:
				slog.Debug("received gw pid", slog.Int("pid", gwPid))
			case <-ctx.Done():
				return
			case <-time.After(10 * time.Second):
				slog.Error("timeout waiting for gw pid")
				return
			}

			toolbox, err := StartToolbox(ctx, gwPid)
			if err != nil {
				slog.Error("error starting toolbox", slog.String("err", err.Error()))
				cancel()
				return
			}

			err = toolbox.Wait()
			if err != nil {
				slog.Error("error waiting for toolbox", slog.String("err", err.Error()))
				return
			}
			slog.Debug("toolbox exited", slog.Int("pid", toolbox.Process.Pid))
		}()
	}

	wg.Wait()
}
