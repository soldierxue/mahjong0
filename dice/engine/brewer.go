package engine

import (
	"bufio"
	"bytes"
	"container/list"
	"context"
	"errors"
	"fmt"
	"github.com/gorilla/websocket"
	log "github.com/sirupsen/logrus"
	"io"
	"io/ioutil"
	"math/rand"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"text/template"
)

// ExecutionPlan represents complete plan.
type ExecutionPlan struct {
	Name string `json:"name"`
	CurrentStage *ExecutionStage `json:"currentStage"`
	Plan       *list.List                `json:"plan"`
	PlanMirror map[string]*ExecutionStage `json:"planMirror"`
}

// ExecutionStage represents an unit of execution plan.
type ExecutionStage struct {
	// Name
	Name          string            `json:"name"`
	// Stage type
	Kind          string            `json:"kind"`     //CDK/Command
	WorkHome      string            `json:"workHome"` //root folder for execution
	Preparation   []string          `json:"preparation"`
	Command       *list.List        `json:"command"`
	CommandMirror map[string]string `json:"commandMirror"`
	TileName string	`json:"tileName"`
	TileVersion string `json:"tileVersion"`
}

type StageKind int
const (
	CDK StageKind = iota
	Command
)
func (sk StageKind) SKString() string {
	return [...]string{"CDK", "Command", "FromCommand"}[sk]
}

// BrewerCore represent a group of core functions to execute & manage for
// execution plan.
type BrewerCore interface {
	ExecutePlan(ctx context.Context, dryRun bool, out *websocket.Conn) error
	CommandExecutor(ctx context.Context, stage *ExecutionStage, cmd []byte, out *websocket.Conn) error
	CommandWrapperExecutor(ctx context.Context, stage *ExecutionStage, out *websocket.Conn) (string, error)
	WsTail(ctx context.Context, reader io.ReadCloser, stageLog *log.Logger, out *websocket.Conn)
	ExtractValue(ctx context.Context, buf []byte, out *websocket.Conn) (*TsOutput, error)
}

//ExecutePlan is a orchestrator to run execution plan.
func (ep *ExecutionPlan) ExecutePlan(ctx context.Context, dryRun bool, out *websocket.Conn) error {
	for e := ep.Plan.Back(); e != nil; e = e.Prev() {
		stage := e.Value.(*ExecutionStage)
		ep.CurrentStage = stage
		cmd, err := ep.CommandWrapperExecutor(ctx, stage, out)
		if err != nil {
			return err
		}
		if !dryRun {
			if err := ep.CommandExecutor(ctx, stage, []byte(cmd), out); err != nil {
				return err
			}
			buf, err := ioutil.ReadFile(s3Config.WorkHome+"/super/"+stage.Name+"-output.log")
			if err != nil {
				return err
			}
			ep.ExtractValue(ctx, buf, out)

			//
		}
	}
	return nil
}

// CommandExecutor exec command and return output.
func (ep *ExecutionPlan) CommandExecutor(ctx context.Context, stage *ExecutionStage, cmdTxt []byte, out *websocket.Conn) error {
	ct := string(cmdTxt)

	var stageLog *log.Logger
	if stage !=nil {
		SR(out, []byte("Initializing stage log file ..."))
		stageLog = log.New()
		fileName := s3Config.WorkHome+"/super/"+stage.Name+"-output.log"
		logFile, err := os.OpenFile(fileName, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			SRf(out,"Failed to save stage log, using default stderr, %s\n", err)
			return err
		}
		stageLog.SetOutput(logFile)
		stageLog.SetFormatter(&log.JSONFormatter{DisableTimestamp:true})
		SR(out, []byte("Initializing stage log file with success"))
	}

	SRf(out, "cmd => '%s'\n", ct)
	cts := strings.Split(ct, " ")
	cmd := exec.Command(cts[0], cts[1:len(cts)]...)

	stdoutIn, _ := cmd.StdoutPipe()
	stderrIn, _ := cmd.StderrPipe()

	err := cmd.Start()
	if err != nil {
		SRf(out, "cmd.Start() failed with '%s'\n", err)
		return err
	}
	go ep.WsTail(ctx, stdoutIn, stageLog, out)
	go ep.WsTail(ctx, stderrIn, stageLog, out)

	go func() {
		select {
			case <-ctx.Done():
				err := cmd.Process.Kill()
				log.Printf("halted cmd with %s\n", err)
			}
	}()

	err = cmd.Wait()

	if err != nil {
		SRf(out, "cmd.Run() failed with %s\n", err)
		return err
	}

	return nil
}

