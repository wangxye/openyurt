/*
Copyright 2024 The OpenYurt Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package util

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"text/template"
	"time"
)

const (
	keepalivedPid  = "/var/run/keepalived.pid"
	vrrpPid        = "/var/run/vrrp.pid"
	keepalivedPath = "/etc/keepalived/keepalived.conf"
)

type LBService struct {
	Name     string
	Priority int
	Vrid     int
	Ips      []string
}

type Keepalived struct {
	Iface          string
	State          string
	Started        bool
	Cmd            *exec.Cmd
	KeepalivedTmpl *template.Template
	emptyTmpl      *template.Template
	LBServices     map[string]LBService
}

func (k *Keepalived) Start() error {
	args := []string{"--dont-fork", "--log-console", "--log-detail", "--release-vips"}
	args = append(args, fmt.Sprintf("--pid=%s", keepalivedPid))
	args = append(args, fmt.Sprintf("--vrrp_pid=%s", vrrpPid))

	k.Cmd = exec.Command("keepalived", args...)
	k.Cmd.Stdout = os.Stdout
	k.Cmd.Stderr = os.Stderr

	k.Cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
		Pgid:    0,
	}

	k.Started = true

	if err := k.Cmd.Run(); err != nil {
		return fmt.Errorf("starting keepalived error: %v", err)
	}
	return nil
}

func (k *Keepalived) ReloadKeepalived() error {
	for !k.IsRunning() {
		time.Sleep(time.Second)
	}

	err := syscall.Kill(k.Cmd.Process.Pid, syscall.SIGHUP)
	if err != nil {
		return fmt.Errorf("reload keepalived error: %v", err)
	}

	return nil
}

func (k *Keepalived) IsRunning() bool {
	if !k.Started {
		return false
	}

	if _, err := os.Stat(keepalivedPid); os.IsNotExist(err) {
		return false
	}

	return true
}

func (k *Keepalived) Healthy() error {
	if !k.IsRunning() {
		return fmt.Errorf("keepalived is not running")
	}

	if _, err := os.Stat(vrrpPid); os.IsNotExist(err) {
		return fmt.Errorf("VRRP child process not running")
	}

	// TODO: check whether bind correct vips

	return nil
}

func (k *Keepalived) Stop() error {
	err := syscall.Kill(k.Cmd.Process.Pid, syscall.SIGTERM)
	if err != nil {
		return fmt.Errorf("stopping keepalived error: %v", err)
	}
	return err
}

func (k *Keepalived) LoadConfig() error {
	conf := make(map[string]interface{})
	conf["Interface"] = k.Iface
	conf["State"] = k.State
	conf["LBServices"] = k.LBServices

	w, err := os.Create(keepalivedPath)
	if err != nil {
		return err
	}
	defer w.Close()
	if len(k.LBServices) == 0 {
		err = k.emptyTmpl.Execute(w, conf)
	} else {
		err = k.KeepalivedTmpl.Execute(w, conf)
	}
	if err != nil {
		return fmt.Errorf("unexpected error creating keepalived.conf: %v", err)
	}

	return nil
}

func (k *Keepalived) LoadTemplate() error {
	tmpl, err := template.New("empty").Parse(`
global_defs {
	
}`)
	if err != nil {
		return fmt.Errorf("load keepalived empty template error: %v", err)
	}

	dir := filepath.Dir(keepalivedPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	k.emptyTmpl = tmpl

	tmpl, err = template.New("keepalived").Parse(`
{{ range $name, $service := .LBServices }}
vrrp_instance {{ $name }} {
  state {{ $.State }}
  interface {{ $.Interface }}
  virtual_router_id {{ $service.Vrid }}
  priority {{ $service.Priority }}
  virtual_ipaddress {
	{{ range $service.Ips }}{{ . }}
	{{ end }}
  }
}
{{ end }}
`)
	if err != nil {
		return fmt.Errorf("load keepalived template error: %v", err)
	}

	k.KeepalivedTmpl = tmpl

	return nil
}
