// +build linux

package main

import (

	"errors"
	"fmt"

	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"crypto/sha1"
	"syscall"
	"strings"

	"github.com/Sirupsen/logrus"
	"github.com/coreos/go-systemd/activation"
	"github.com/opencontainers/runc/libcontainer"
	"github.com/opencontainers/runc/libcontainer/cgroups/systemd"
	"github.com/opencontainers/runc/libcontainer/configs"
	"github.com/opencontainers/runc/hypervisor"
	"github.com/opencontainers/runc/libcontainer/specconv"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/urfave/cli"
	"io"
	"encoding/hex"
)

var errEmptyID = errors.New("container id cannot be empty")

var container libcontainer.Container

// loadFactory returns the configured factory instance for execing containers.
func loadFactory(context *cli.Context) (libcontainer.Factory, error) {
	root := context.GlobalString("root")
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	cgroupManager := libcontainer.Cgroupfs
	if context.GlobalBool("systemd-cgroup") {
		if systemd.UseSystemd() {
			cgroupManager = libcontainer.SystemdCgroups
		} else {
			return nil, fmt.Errorf("systemd cgroup flag passed, but systemd support for managing cgroups is not available")
		}
	}
	return libcontainer.New(abs, cgroupManager, libcontainer.CriuPath(context.GlobalString("criu")))
}

// getContainer returns the specified container instance by loading it from state
// with the default factory.
func getContainer(context *cli.Context) (libcontainer.Container, error) {
	id := context.Args().First()
	if id == "" {
		return nil, errEmptyID
	}
	factory, err := loadFactory(context)
	if err != nil {
		return nil, err
	}
	return factory.Load(id)
}

func fatalf(t string, v ...interface{}) {
	fatal(fmt.Errorf(t, v...))
}

func getDefaultImagePath(context *cli.Context) string {
	cwd, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	return filepath.Join(cwd, "checkpoint")
}

// newProcess returns a new libcontainer Process with the arguments from the
// spec and stdio from the current process.
func newProcess(p specs.Process) (*libcontainer.Process, error) {
	// ISOLATED
	p.Args = []string{"sh"}
	lp := &libcontainer.Process{
		Args: p.Args,
		Env:  p.Env,
		// TODO: fix libcontainer's API to better support uid/gid in a typesafe way.
		User:            fmt.Sprintf("%d:%d", p.User.UID, p.User.GID),
		Cwd:             p.Cwd,
		Capabilities:    p.Capabilities,
		Label:           p.SelinuxLabel,
		NoNewPrivileges: &p.NoNewPrivileges,
		AppArmorProfile: p.ApparmorProfile,
	}
	for _, gid := range p.User.AdditionalGids {
		lp.AdditionalGroups = append(lp.AdditionalGroups, strconv.FormatUint(uint64(gid), 10))
	}
	for _, rlimit := range p.Rlimits {
		rl, err := createLibContainerRlimit(rlimit)
		if err != nil {
			return nil, err
		}
		lp.Rlimits = append(lp.Rlimits, rl)
	}
	return lp, nil
}

func dupStdio(process *libcontainer.Process, rootuid, rootgid int) error {
	process.Stdin = os.Stdin
	process.Stdout = os.Stdout
	process.Stderr = os.Stderr
	for _, fd := range []uintptr{
		os.Stdin.Fd(),
		os.Stdout.Fd(),
		os.Stderr.Fd(),
	} {
		if err := syscall.Fchown(int(fd), rootuid, rootgid); err != nil {
			return err
		}
	}
	return nil
}

// If systemd is supporting sd_notify protocol, this function will add support
// for sd_notify protocol from within the container.
func setupSdNotify(spec *specs.Spec, notifySocket string) {
	spec.Mounts = append(spec.Mounts, specs.Mount{Destination: notifySocket, Type: "bind", Source: notifySocket, Options: []string{"bind"}})
	spec.Process.Env = append(spec.Process.Env, fmt.Sprintf("NOTIFY_SOCKET=%s", notifySocket))
}

func destroy(container libcontainer.Container) {
	if err := container.Destroy(); err != nil {
		logrus.Error(err)
	}
}

// setupIO sets the proper IO on the process depending on the configuration
// If there is a nil error then there must be a non nil tty returned
func setupIO(process *libcontainer.Process, rootuid, rootgid int, console string, createTTY, detach bool) (*tty, error) {
	// detach and createTty will not work unless a console path is passed
	// so error out here before changing any terminal settings
	if createTTY && detach && console == "" {
		return nil, fmt.Errorf("cannot allocate tty if runc will detach")
	}
	if createTTY {
		return createTty(process, rootuid, rootgid, console)
	}
	if detach {
		if err := dupStdio(process, rootuid, rootgid); err != nil {
			return nil, err
		}
		return &tty{}, nil
	}
	return createStdioPipes(process, rootuid, rootgid)
}

// createPidFile creates a file with the processes pid inside it atomically
// it creates a temp file with the paths filename + '.' infront of it
// then renames the file
func createPidFile(path string, process *libcontainer.Process) error {
	pid, err := process.Pid()
	if err != nil {
		return err
	}
	var (
		tmpDir  = filepath.Dir(path)
		tmpName = filepath.Join(tmpDir, fmt.Sprintf(".%s", filepath.Base(path)))
	)
	f, err := os.OpenFile(tmpName, os.O_RDWR|os.O_CREATE|os.O_EXCL|os.O_SYNC, 0666)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(f, "%d", pid)
	f.Close()
	if err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}

