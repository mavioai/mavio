package containers

import (
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"
)

// Commands run a dependency outside the server, under the name and
// volumes a managed one would have.
type Commands struct {
	// DockerRun is a docker run command; it joins the network
	// "<network>" when the server is not on the host.
	DockerRun string
	// Compose is a Compose file of the service.
	Compose string
	// Address is where the server reaches it once it runs.
	Address string
}

// RunCommands returns the commands running a dependency: with its port
// published on 127.0.0.1 for a server on the host, else reached by name
// on a network the server's container joins.
func RunCommands(n Names, d Dependency, onHost bool) Commands {
	name := n.Container(d.Plugin, d.Name)
	port := strconv.Itoa(d.Port)
	volumes := make([]string, 0, len(d.Volumes))
	for _, v := range slices.Sorted(maps.Keys(d.Volumes)) {
		volumes = append(volumes, n.Volume(d.Plugin, d.Name, v)+":"+d.Volumes[v])
	}
	var binds []string
	for _, f := range slices.Sorted(maps.Keys(d.Folders)) {
		binds = append(binds, f+":"+d.Folders[f])
	}
	env := slices.Sorted(maps.Keys(d.Env))

	args := []string{"docker", "run", "-d", "--name", name, "--restart", "unless-stopped"}
	if onHost {
		args = append(args, "-p", "127.0.0.1:"+port+":"+port)
	} else {
		args = append(args, "--network", "<network>")
	}
	peer := peerPorts(d.PeerPort)
	for _, p := range peer {
		args = append(args, "-p", p)
	}
	args = append(args, d.GPU.flags()...)
	if d.HostAccess {
		args = append(args, "--add-host", hostGateway)
	}
	if d.Memory > 0 {
		args = append(args, "--memory", strconv.FormatInt(d.Memory>>20, 10)+"m")
	}
	if d.CPUs > 0 {
		args = append(args, "--cpus", strconv.FormatFloat(d.CPUs, 'f', -1, 64))
	}
	for _, v := range slices.Concat(volumes, binds) {
		args = append(args, "-v", v)
	}
	for _, k := range env {
		args = append(args, "-e", k+"="+d.Env[k])
	}
	args = append(args, entrypointFlags(d.Entrypoint)...)
	args = append(args, d.Image)
	args = append(args, entrypointArgs(d.Entrypoint, d.Cmd)...)
	quoted := make([]string, len(args))
	for i, a := range args {
		quoted[i] = shellQuote(a)
	}

	// Compose files are YAML, of which JSON strings are a part.
	var b strings.Builder
	fmt.Fprintf(&b, "services:\n  %s:\n    image: %q\n    container_name: %q\n    restart: unless-stopped\n", name, d.Image, name)
	if len(d.Entrypoint) > 0 {
		fmt.Fprintf(&b, "    entrypoint: [%s]\n", joinQuoted(d.Entrypoint))
	}
	if len(d.Cmd) > 0 {
		fmt.Fprintf(&b, "    command: [%s]\n", joinQuoted(d.Cmd))
	}
	ports := peer
	if onHost {
		ports = append([]string{"127.0.0.1:" + port + ":" + port}, peer...)
	}
	if len(ports) > 0 {
		fmt.Fprintf(&b, "    ports: [%s]\n", joinQuoted(ports))
	}
	if d.HostAccess {
		fmt.Fprintf(&b, "    extra_hosts: [%q]\n", hostGateway)
	}
	if len(env) > 0 {
		b.WriteString("    environment:\n")
		for _, k := range env {
			fmt.Fprintf(&b, "      %s: %q\n", k, d.Env[k])
		}
	}
	if len(volumes)+len(binds) > 0 {
		fmt.Fprintf(&b, "    volumes: [%s]\n", joinQuoted(slices.Concat(volumes, binds)))
	}
	if d.Memory > 0 || d.CPUs > 0 || d.GPU != NoGPU {
		b.WriteString("    deploy:\n      resources:\n")
		if d.Memory > 0 || d.CPUs > 0 {
			b.WriteString("        limits:\n")
			if d.Memory > 0 {
				fmt.Fprintf(&b, "          memory: %dm\n", d.Memory>>20)
			}
			if d.CPUs > 0 {
				fmt.Fprintf(&b, "          cpus: %q\n", strconv.FormatFloat(d.CPUs, 'f', -1, 64))
			}
		}
		if d.GPU != NoGPU {
			b.WriteString("        reservations:\n          devices:\n" + d.GPU.compose())
		}
	}
	if len(d.Volumes) > 0 {
		b.WriteString("volumes:\n")
		for _, v := range slices.Sorted(maps.Keys(d.Volumes)) {
			fmt.Fprintf(&b, "  %s: {}\n", n.Volume(d.Plugin, d.Name, v))
		}
	}

	addr := "http://" + name + ":" + port
	if onHost {
		addr = "http://127.0.0.1:" + port
	}
	return Commands{DockerRun: strings.Join(quoted, " "), Compose: b.String(), Address: addr}
}

