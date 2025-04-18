package versions

import (
	"fmt"

	"github.com/cloudwego/kitex"
	"github.com/cloudwego/kitex/tool/internal_pkg/log"
)

type DependencyChecker struct {
	minDepRepoPath       string
	minDepRepoVersionStr string
	promptFunc           CheckerPromptFunc
}

const (
	KitexDependencyPath    = "github.com/cloudwego/kitex"
	KitexDependencyVersion = "v0.11.0"
)

var defaultDepdencyChecker = &DependencyChecker{
	minDepRepoPath:       KitexDependencyPath,
	minDepRepoVersionStr: KitexDependencyVersion,
	promptFunc:           DefaultDependencyFailedPrompt,
}

func DefaultDependencyFailedPrompt(minDepRepoPath, currentVersion, toolVersion string) string {
	return fmt.Sprintf(defaultPrompt, toolVersion, minDepRepoPath, currentVersion,
		minDepRepoPath,
		toolVersion,
		minDepRepoPath, toolVersion,
		currentVersion,
		currentVersion,
	)
}

type CheckerPromptFunc func(minDepRepoPath, currentVersion, toolVersion string) string

func RegisterCustomDependencyChecker(
	minDepRepoPath, minDepRepoVersionStr string,
	promptFunc CheckerPromptFunc,
) *DependencyChecker {
	return &DependencyChecker{
		minDepRepoPath:       minDepRepoPath,
		minDepRepoVersionStr: minDepRepoVersionStr,
		promptFunc:           promptFunc,
	}
}

func CheckVersion() (ok bool, err error) {
	cr := defaultDepdencyChecker

	if cr.minDepRepoVersionStr == "" || cr.minDepRepoPath == "" || cr.promptFunc == nil {
		return true, fmt.Errorf("global dependency checker error")
	}

	defer func() {
		if err != nil {
			log.Warnf("[Dependency Check]: %s", err.Error())
		}
	}()
	minDepRepoVersion, err := newVersion(cr.minDepRepoVersionStr)
	if err != nil {
		return true, err
	}

	// output: github.com/xxx/xxx v1.2.3
	depRepoInfo, err := runGoListCmd(cr.minDepRepoPath)
	if err != nil {
		return true, err
	}

	currentVersionStr := parseGoListVersion(depRepoInfo)
	// replace with local repository
	if currentVersionStr == "" {
		return true, ErrDependencyReplacedWithLocalRepo
	}

	currentVersion, err := newVersion(currentVersionStr)
	if err != nil {
		return true, ErrDependencyVersionNotSemantic
	}

	// check compatibility
	if !currentVersion.greatOrEqual(minDepRepoVersion) {
		prompt := cr.promptFunc(cr.minDepRepoPath, currentVersionStr, kitex.Version)
		log.Infof(prompt)
		return false, fmt.Errorf("kitex cmd tool dependency compatibility check failed")
	}

	return true, nil
}