func createContainer(context *cli.Context, id string, spec *specs.Spec) (libcontainer.Container, error) {
	config, err := specconv.CreateLibcontainerConfig(&specconv.CreateOpts{
		CgroupName:       id,
		UseSystemdCgroup: context.GlobalBool("systemd-cgroup"),
		NoPivotRoot:      context.Bool("no-pivot"),
		NoNewKeyring:     context.Bool("no-new-keyring"),
		Spec:             spec,
	})
	if err != nil {
		return nil, err
	}
	factory, err := loadFactory(context)
	if err != nil {
		return nil, err
	}
	return factory.Create(id, config)
}

type runner struct {
	enableSubreaper bool
	shouldDestroy   bool
	detach          bool
	listenFDs       []*os.File
	pidFile         string
	console         string
	container       libcontainer.Container
	create          bool
}


func (r *runner) run(config *specs.Process) (int, error) {
	process, err := newProcess(*config)

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	go func(){
		for sig := range c {
			//ISOLATED
			fmt.Println(sig)
			hyperVisor, err := hypervisor.HypFactory()
			virtualMachine, err := hyperVisor.GetVM(r.container.ID())
			if err == nil {
				virtualMachine.Kill()
			}
		}
	}()

	if err != nil {
		r.destroy()
		return -1, err
	}
	if len(r.listenFDs) > 0 {
		process.Env = append(process.Env, fmt.Sprintf("LISTEN_FDS=%d", len(r.listenFDs)), "LISTEN_PID=1")
		process.ExtraFiles = append(process.ExtraFiles, r.listenFDs...)
	}

	startFn := r.container.Start
	if !r.create {
		startFn = r.container.Run
	}

	if err := startFn(process); err != nil {
		r.destroy()
		return -1, err
	}

	if r.pidFile != "" {
		if err := createPidFile(r.pidFile, process); err != nil {
			r.terminate(process)
			r.destroy()
			return -1, err
		}
	}

	containerState, err := r.container.State()
	if err != nil {
		r.destroy()
		return -1, err
	}

	// Isolated containers - launch VM
	//
	//err = verifyImage()
	//if err != nil {
	//	r.destroy()
	//	return -1, err
	//}


	hyperVisor, err := hypervisor.HypFactory()

	vmParams := new(hypervisor.VirtualMachineParams)
	vmParams.Id = r.container.ID()
        vmParams.Detach = r.detach
	vmParams.Args = config.Args
	vmParams.EnvPath(config.Env)
	vmParams.CwD = config.Cwd

	vmParams.NetworkNSPath = containerState.NamespacePaths[configs.NEWNET]
	vmParams.NetworkInfo()
	if err != nil {
		r.destroy()
		return -1, err
	}

	vmParams.Rootfs = r.container.Config().Rootfs

	mountPoints := make(map[string]string)
	for _, mount := range r.container.Config().Mounts{
		if strings.HasPrefix(mount.Source, "/") && len(mount.PropagationFlags) == 0 {
			mountPoints[mount.Source] = mount.Destination
		}
	}

	vmParams.Mounts = mountPoints

	_, err = hyperVisor.CreateVM(*vmParams)
	if err != nil {
		r.destroy()
		return -1, err
	}

	if r.detach || r.create {
		return 0, nil
	}

	r.destroy()
	return 0, err
}

func verifyImage() error {
	diskFile, err := os.Open(hypervisor.OriginalDiskPath)
	if err != nil {
		return err
	}
	defer diskFile.Close()

	h := sha1.New()
	if _, err := io.Copy(h, diskFile); err != nil {
		return err
	}

	hashInBytes := h.Sum(nil)[:20]
	sumString := hex.EncodeToString(hashInBytes)
	config, err := hypervisor.ParseConfig()
	if err != nil {
		return err
	}

	parsedSha1Sum := config.Sha1Sum

	if sumString != parsedSha1Sum {
		return errors.New("SHA1 sum is not valid.")
	}

	return nil
}

func (r *runner) destroy() {
	if r.shouldDestroy {
		destroy(r.container)
	}
}

func (r *runner) terminate(p *libcontainer.Process) {
	p.Signal(syscall.SIGKILL)
	p.Wait()
}

func validateProcessSpec(spec *specs.Process) error {
	if spec.Cwd == "" {
		return fmt.Errorf("Cwd property must not be empty")
	}
	if !filepath.IsAbs(spec.Cwd) {
		return fmt.Errorf("Cwd must be an absolute path")
	}
	if len(spec.Args) == 0 {
		return fmt.Errorf("args must not be empty")
	}
	return nil
}

func startContainer(context *cli.Context, spec *specs.Spec, create bool) (int, error) {
	id := context.Args().First()
	if id == "" {
		return -1, errEmptyID
	}
	container, err := createContainer(context, id, spec)
	if err != nil {
		return -1, err
	}
	// Support on-demand socket activation by passing file descriptors into the container init process.
	listenFDs := []*os.File{}
	if os.Getenv("LISTEN_FDS") != "" {
		listenFDs = activation.Files(false)
	}
	r := &runner{
		enableSubreaper: !context.Bool("no-subreaper"),
		shouldDestroy:   true,
		container:       container,
		listenFDs:       listenFDs,
		console:         context.String("console"),
		detach:          context.Bool("detach"),
		pidFile:         context.String("pid-file"),
		create:          create,
	}
	return r.run(&spec.Process)
}