// DockerRun returns the docker run command creating and starting a
// container as CreateContainer and StartContainer do, without its labels.
func (c ContainerSpec) DockerRun() string {
	args := []string{"docker", "run", "-d", "--name", c.Name}
	if c.Network != "" {
		args = append(args, "--network", c.Network)
		for _, a := range c.Aliases {
			args = append(args, "--network-alias", a)
		}
	}
	if c.Port > 0 && c.PublishLoopback {
		args = append(args, "-p", "127.0.0.1::"+strconv.Itoa(c.Port))
	}
	for _, p := range peerPorts(c.PeerPort) {
		args = append(args, "-p", p)
	}
	args = append(args, c.GPU.flags()...)
	if c.HostGateway {
		args = append(args, "--add-host", hostGateway)
	}
	if c.Memory > 0 {
		args = append(args, "--memory", strconv.FormatInt(c.Memory>>20, 10)+"m")
	}
	if c.CPUs > 0 {
		args = append(args, "--cpus", strconv.FormatFloat(c.CPUs, 'f', -1, 64))
	}
	for _, v := range slices.Sorted(maps.Keys(c.Volumes)) {
		args = append(args, "-v", v+":"+c.Volumes[v])
	}
	for _, b := range c.Binds {
		if b.Source != "" {
			args = append(args, "-v", b.Source+":"+b.Target)
		} else {
			m := "type=volume,source=" + b.Volume + ",target=" + b.Target
			if b.Subpath != "" {
				m += ",volume-subpath=" + b.Subpath
			}
			args = append(args, "--mount", m)
		}
	}
	for _, e := range c.Env {
		args = append(args, "-e", e)
	}
	args = append(args, entrypointFlags(c.Entrypoint)...)
	args = append(append(args, c.Image), entrypointArgs(c.Entrypoint, c.Cmd)...)
	quoted := make([]string, len(args))
	for i, a := range args {
		quoted[i] = shellQuote(a)
	}
	return strings.Join(quoted, " ")
}

// peerPorts are docker run's publications of a peer port, TCP and UDP.
func peerPorts(port int) []string {
	if port == 0 {
		return nil
	}
	p := strconv.Itoa(port)
	return []string{p + ":" + p + "/tcp", p + ":" + p + "/udp"}
}

// entrypointFlags is docker run's flag of an entrypoint, whose first
// element it takes; entrypointArgs puts the rest before the command.
func entrypointFlags(entrypoint []string) []string {
	if len(entrypoint) == 0 {
		return nil
	}
	return []string{"--entrypoint", entrypoint[0]}
}

func entrypointArgs(entrypoint, cmd []string) []string {
	if len(entrypoint) == 0 {
		return cmd
	}
	return slices.Concat(entrypoint[1:], cmd)
}

func joinQuoted(ss []string) string {
	q := make([]string, len(ss))
	for i, s := range ss {
		q[i] = strconv.Quote(s)
	}
	return strings.Join(q, ", ")
}

// shellQuote quotes s for a POSIX shell when it needs quoting.
func shellQuote(s string) string {
	safe := func(r rune) bool {
		return r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || strings.ContainsRune("-_./:=@,+%", r)
	}
	if s == "<network>" || s != "" && !strings.ContainsFunc(s, func(r rune) bool { return !safe(r) }) {
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
