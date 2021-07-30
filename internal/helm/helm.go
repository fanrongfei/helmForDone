package helm

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"log"
	"os/exec"
	"strings"
	"time"

	"helmForDone/internal/core"
)

type (
	HelmCmd struct {
		Release string
		Chart   string
		Args    []string
		//pre删除release--Y/N
		UnInsArgs []string
		//镜像
		Image  string
		//tag
		Tag string
		PreCmds  [][]string
		PostCmds [][]string
		Runner   Runner

		Test         bool
		TestRollback bool

		OnSuccess                   []func()
		OnTestSuccess               []func()
		OnTestFailed                []func()
		OnTestFailedRollbackSuccess []func()
		OnTestFailedRollbackFailed  []func()
	}

	HelmOption     func(*HelmCmd) error
	HelmModeOption func(*HelmCmd)
	Runner         interface {
		Run(ctx context.Context, command string, args ...string) error
	}
)

func WithInstallUpgradeMode() HelmModeOption {
	return func(c *HelmCmd) {
		c.Args = append([]string{"upgrade", "--install"}, c.Args...)
	}
}

func WithRelease(release string) HelmOption {
	return func(c *HelmCmd) error {
		c.Release = release
		return nil
	}
}

func WithChart(chart string) HelmOption {
	return func(c *HelmCmd) error {
		c.Chart = chart
		return nil
	}
}

func WithNamespace(namespace string) HelmOption {
	return func(c *HelmCmd) error {
		c.Args = append(c.Args, "-n", namespace)
		//uninstall 参数--添加namespace参数
		c.UnInsArgs=append(c.UnInsArgs,"-n", namespace)
		return nil
	}
}

func WithLint(lint bool) HelmOption {
	return func(c *HelmCmd) error {
		if lint {
			c.PreCmds = append(c.PreCmds, []string{
				"helm", "lint", c.Chart,
			})
		}
		return nil
	}
}

func WithAtomic(atomic bool) HelmOption {
	return func(c *HelmCmd) error {
		if atomic {
			c.Args = append(c.Args, "--atomic")
		}
		return nil
	}
}

func WithWait(wait bool) HelmOption {
	return func(c *HelmCmd) error {
		if wait {
			c.Args = append(c.Args, "--wait")
		}
		return nil
	}
}

func WithForce(force bool) HelmOption {
	return func(c *HelmCmd) error {
		if force {
			c.Args = append(c.Args, "--force")
		}
		return nil
	}
}

func WithCleanupOnFail(cleanup bool) HelmOption {
	return func(c *HelmCmd) error {
		if cleanup {
			c.Args = append(c.Args, "--cleanup-on-fail")
		}
		return nil
	}
}

func WithDryRun(dry bool) HelmOption {
	return func(c *HelmCmd) error {
		if dry {
			c.Args = append(c.Args, "--dry-run")
		}
		return nil
	}
}

func WithDebug(dry bool) HelmOption {
	return func(c *HelmCmd) error {
		if dry {
			c.Args = append(c.Args, "--debug")
		}
		return nil
	}
}

func WithTimeout(timeout time.Duration) HelmOption {
	return func(c *HelmCmd) error {
		c.Args = append(c.Args, "--timeout", timeout.String())
		return nil
	}
}

func WithHelmRepos(repos []string) HelmOption {
	return func(c *HelmCmd) error {
		if len(repos) == 0 {
			return nil
		}
		//name=repoUrl=username=password=insecure(yes/no)
		for _, repo := range repos {
			//split := strings.SplitN(repo, "=", 2)
			//if len(split) != 2 {
			//	return fmt.Errorf("not in key=value format: %s", repo)
			//}
			split := strings.SplitN(repo, "=", 5)
			if len(split) != 5 {
				return fmt.Errorf("not in key=value format: %s", repo)
			}
			name := split[0]
			url := split[1]
			username:=split[2]
			password:=split[3]
			insecureNY:=split[4]
			log.Printf("added repo: name:%q url:%q username:%q,password:%q insecureNY:%q", name, url,username,password,insecureNY)
			if insecureNY=="yes"{
				c.PreCmds = append(c.PreCmds, []string{
					"helm", "repo", "add","--username",username,"--password",password, name, url,"--insecure-skip-tls-verify",
				})
			}else{
				c.PreCmds = append(c.PreCmds, []string{
					"helm", "repo", "add","--username",username,"--password",password, name, url,
				})
			}
		}
		c.PreCmds = append(c.PreCmds, []string{
			"helm", "repo", "update",
		})
		//查看repo
		c.PreCmds = append(c.PreCmds, []string{
			"helm", "repo", "list",
		})
		return nil
	}
}

