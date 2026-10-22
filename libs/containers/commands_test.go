package containers_test

import (
	"strings"
	"testing"

	"github.com/mavioai/mavio/libs/containers"
)

func TestRunCommands(t *testing.T) {
	d := containers.Dependency{
		Plugin: "dev.mavio.voice", Name: "whisper", Image: "whisper:1@sha256:abc", Port: 9000,
		Cmd: []string{"serve"}, Env: map[string]string{"KEY": "it's"}, Volumes: map[string]string{"cache": "/root/.cache"},
		Memory: 512 << 20, CPUs: 1.5, GPU: containers.GPURuntime,
	}
	c := containers.RunCommands(containers.Names{}, d, true)
	if want := `docker run -d --name mavio-voice-whisper --restart unless-stopped -p 127.0.0.1:9000:9000 --gpus all --memory 512m --cpus 1.5 -v mavio-voice-whisper-cache:/root/.cache -e 'KEY=it'\''s' whisper:1@sha256:abc serve`; c.DockerRun != want {
		t.Errorf("DockerRun = %s, want = %s", c.DockerRun, want)
	}
	if c.Address != "http://127.0.0.1:9000" {
		t.Errorf("Address = %s, want = http://127.0.0.1:9000", c.Address)
	}
	for _, s := range []string{`image: "whisper:1@sha256:abc"`, `ports: ["127.0.0.1:9000:9000"]`, `KEY: "it's"`, `volumes: ["mavio-voice-whisper-cache:/root/.cache"]`, "memory: 512m", "driver: nvidia", "  mavio-voice-whisper-cache: {}"} {
		if !strings.Contains(c.Compose, s) {
			t.Errorf("Compose lacks %q:\n%s", s, c.Compose)
		}
	}

	cdi := d
	cdi.GPU = containers.GPUCDI
	c = containers.RunCommands(containers.Names{}, cdi, true)
	if !strings.Contains(c.DockerRun, " --device nvidia.com/gpu=all ") || strings.Contains(c.DockerRun, "--gpus") ||
		!strings.Contains(c.Compose, "driver: cdi") || !strings.Contains(c.Compose, `device_ids: ["nvidia.com/gpu=all"]`) {
		t.Errorf("with CDI, got = %s\n%s", c.DockerRun, c.Compose)
	}

	c = containers.RunCommands(containers.Names{}, d, false)
	if !strings.Contains(c.DockerRun, " --network <network> ") || strings.Contains(c.DockerRun, " -p ") || strings.Contains(c.Compose, "ports:") {
		t.Errorf("in a container, got = %s\n%s", c.DockerRun, c.Compose)
	}
	if c.Address != "http://mavio-voice-whisper:9000" {
		t.Errorf("Address = %s, want = http://mavio-voice-whisper:9000", c.Address)
	}
	if strings.Contains(c.DockerRun, "--add-host") || strings.Contains(c.Compose, "extra_hosts") {
		t.Errorf("without host access, got = %s\n%s", c.DockerRun, c.Compose)
	}

	d.HostAccess = true
	c = containers.RunCommands(containers.Names{}, d, true)
	if !strings.Contains(c.DockerRun, " --add-host host.docker.internal:host-gateway ") ||
		!strings.Contains(c.Compose, `extra_hosts: ["host.docker.internal:host-gateway"]`) {
		t.Errorf("with host access, got = %s\n%s", c.DockerRun, c.Compose)
	}

	d.Folders, d.PeerPort, d.Entrypoint = map[string]string{"/srv/downloads": "/downloads"}, 6881, []string{"/bin/sh", "-e"}
	c = containers.RunCommands(containers.Names{}, d, true)
	for _, s := range []string{" -p 6881:6881/tcp -p 6881:6881/udp ", " -v /srv/downloads:/downloads ", " --entrypoint /bin/sh whisper:1@sha256:abc -e serve"} {
		if !strings.Contains(c.DockerRun, s) {
			t.Errorf("with folders, a peer port and an entrypoint, DockerRun = %s, want %q in it", c.DockerRun, s)
		}
	}
	for _, s := range []string{`entrypoint: ["/bin/sh", "-e"]`, `ports: ["127.0.0.1:9000:9000", "6881:6881/tcp", "6881:6881/udp"]`, `volumes: ["mavio-voice-whisper-cache:/root/.cache", "/srv/downloads:/downloads"]`} {
		if !strings.Contains(c.Compose, s) {
			t.Errorf("Compose lacks %q:\n%s", s, c.Compose)
		}
	}
	if strings.Contains(c.Compose, "/srv/downloads: {}") {
		t.Errorf("Compose declares a folder as a volume:\n%s", c.Compose)
	}
}