// CommandWrapperExecutor wrap commands as a unix script in order to execute.
func (ep *ExecutionPlan) CommandWrapperExecutor(ctx context.Context, stage *ExecutionStage, out *websocket.Conn) (string, error) {
	//stage.WorkHome
	script := stage.WorkHome + "/script-" + stage.Name + "-" + randString(8) + ".sh"
	tContent := `#!/bin/bash
set -xe

{{range .Preparation}}
{{.}}
{{end}}

{{$map := .CommandMirror}}
{{range $key, $value := $map }}
{{$value}}
{{end}}
echo $?
`

	tContent4K8s := `#!/bin/bash
set -xe
[kube.config]
{{range .Preparation}}
{{.}}
{{end}}

{{$map := .CommandMirror}}
{{range $key, $value := $map }}
{{$value}}
{{end}}
echo $?
`
	key := ctx.Value(`d-sid`).(string)
	if ts, ok := AllTs[key]; ok {
		if t, ok := ts.TsStacksMap[stage.TileName]; ok {
			if t.TsManifests != nil {
				if t.TsManifests.VendorService == EKS.VSString() {
					if outputs, ok := ts.AllOutputs[t.TsManifests.DependentTile]; ok {
						clusterName, ok := outputs.TsOutputs["clusterName"]
						if !ok {
							return script, errors.New("ContainerProvider with EKS didn't include output: clusterName.")
						}

						masterRoleARN, ok := outputs.TsOutputs["masterRoleARN"]
						if !ok {
							return script, errors.New("ContainerProvider with EKS didn't include output: masterRoleARN.")
						}
						tContent4K8s = strings.ReplaceAll(tContent4K8s, "[kube.config]",
							fmt.Sprintf("aws eks update-kubeconfig --name %s --role-arn %s --kubeconfig %s\nexport KUBECONFIG=%s",
								clusterName.OutputValue,
								masterRoleARN.OutputValue,
								s3Config.WorkHome+"/super/kube.config",
								s3Config.WorkHome+"/super/kube.config",
							))
						tContent = tContent4K8s
					}
				}
			}
		}
	}

	tp := template.New("script")
	tp, err := tp.Parse(tContent)
	if err != nil {
		return script, err
	}

	file, err := os.OpenFile(script, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0755) //Create(script)
	if err != nil {
		SR(out, []byte(err.Error()))
		return script, err
	}
	defer file.Close()
	err = tp.Execute(file, stage)
	if err != nil {
		SR(out, []byte(err.Error()))
		return script, err
	}
	// Show script
	buf, err := ioutil.ReadFile(script)
	SRf(out, "Generated script -  %s with content: \n", script)
	SR(out, []byte("--BO:-------------------------------------------------"))
	SR(out, buf)
	SR(out, []byte("--EO:-------------------------------------------------"))

	return script, err

}

// WsTail collect output from stdout/stderr, and also catch up defined output value & persist them.
func (ep *ExecutionPlan) WsTail(ctx context.Context, reader io.ReadCloser, stageLog *log.Logger, out *websocket.Conn) {
	scanner := bufio.NewScanner(reader)
	scanner.Split(bufio.ScanLines)
	for scanner.Scan() {
		buf := scanner.Bytes()
		if stageLog != nil {
			stageLog.Printf("%s", buf)
		}
		SR(out, buf)
	}
}

func (ep *ExecutionPlan) ExtractValue(ctx context.Context, buf []byte, out *websocket.Conn) error {
	key := ctx.Value(`d-sid`).(string)
	if ts, ok := AllTs[key]; ok {
		tileName := ep.CurrentStage.TileName
		var tileCategory string
		//var vendorService string
		if tile, ok := ts.AllTiles[tileName]; ok {
			tileCategory = tile.Metadata.Category
			//vendorService = tile.Metadata.VendorService
		}

		if outputs, ok := ts.AllOutputs[tileName]; ok {
			outputs.StageName = ep.CurrentStage.Name
			for outputName, outputDetail := range outputs.TsOutputs {
				var regx *regexp.Regexp
				if tileCategory == ContainerApplication.CString() || tileCategory == Application.CString() {
					// Extract key, value from Command outputs
					regx = regexp.MustCompile("^\\{\"(" +
						outputName +
						"=" +
						".*?)\"}$")
				} else {
					// Extract key, value from CDK outputs
					if stack, ok := ts.TsStacksMap[tileName]; ok {
						regx = regexp.MustCompile("^\\{\"level\":\"info\"\\,\"msg\".*("+
							stack.TileStackName+"."+
							stack.TileName+
							outputName+
							".*?)\"}$")

					}
				}
				scanner := bufio.NewScanner(bytes.NewReader(buf))
				scanner.Split(bufio.ScanLines)
				for scanner.Scan() {
					txt := scanner.Text()
					match := regx.FindStringSubmatch(txt)
					if len(match) >0 {
						kv := strings.Split(match[1],"=")
						outputDetail.OutputValue = strings.TrimSpace(kv[1])
						SRf(out,"Extract outputs: [%s] = [%s] ", outputName,strings.TrimSpace(kv[1]))
						break
					}
				}

			}

		}

	}

	return nil
}


func randString(n int) string {
	const letterBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	b := make([]byte, n)
	for i := range b {
		b[i] = letterBytes[rand.Int63()%int64(len(letterBytes))]
	}
	return string(b)
}