func WithBuildDependencies(build bool, chart string) HelmOption {
	return func(c *HelmCmd) error {
		if build {
			c.PreCmds = append(c.PreCmds, []string{
				"helm", "dependency", "build", chart,
			})
		}
		return nil
	}
}

func WithUpdateDependencies(update bool, chart string) HelmOption {
	return func(c *HelmCmd) error {
		if update {
			c.PreCmds = append(c.PreCmds, []string{
				"helm", "dependency", "update", chart,
			})
		}
		return nil
	}
}

func WithTest(test bool, release string) HelmOption {
	return func(c *HelmCmd) error {
		c.Test = test
		return nil
	}
}

func WithTestRollback(test bool, release string) HelmOption {
	return func(c *HelmCmd) error {
		c.TestRollback = test
		return nil
	}
}

func WithValues(values []string) HelmOption {
	return func(c *HelmCmd) error {
		for _, v := range values {
			split := strings.SplitN(v, "=", 2)
			if len(split) != 2 {
				return fmt.Errorf("not in key=value format: %s", v)
			}
			key := split[0]
			value := split[1]
			//记录image
			if key=="image.repository"{
				c.Image=value
			}
			//记录tag
			if key=="image.tag"{
				c.Tag=value
			}
			c.Args = append(c.Args, "--set", fmt.Sprintf("%s=%s", key, value))
		}
		return nil
	}
}

func WithValuesString(values []string) HelmOption {
	return func(c *HelmCmd) error {
		for _, v := range values {
			split := strings.SplitN(v, "=", 2)
			if len(split) != 2 {
				return fmt.Errorf("not in key=value format: %s", v)
			}
			key := split[0]
			value := split[1]
			c.Args = append(c.Args, "--set-string", fmt.Sprintf("%s=%s", key, value))
		}
		return nil
	}
}

func WithValuesYaml(file string) HelmOption {
	return func(c *HelmCmd) error {
		if file != "" {
			c.Args = append(c.Args, "--values", file)
		}
		return nil
	}
}

func WithPreCommand(command ...string) HelmOption {
	return func(c *HelmCmd) error {
		c.PreCmds = append(c.PreCmds, command)
		return nil
	}
}

func WithPostCommand(command ...string) HelmOption {
	return func(c *HelmCmd) error {
		c.PostCmds = append(c.PostCmds, command)
		return nil
	}
}

func WithKubeConfig(config string) HelmOption {
	return func(c *HelmCmd) error {
		if config != "" {
			c.Args = append(c.Args, "--kubeconfig", config)
			//uninstall 参数--添加release参数
			c.UnInsArgs=append(c.UnInsArgs,"--kubeconfig", config)
		}
		return nil
	}
}

func WithRunner(runner Runner) HelmOption {
	return func(c *HelmCmd) error {
		c.Runner = runner
		return nil
	}
}

func NewHelmCmd(mode HelmModeOption, options ...HelmOption) (*HelmCmd, error) {
	h := &HelmCmd{
		Args:     []string{},
		PreCmds:  [][]string{},
		PostCmds: [][]string{},
		Runner:   nil,
	}
	mode(h)
	for _, option := range options {
		err := option(h)
		if err != nil {
			return nil, fmt.Errorf("unable to parse option: %s", err)
		}
	}
	if h.Release == "" {
		return nil, fmt.Errorf("release name is required")
	}
	if h.Chart == "" {
		return nil, fmt.Errorf("chart path is required")
	}
	if h.Runner == nil {
		return nil, fmt.Errorf("runner is required")
	}
	h.Args = append(h.Args, h.Release, h.Chart)
	//uninstall 参数--添加release参数
	//h.UnInsArgs=append(h.UnInsArgs,h.Release)
	return h, nil
}

func (h *HelmCmd) Run(ctx context.Context) error {
	for _, preCmd := range h.PreCmds {
		err := h.Runner.Run(ctx, preCmd[0], preCmd[1:]...)
		if err != nil {
			return Wrap(err, "precmd failed", core.PreFailErrorKind)
		}
	}
	//是否执行执行helm uninstall release
	if h.C2U(){
		var unistallAr []string
		unistallAr=append(unistallAr, "uninstall",h.Release)
		unistallAr=append(unistallAr, h.UnInsArgs...)
		fmt.Printf("hlem uninstall %s \n",strings.Join(unistallAr," "))
		err :=h.Runner.Run(ctx,"helm",unistallAr...)
		if err != nil {
			fmt.Printf( "helm uninstall failed:%v",err)
			return Wrap(err, "helm uinnstall failed", core.FailedErrorKind)
		}
	}
	fmt.Println("helm upgrade install……")
	err := h.Runner.Run(ctx, "helm", h.Args...)
	if err != nil {
		return Wrap(err, "helm failed", core.FailedErrorKind)
	}
	if h.Test {
		err := h.Runner.Run(ctx, "helm", "test", "--logs", h.Release)
		if err != nil {
			log.Printf("TEST FAILED: %s", err)
			if h.TestRollback {
				rollbackErr := h.Runner.Run(ctx, "helm", "rollback", h.Release)
				if rollbackErr != nil {
					log.Printf("ROLLBACK FAILED: %s", rollbackErr)
					return Wrap(rollbackErr, "release and rollback failed", core.RollbackFailedErrorKind)
				} else {
					log.Printf("TEST FAILED: %s", err)
				}
			}
			return Wrap(err, "release failed and rollback successful", core.RollbackSuccessErrorKind)
		}
	}
	for _, postCmd := range h.PostCmds {
		err := h.Runner.Run(ctx, postCmd[0], postCmd[1:]...)
		if err != nil {
			return Wrap(err, "postcmd failed", core.PostFailErrorKind)
		}
	}
	return nil
}
//create|upgrade
func (h *HelmCmd)C2U()bool{
	OldImage,err:=getReleaseImage(h.Release,h.UnInsArgs)
	if err!=nil{
		return false
	}
	if OldImage==strings.Trim(fmt.Sprintf("%s:%s",h.Image,h.Tag)," "){
		fmt.Printf("oldimage==newimage//(old=%s)\n",OldImage)
		return true
	}else{
		fmt.Printf("oldimage!=newimage//(old=%s;new=%s\n)",OldImage,fmt.Sprintf("%s:%s",h.Image,h.Tag))
	    return false
	}
}
//get release image
func getReleaseImage(releasename string,args []string)(oldimage string,err error){
	var argsManfiest []string
	argsManfiest=append(argsManfiest, "get")
	argsManfiest=append(argsManfiest,"manifest")
	argsManfiest=append(argsManfiest,releasename)
	argsManfiest=append(argsManfiest,args...)
	cmd:=exec.Command("helm",argsManfiest...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		fmt.Println(err)
		return oldimage,err
	}
	defer stdout.Close()
	if err := cmd.Start(); err != nil {
		fmt.Println(err)
		return oldimage,err
	}
	bio:=bufio.NewReader(stdout)
	for{
		bl,err:=bio.ReadBytes('\n')
		if err!=nil{
			fmt.Println(err)
			return oldimage,err
		}else{
			fmt.Println(string(bl))
			if strings.Contains(string(bl)," image: "){
				imaArr:=strings.Split(string(bl),"\"")
				if len(imaArr)!=0{
					oldimage=imaArr[1]
					return oldimage,nil
				}
			}
		}
	}
	return oldimage,errors.New("no such release")
}
type (
	HelmError struct {
		Context string
		Kind    core.ErrorKind
		Err     error
	}
)

func (e *HelmError) Error() string {
	return fmt.Sprintf("%s: %s", e.Context, e.Err)
}

func Wrap(err error, info string, kind core.ErrorKind) *HelmError {
	return &HelmError{
		Context: info,
		Kind:    kind,
		Err:     err,
	}
}
