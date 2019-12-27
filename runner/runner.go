package runner

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	base "github.com/omegaup/go-base"
	"github.com/omegaup/quark/common"
	"github.com/vincent-petithory/dataurl"
	"io"
	"io/ioutil"
	"math"
	"math/big"
	"os"
	"path"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// A CaseResult represents the sub-results of a specific test case.
type CaseResult struct {
	Verdict        string                 `json:"verdict"`
	Name           string                 `json:"name"`
	Score          *big.Rat               `json:"score"`
	ContestScore   *big.Rat               `json:"contest_score"`
	MaxScore       *big.Rat               `json:"max_score"`
	Meta           RunMetadata            `json:"meta"`
	IndividualMeta map[string]RunMetadata `json:"individual_meta,omitempty"`
}

// MarshalJSON implements the json.Marshaler interface.
func (c *CaseResult) MarshalJSON() ([]byte, error) {
	return json.Marshal(&struct {
		Verdict        string                 `json:"verdict"`
		Name           string                 `json:"name"`
		Score          float64                `json:"score"`
		ContestScore   float64                `json:"contest_score"`
		MaxScore       float64                `json:"max_score"`
		Meta           RunMetadata            `json:"meta"`
		IndividualMeta map[string]RunMetadata `json:"individual_meta,omitempty"`
	}{
		Verdict:        c.Verdict,
		Name:           c.Name,
		Score:          base.RationalToFloat(c.Score),
		ContestScore:   base.RationalToFloat(c.ContestScore),
		MaxScore:       base.RationalToFloat(c.MaxScore),
		Meta:           c.Meta,
		IndividualMeta: c.IndividualMeta,
	})
}

// UnmarshalJSON implements the json.Unmarshaler interface.
func (c *CaseResult) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, []byte("null")) {
		return nil
	}

	result := struct {
		Verdict        string                 `json:"verdict"`
		Name           string                 `json:"name"`
		Score          float64                `json:"score"`
		ContestScore   float64                `json:"contest_score"`
		MaxScore       float64                `json:"max_score"`
		Meta           RunMetadata            `json:"meta"`
		IndividualMeta map[string]RunMetadata `json:"individual_meta,omitempty"`
	}{}

	if err := json.Unmarshal(data, &result); err != nil {
		return err
	}

	c.Verdict = result.Verdict
	c.Name = result.Name
	c.Score = base.FloatToRational(result.Score)
	c.ContestScore = base.FloatToRational(result.ContestScore)
	c.MaxScore = base.FloatToRational(result.MaxScore)
	c.Meta = result.Meta
	c.IndividualMeta = result.IndividualMeta

	return nil
}

// A GroupResult represents the sub-results of a specific group of test cases.
type GroupResult struct {
	Group        string       `json:"group"`
	Score        *big.Rat     `json:"score"`
	ContestScore *big.Rat     `json:"contest_score"`
	MaxScore     *big.Rat     `json:"max_score"`
	Cases        []CaseResult `json:"cases"`
}

// MarshalJSON implements the json.Marshaler interface.
func (g *GroupResult) MarshalJSON() ([]byte, error) {
	return json.Marshal(&struct {
		Group        string       `json:"group"`
		Score        float64      `json:"score"`
		ContestScore float64      `json:"contest_score"`
		MaxScore     float64      `json:"max_score"`
		Cases        []CaseResult `json:"cases"`
	}{
		Group:        g.Group,
		Score:        base.RationalToFloat(g.Score),
		ContestScore: base.RationalToFloat(g.ContestScore),
		MaxScore:     base.RationalToFloat(g.MaxScore),
		Cases:        g.Cases,
	})
}

// UnmarshalJSON implements the json.Unmarshaler interface.
func (g *GroupResult) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, []byte("null")) {
		return nil
	}

	result := struct {
		Group        string       `json:"group"`
		Score        float64      `json:"score"`
		ContestScore float64      `json:"contest_score"`
		MaxScore     float64      `json:"max_score"`
		Cases        []CaseResult `json:"cases"`
	}{}

	if err := json.Unmarshal(data, &result); err != nil {
		return err
	}

	g.Group = result.Group
	g.Score = base.FloatToRational(result.Score)
	g.ContestScore = base.FloatToRational(result.ContestScore)
	g.MaxScore = base.FloatToRational(result.MaxScore)
	g.Cases = result.Cases

	return nil
}

// A RunResult represents the results of a run.
type RunResult struct {
	Verdict      string                 `json:"verdict"`
	CompileError *string                `json:"compile_error,omitempty"`
	CompileMeta  map[string]RunMetadata `json:"compile_meta"`
	Score        *big.Rat               `json:"score"`
	ContestScore *big.Rat               `json:"contest_score"`
	MaxScore     *big.Rat               `json:"max_score"`
	Time         float64                `json:"time"`
	WallTime     float64                `json:"wall_time"`
	Memory       base.Byte              `json:"memory"`
	JudgedBy     string                 `json:"judged_by,omitempty"`
	Groups       []GroupResult          `json:"groups"`
}

// NewRunResult returns a new RunResult.
func NewRunResult(verdict string, maxScore *big.Rat) *RunResult {
	return &RunResult{
		Verdict:      verdict,
		Score:        &big.Rat{},
		ContestScore: &big.Rat{},
		MaxScore:     maxScore,
	}
}

// MarshalJSON implements the json.Marshaler interface.
func (r *RunResult) MarshalJSON() ([]byte, error) {
	return json.Marshal(&struct {
		Verdict      string                 `json:"verdict"`
		CompileError *string                `json:"compile_error,omitempty"`
		CompileMeta  map[string]RunMetadata `json:"compile_meta"`
		Score        float64                `json:"score"`
		ContestScore float64                `json:"contest_score"`
		MaxScore     float64                `json:"max_score"`
		Time         float64                `json:"time"`
		WallTime     float64                `json:"wall_time"`
		Memory       base.Byte              `json:"memory"`
		JudgedBy     string                 `json:"judged_by,omitempty"`
		Groups       []GroupResult          `json:"groups"`
	}{
		Verdict:      r.Verdict,
		CompileError: r.CompileError,
		CompileMeta:  r.CompileMeta,
		Score:        base.RationalToFloat(r.Score),
		ContestScore: base.RationalToFloat(r.ContestScore),
		MaxScore:     base.RationalToFloat(r.MaxScore),
		Time:         r.Time,
		WallTime:     r.WallTime,
		Memory:       r.Memory,
		JudgedBy:     r.JudgedBy,
		Groups:       r.Groups,
	})
}

// UnmarshalJSON implements the json.Unmarshaler interface.
func (r *RunResult) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, []byte("null")) {
		return nil
	}

	result := struct {
		Verdict      string                 `json:"verdict"`
		CompileError *string                `json:"compile_error,omitempty"`
		CompileMeta  map[string]RunMetadata `json:"compile_meta"`
		Score        float64                `json:"score"`
		ContestScore float64                `json:"contest_score"`
		MaxScore     float64                `json:"max_score"`
		Time         float64                `json:"time"`
		WallTime     float64                `json:"wall_time"`
		Memory       base.Byte              `json:"memory"`
		JudgedBy     string                 `json:"judged_by,omitempty"`
		Groups       []GroupResult          `json:"groups"`
	}{}

	if err := json.Unmarshal(data, &result); err != nil {
		return err
	}

	r.Verdict = result.Verdict
	r.CompileError = result.CompileError
	r.CompileMeta = result.CompileMeta
	r.Score = base.FloatToRational(result.Score)
	r.ContestScore = base.FloatToRational(result.ContestScore)
	r.MaxScore = base.FloatToRational(result.MaxScore)
	r.Time = result.Time
	r.WallTime = result.WallTime
	r.Memory = result.Memory
	r.JudgedBy = result.JudgedBy
	r.Groups = result.Groups

	return nil
}

type binaryType int

const (
	binaryProblemsetter binaryType = iota
	binaryContestant
	binaryValidator
)

type binary struct {
	name             string
	target           string
	language         string
	binPath          string
	outputPathPrefix string
	binaryType       binaryType
	limits           common.LimitsSettings
	receiveInput     bool
	sourceFiles      []string
	extraFlags       []string
	extraMountPoints map[string]string
}

type intermediateRunResult struct {
	name           string
	runMeta        *RunMetadata
	binaryType     binaryType
	generatedFiles []string
}

type outputOnlyFile struct {
	contents string
	ole      bool
}

func extraParentFlags(language string) []string {
	if language == "c" || language == "cpp" || language == "cpp11" {
		return []string{"-Wl,-e__entry"}
	}
	return []string{}
}

func targetName(language string, target string) string {
	if language == "py" || language == "py2" || language == "py3" || language == "java" {
		return fmt.Sprintf("%s_entry", target)
	}
	return target
}

func normalizedSourceFiles(
	runRoot string,
	lang string,
	name string,
	iface *common.InteractiveInterface,
) []string {
	binRoot := path.Join(runRoot, name, "bin")
	sources := make([]string, len(iface.MakefileRules[0].Requisites))
	for idx, requisite := range iface.MakefileRules[0].Requisites {
		sources[idx] = path.Join(binRoot, path.Base(requisite))
	}
	return sources
}

func parseOutputOnlyFile(
	ctx *common.Context,
	data string,
	settings *common.ProblemSettings,
) (map[string]outputOnlyFile, error) {
	dataURL, err := dataurl.DecodeString(data)
	result := make(map[string]outputOnlyFile)
	if err != nil {
		// |data| is not a dataurl. Try just returning the data as an Entry.
		ctx.Log.Info("data is not a dataurl. Generating Main.out", "err", err)
		result["Main.out"] = outputOnlyFile{data, false}
		return result, nil
	}
	z, err := zip.NewReader(bytes.NewReader(dataURL.Data), int64(len(dataURL.Data)))
	if err != nil {
		ctx.Log.Warn("error reading zip", "err", err)
		return result, err
	}

	expectedFileNames := make(map[string]struct{})
	for _, groupSettings := range settings.Cases {
		for _, caseSettings := range groupSettings.Cases {
			expectedFileNames[fmt.Sprintf("%s.out", caseSettings.Name)] = struct{}{}
		}
	}

	for _, f := range z.File {
		if !strings.HasSuffix(f.FileHeader.Name, ".out") {
			ctx.Log.Info(
				"Output-only compressed file has invalid name. Skipping",
				"name", f.FileHeader.Name,
			)
			continue
		}
		// Some people just cannot follow instructions. Be a little bit more
		// tolerant and skip any intermediate directories.
		fileName := f.FileHeader.Name
		if idx := strings.LastIndex(fileName, "/"); idx != -1 {
			fileName = fileName[idx+1:]
		}
		if _, ok := expectedFileNames[fileName]; !ok {
			ctx.Log.Info(
				"Output-only compressed file not expected. Skipping",
				"name", f.FileHeader.Name,
			)
			continue
		}
		if f.FileHeader.UncompressedSize64 > uint64(settings.Limits.OutputLimit) {
			ctx.Log.Info(
				"Output-only compressed file is too large. Generating empty file",
				"name", f.FileHeader.Name,
				"size", f.FileHeader.UncompressedSize64,
			)
			result[fileName] = outputOnlyFile{"", true}
			continue
		}
		rc, err := f.Open()
		if err != nil {
			ctx.Log.Info(
				"Error opening file",
				"name", f.FileHeader.Name,
				"err", err,
			)
			continue
		}
		defer rc.Close()
		var buf bytes.Buffer
		if _, err := io.Copy(&buf, rc); err != nil {
			ctx.Log.Info(
				"Error reading file",
				"name", f.FileHeader.Name,
				"err", err,
			)
			continue
		}
		result[fileName] = outputOnlyFile{buf.String(), false}
	}
	return result, nil
}

func generateParentMountpoints(
	runRoot string,
	interactive *common.InteractiveSettings,
) map[string]string {
	result := make(map[string]string)
	for name := range interactive.Interfaces {
		if name == interactive.Main {
			continue
		}
		for src, dst := range generateMountpoint(runRoot, name) {
			result[src] = dst
		}
	}
	return result
}

func generateMountpoint(
	runRoot string,
	name string,
) map[string]string {
	return map[string]string{
		path.Join(runRoot, name, "pipes"): fmt.Sprintf("/home/%s_pipes", name),
	}
}

func validatorLimits(
	limits *common.LimitsSettings,
	validatorLimits *common.LimitsSettings,
) *common.LimitsSettings {
	var limitsCopy common.LimitsSettings
	if validatorLimits != nil {
		limitsCopy = *validatorLimits
	} else {
		limitsCopy = common.DefaultValidatorLimits
		limitsCopy.TimeLimit = limits.TimeLimit
	}
	return &limitsCopy
}

// copyFile copies one file. First it tries to use os.Link() to make the
// process faster, but if that fails, it falls back to a physical copy of it.
// This can be needed if the runner is invoked in oneshot mode and the input is
// in a different mount than the runtime path, which is something not supported
// by hard links.
func copyFile(src string, dst string) error {
	err := os.Link(src, dst)
	if err == nil {
		return nil
	}

	srcFd, err := os.Open(src)
	if err != nil {
		return nil
	}
	defer srcFd.Close()

	dstFd, err := os.Create(dst)
	if err != nil {
		return nil
	}
	defer dstFd.Close()

	_, err = io.Copy(dstFd, srcFd)
	return err
}

// Grade compiles and runs a contestant-provided program, supplies it with the
// Input-specified inputs, and computes its final score and verdict.
func Grade(
	ctx *common.Context,
	filesWriter io.Writer,
	run *common.Run,
	input common.Input,
	sandbox Sandbox,
) (*RunResult, error) {
	runResult := NewRunResult("JE", run.MaxScore)
	if !sandbox.Supported() {
		return runResult, errors.New("Sandbox not supported")
	}
	runRoot := path.Join(
		ctx.Config.Runner.RuntimePath,
		"grade",
		strconv.FormatUint(run.AttemptID, 10),
	)
	if !ctx.Config.Runner.PreserveFiles {
		defer os.RemoveAll(runRoot)
	}

	ctx.Log.Info("Running", "run", run)

	var binaries []*binary
	var outputOnlyFiles map[string]outputOnlyFile
	runResult.CompileMeta = make(map[string]RunMetadata)

	settings := *input.Settings()

	// totalWeightFactor is used to normalize all the weights in the case data.
	totalWeightFactor := new(big.Rat)
	for _, group := range settings.Cases {
		for _, caseData := range group.Cases {
			totalWeightFactor.Add(totalWeightFactor, caseData.Weight)
		}
	}
	if totalWeightFactor.Cmp(new(big.Rat)) <= 0 {
		totalWeightFactor = big.NewRat(1, 1)
	} else {
		totalWeightFactor.Quo(big.NewRat(1, 1), totalWeightFactor)
	}

	interactive := settings.Interactive
	if interactive != nil {
		ctx.Log.Info("libinteractive", "version", interactive.LibinteractiveVersion)
		lang := interactive.ParentLang
		target := targetName(run.Language, interactive.Main)

		if lang == "cpp" {
			// Let's not make problemsetters be forced to use old languages.
			lang = "cpp11"
		}

		binaries = []*binary{
			{
				name:             interactive.Main,
				target:           target,
				language:         lang,
				binPath:          path.Join(runRoot, interactive.Main, "bin"),
				outputPathPrefix: "",
				binaryType:       binaryProblemsetter,
				limits:           *validatorLimits(&settings.Limits, settings.Validator.Limits),
				receiveInput:     true,
				sourceFiles: normalizedSourceFiles(
					runRoot,
					interactive.ParentLang,
					interactive.Main,
					interactive.Interfaces[interactive.Main][interactive.ParentLang],
				),
				extraFlags:       extraParentFlags(interactive.ParentLang),
				extraMountPoints: generateParentMountpoints(runRoot, interactive),
			},
		}
		for name, langIface := range interactive.Interfaces {
			if name == interactive.Main {
				continue
			}
			iface, ok := langIface[common.LanguageFileExtension(run.Language)]
			if !ok {
				runResult.Verdict = "CE"
				compileError := fmt.Sprintf("libinteractive does not support language '%s'", run.Language)
				runResult.CompileError = &compileError
				return runResult, nil
			}
			target := targetName(run.Language, name)
			binaries = append(
				binaries,
				&binary{
					name:             name,
					target:           target,
					language:         run.Language,
					binPath:          path.Join(runRoot, name, "bin"),
					outputPathPrefix: name,
					binaryType:       binaryContestant,
					limits:           settings.Limits,
					receiveInput:     false,
					sourceFiles: normalizedSourceFiles(
						runRoot,
						run.Language,
						name,
						iface,
					),
					extraFlags:       []string{},
					extraMountPoints: generateMountpoint(runRoot, name),
				},
			)
		}

		// Setup all source files.
		for _, bin := range binaries {
			binPath := path.Join(runRoot, bin.name, "bin")
			if err := os.MkdirAll(binPath, 0755); err != nil {
				return runResult, err
			}
		}
		if err := copyFile(
			path.Join(
				input.Path(),
				fmt.Sprintf(
					"interactive/Main.%s",
					common.LanguageFileExtension(interactive.ParentLang),
				),
			),
			path.Join(
				runRoot,
				fmt.Sprintf(
					"Main/bin/Main.%s",
					common.LanguageFileExtension(interactive.ParentLang),
				),
			),
		); err != nil {
			return runResult, err
		}
		for name, langIface := range interactive.Interfaces {
			var lang string
			if name == "Main" {
				lang = common.LanguageFileExtension(interactive.ParentLang)
			} else {
				lang = common.LanguageFileExtension(run.Language)
			}
			for filename, contents := range langIface[lang].Files {
				sourcePath := path.Join(
					runRoot,
					fmt.Sprintf("%s/bin/%s", name, path.Base(filename)),
				)
				err := ioutil.WriteFile(sourcePath, []byte(contents), 0644)
				if err != nil {
					return runResult, err
				}
			}
			if name == "Main" {
				for ifaceName := range interactive.Interfaces {
					if ifaceName == "Main" {
						continue
					}
					pipesMountPath := path.Join(
						runRoot,
						fmt.Sprintf("%s/bin/%s_pipes", name, ifaceName),
					)
					if err := os.MkdirAll(pipesMountPath, 0755); err != nil {
						return runResult, err
					}
				}
				continue
			}
			sourcePath := path.Join(
				runRoot,
				fmt.Sprintf(
					"%s/bin/%s.%s",
					name,
					interactive.ModuleName,
					common.LanguageFileExtension(run.Language),
				),
			)
			err := ioutil.WriteFile(sourcePath, []byte(run.Source), 0644)
			if err != nil {
				return runResult, err
			}
			pipesMountPath := path.Join(
				runRoot,
				fmt.Sprintf("%s/bin/%s_pipes", name, name),
			)
			if err := os.MkdirAll(pipesMountPath, 0755); err != nil {
				return runResult, err
			}
			pipesPath := path.Join(runRoot, name, "pipes")
			if err := os.MkdirAll(pipesPath, 0755); err != nil {
				return runResult, err
			}
			if err := syscall.Mkfifo(path.Join(pipesPath, "in"), 0644); err != nil {
				return runResult, err
			}
			if err := syscall.Mkfifo(path.Join(pipesPath, "out"), 0644); err != nil {
				return runResult, err
			}
		}
	} else {
		// Setup all source files.
		mainBinPath := path.Join(runRoot, "Main", "bin")
		if err := os.MkdirAll(mainBinPath, 0755); err != nil {
			return runResult, err
		}
		mainSourcePath := path.Join(
			mainBinPath,
			fmt.Sprintf("Main.%s", common.LanguageFileExtension(run.Language)),
		)
		err := ioutil.WriteFile(mainSourcePath, []byte(run.Source), 0644)
		if err != nil {
			return runResult, err
		}

		if run.Language == "cat" {
			outputOnlyFiles, err = parseOutputOnlyFile(ctx, run.Source, &settings)
			if err != nil {
				runResult.Verdict = "CE"
				compileError := err.Error()
				runResult.CompileError = &compileError
				return runResult, nil
			}
			runResult.CompileMeta["Main"] = RunMetadata{
				Verdict: "OK",
			}
			binaries = []*binary{}
		} else {
			extraFlags := []string{}
			if run.Debug &&
				(run.Language == "c" || run.Language == "cpp" || run.Language == "cpp11") {
				// We don't ship the dynamic library for ASan, so link it statically.
				extraFlags = []string{"-static-libasan", "-fsanitize=address"}
				// ASan uses TONS of extra memory.
				settings.Limits.MemoryLimit = -1
				// ASan claims to be 2x slower.
				settings.Limits.TimeLimit = settings.Limits.TimeLimit*2 + base.Duration(1*time.Second)
				// 16kb should be enough to emit the report.
				settings.Limits.OutputLimit += 16 * 1024
			}
			binaries = []*binary{
				{
					name:             "Main",
					target:           "Main",
					language:         run.Language,
					binPath:          mainBinPath,
					outputPathPrefix: "",
					binaryType:       binaryContestant,
					limits:           settings.Limits,
					receiveInput:     true,
					sourceFiles:      []string{mainSourcePath},
					extraFlags:       extraFlags,
					extraMountPoints: map[string]string{},
				},
			}
		}
	}

	validatorBinPath := path.Join(runRoot, "validator", "bin")
	regularBinaryCount := len(binaries)
	if settings.Validator.Name == "custom" {
		if err := os.MkdirAll(validatorBinPath, 0755); err != nil {
			return runResult, err
		}
		validatorLang := *settings.Validator.Lang
		// The file will always have the actual language as the extension.
		validatorInputFile := path.Join(
			input.Path(),
			fmt.Sprintf("validator.%s", validatorLang),
		)
		// But for omegajail's purposes, the extension needs to be normalized (e.g. .py3 -> .py)
		validatorSourceFile := path.Join(
			validatorBinPath,
			fmt.Sprintf("validator.%s", common.LanguageFileExtension(validatorLang)),
		)
		err := copyFile(validatorInputFile, validatorSourceFile)
		if err != nil {
			return runResult, err
		}
		binaries = append(
			binaries,
			&binary{
				name:             "validator",
				target:           "validator",
				language:         validatorLang,
				binPath:          validatorBinPath,
				outputPathPrefix: "validator",
				binaryType:       binaryValidator,
				limits:           *validatorLimits(&settings.Limits, settings.Validator.Limits),
				receiveInput:     false,
				sourceFiles:      []string{validatorSourceFile},
				extraFlags:       []string{},
				extraMountPoints: map[string]string{},
			},
		)
	}

	generatedFiles := make([]string, 0)

	ctx.EventCollector.Add(ctx.EventFactory.NewEvent("compile", common.EventBegin))
	for _, b := range binaries {
		binRoot := path.Join(runRoot, b.name)
		binPath := path.Join(binRoot, "bin")

		singleCompileEvent := ctx.EventFactory.NewCompleteEvent(
			b.name,
			common.Arg{Name: "language", Value: b.language},
		)
		lang := b.language
		if b.binaryType == binaryValidator && lang == "cpp" {
			// Let's not make problemsetters be forced to use old languages.
			lang = "cpp11"
		}
		compileMeta, err := sandbox.Compile(
			ctx,
			lang,
			b.sourceFiles,
			binPath,
			path.Join(binRoot, "compile.out"),
			path.Join(binRoot, "compile.err"),
			path.Join(binRoot, "compile.meta"),
			b.target,
			b.extraFlags,
		)
		ctx.EventCollector.Add(singleCompileEvent)
		generatedFiles = append(
			generatedFiles,
			path.Join(b.name, "compile.out"),
			path.Join(b.name, "compile.err"),
			path.Join(b.name, "compile.meta"),
		)

		if compileMeta != nil {
			runResult.CompileMeta[b.name] = *compileMeta
		}

		if err != nil || compileMeta.Verdict != "OK" {
			ctx.Log.Error("Compile error", "err", err, "compileMeta", compileMeta)
			runResult.Verdict = "CE"
			compileErrorFile := "compile.err"
			if b.language == "pas" || b.language == "cs" {
				// Lazarus and dotnet writes the output of the compile error in compile.out.
				compileErrorFile = "compile.out"
			} else {
				compileErrorFile = "compile.err"
			}
			compileError := fmt.Sprintf(
				"%s:\n%s",
				b.name,
				getCompileError(path.Join(binRoot, compileErrorFile)),
			)
			runResult.CompileError = &compileError
			ctx.EventCollector.Add(ctx.EventFactory.NewEvent("compile", common.EventEnd))
			return runResult, err
		}
	}
	ctx.EventCollector.Add(ctx.EventFactory.NewEvent("compile", common.EventEnd))

	groupResults := make([]GroupResult, len(settings.Cases))
	runResult.Verdict = "OK"
	ctx.EventCollector.Add(ctx.EventFactory.NewEvent("run", common.EventBegin))
	for i, group := range settings.Cases {
		caseResults := make([]CaseResult, len(group.Cases))
		for j, caseData := range group.Cases {
			var runMeta *RunMetadata
			var individualMeta = make(map[string]RunMetadata)
			if runResult.WallTime > settings.Limits.OverallWallTimeLimit.Seconds() {
				ctx.Log.Debug(
					"Not even running since the wall time limit has been exceeded",
					"case", caseData.Name,
					"wall time", runResult.WallTime,
					"limit", settings.Limits.OverallWallTimeLimit.Seconds(),
				)
				runMeta = &RunMetadata{
					Verdict: "TLE",
				}
			} else if run.Language == "cat" {
				outName := fmt.Sprintf("%s.out", caseData.Name)
				errName := fmt.Sprintf("%s.err", caseData.Name)
				metaName := fmt.Sprintf("%s.meta", caseData.Name)
				outPath := path.Join(runRoot, outName)
				metaPath := path.Join(runRoot, metaName)
				if file, ok := outputOnlyFiles[outName]; ok {
					if err := ioutil.WriteFile(outPath, []byte(file.contents), 0644); err != nil {
						ctx.Log.Error(
							"failed to write output file contents",
							"case", caseData.Name,
							"path", outPath,
							"err", err,
						)
					}
					runMeta = &RunMetadata{
						Verdict: "OK",
					}
					if file.ole {
						runMeta.Verdict = "OLE"
					}
					if err := ioutil.WriteFile(metaPath, []byte("status:0"), 0644); err != nil {
						ctx.Log.Error(
							"failed to write meta file",
							"case", caseData.Name,
							"path", metaPath,
							"err", err,
						)
					}
				} else {
					ctx.Log.Error(
						"missing an output file",
						"case", caseData.Name,
						"path", outPath,
					)
					if err := ioutil.WriteFile(outPath, []byte{}, 0644); err != nil {
						ctx.Log.Error(
							"failed to write output file",
							"case", caseData.Name,
							"path", outPath,
							"err", err,
						)
					}
					runMeta = &RunMetadata{
						Verdict: "RTE",
					}
					if err := ioutil.WriteFile(metaPath, []byte("status:1"), 0644); err != nil {
						ctx.Log.Error(
							"failed to write meta file",
							"case", caseData.Name,
							"path", metaPath,
							"err", err,
						)
					}
				}
				errPath := path.Join(runRoot, errName)
				if err := ioutil.WriteFile(errPath, []byte{}, 0644); err != nil {
					ctx.Log.Error(
						"failed to write err file",
						"case", caseData.Name,
						"path", metaPath,
						"err", err,
					)
				}
				generatedFiles = append(generatedFiles, outName, errName, metaName)
			} else {
				singleRunEvent := ctx.EventFactory.NewCompleteEvent(caseData.Name)
				metaChan := make(chan intermediateRunResult, regularBinaryCount)
				for _, bin := range binaries {
					if bin.binaryType == binaryValidator {
						continue
					}
					go func(bin *binary, caseData *common.CaseSettings) {
						var inputPath string
						if bin.receiveInput {
							inputPath = path.Join(
								input.Path(),
								"cases",
								fmt.Sprintf("%s.in", caseData.Name),
							)
						} else {
							inputPath = "/dev/null"
						}
						extraParams := make([]string, 0)
						if bin.binaryType == binaryProblemsetter {
							extraParams = append(extraParams, caseData.Name, run.Language)
						}
						singleBinary := ctx.EventFactory.NewCompleteEvent(
							fmt.Sprintf("%s - %s", caseData.Name, bin.name),
						)
						runMeta, err := sandbox.Run(
							ctx,
							&bin.limits,
							bin.language,
							bin.binPath,
							inputPath,
							path.Join(
								runRoot,
								bin.outputPathPrefix,
								fmt.Sprintf("%s.out", caseData.Name),
							),
							path.Join(
								runRoot,
								bin.outputPathPrefix,
								fmt.Sprintf("%s.err", caseData.Name),
							),
							path.Join(
								runRoot,
								bin.outputPathPrefix,
								fmt.Sprintf("%s.meta", caseData.Name),
							),
							bin.target,
							nil,
							nil,
							nil,
							extraParams,
							bin.extraMountPoints,
						)
						if err != nil {
							ctx.Log.Error(
								"failed to run",
								"caseName", caseData.Name,
								"interface", bin.name,
								"err", err,
							)
						}
						generatedFiles := []string{
							path.Join(
								bin.outputPathPrefix,
								fmt.Sprintf("%s.out", caseData.Name),
							),
							path.Join(
								bin.outputPathPrefix,
								fmt.Sprintf("%s.err", caseData.Name),
							),
							path.Join(
								bin.outputPathPrefix,
								fmt.Sprintf("%s.meta", caseData.Name),
							),
						}
						ctx.EventCollector.Add(singleBinary)
						metaChan <- intermediateRunResult{
							bin.name,
							runMeta,
							bin.binaryType,
							generatedFiles,
						}
					}(bin, &caseData)
				}
				var parentMetadata *RunMetadata
				chosenMetadata := RunMetadata{
					Verdict: "OK",
				}
				chosenMetadataEmpty := true
				var finalVerdict = "OK"
				var totalTime float64
				var totalWallTime float64
				var totalMemory base.Byte
				for i := 0; i < regularBinaryCount; i++ {
					intermediateResult := <-metaChan
					generatedFiles = append(generatedFiles, intermediateResult.generatedFiles...)
					if regularBinaryCount != 1 {
						// Only populate invidualMeta if there is more than one binary.
						individualMeta[intermediateResult.name] = *intermediateResult.runMeta
					}
					if intermediateResult.binaryType == binaryProblemsetter {
						parentMetadata = intermediateResult.runMeta
					} else {
						if intermediateResult.runMeta.Verdict != "OK" {
							if chosenMetadataEmpty {
								chosenMetadata = *intermediateResult.runMeta
								chosenMetadataEmpty = false
							}
						}
						finalVerdict = worseVerdict(
							finalVerdict,
							intermediateResult.runMeta.Verdict,
						)
						totalTime += intermediateResult.runMeta.Time
						totalWallTime = math.Max(
							totalWallTime,
							intermediateResult.runMeta.WallTime,
						)
						totalMemory += base.MaxBytes(totalMemory, intermediateResult.runMeta.Memory)
					}
				}
				close(metaChan)
				ctx.EventCollector.Add(singleRunEvent)
				chosenMetadata.Verdict = finalVerdict
				chosenMetadata.Time = totalTime
				chosenMetadata.WallTime = totalWallTime
				chosenMetadata.Memory = totalMemory

				if parentMetadata != nil && parentMetadata.Verdict != "OK" &&
					chosenMetadata.Verdict == "OK" {
					ctx.Log.Warn(
						"child process finished correctly, but parent did not",
						"parent", parentMetadata,
					)
					if parentMetadata.Verdict == "OLE" {
						chosenMetadata.Verdict = "OLE"
					} else if parentMetadata.Verdict == "TLE" {
						chosenMetadata.Verdict = "TLE"
					} else if parentMetadata.ExitStatus == 239 {
						// Child died before finishing message
						chosenMetadata.Verdict = "RTE"
					} else if parentMetadata.ExitStatus == 240 {
						// Child sent invalid cookie
						chosenMetadata.Verdict = "RTE"
					} else if parentMetadata.ExitStatus == 241 {
						// Child sent invalid message id
						chosenMetadata.Verdict = "RTE"
					} else if parentMetadata.ExitStatus == 242 {
						// Child terminated without replying call.
						chosenMetadata.Verdict = "RTE"
					} else if parentMetadata.Signal != nil &&
						*parentMetadata.Signal == "SIGPIPE" {
						// Child unexpectedly closed the pipe.
						chosenMetadata.Verdict = "RTE"
					} else {
						chosenMetadata.Verdict = "JE"
					}
				}

				runMeta = &chosenMetadata
			}
			runResult.Verdict = worseVerdict(runResult.Verdict, runMeta.Verdict)
			runResult.Time += runMeta.Time
			runResult.WallTime += runMeta.WallTime
			runResult.Memory = base.MaxBytes(runResult.Memory, runMeta.Memory)

			// TODO: change CaseResult to split original metadatas and final metadata
			caseResults[j] = CaseResult{
				Name:           caseData.Name,
				Verdict:        runMeta.Verdict,
				Meta:           *runMeta,
				IndividualMeta: individualMeta,

				Score:        &big.Rat{},
				ContestScore: &big.Rat{},
				MaxScore: new(big.Rat).Mul(
					runResult.MaxScore,
					new(big.Rat).Mul(caseData.Weight, totalWeightFactor),
				),
			}
		}
		groupResults[i] = GroupResult{
			Group: group.Name,
			Cases: caseResults,

			Score:        &big.Rat{},
			ContestScore: &big.Rat{},
			MaxScore: new(big.Rat).Mul(
				runResult.MaxScore,
				new(big.Rat).Mul(group.Weight(), totalWeightFactor),
			),
		}
	}
	ctx.EventCollector.Add(ctx.EventFactory.NewEvent("run", common.EventEnd))

	// Validate outputs.
	ctx.EventCollector.Add(ctx.EventFactory.NewEvent("validate", common.EventBegin))
	for i, group := range settings.Cases {
		correct := true
		score := &big.Rat{}
		for j, caseData := range group.Cases {
			caseResults := &groupResults[i].Cases[j]
			if caseResults.Verdict == "OK" {
				contestantPath := path.Join(
					runRoot, fmt.Sprintf("%s.out", caseData.Name),
				)
				if settings.Validator.Name == "custom" {
					originalInputFile := path.Join(
						input.Path(),
						"cases",
						fmt.Sprintf("%s.in", caseData.Name),
					)
					originalOutputFile := path.Join(
						input.Path(),
						"cases",
						fmt.Sprintf("%s.out", caseData.Name),
					)
					if _, err := os.Stat(originalOutputFile); os.IsNotExist(err) {
						ctx.Metrics.CounterAdd("runner_validator_errors", 1)
						ctx.Log.Info(
							"original file did not exist, using /dev/null",
							"case name", caseData.Name,
						)
						originalOutputFile = "/dev/null"
					}
					runMetaFile := path.Join(runRoot, fmt.Sprintf("%s.meta", caseData.Name))
					validateMeta, err := sandbox.Run(
						ctx,
						validatorLimits(&settings.Limits, settings.Validator.Limits),
						*settings.Validator.Lang,
						validatorBinPath,
						contestantPath,
						path.Join(runRoot, "validator", fmt.Sprintf("%s.out", caseData.Name)),
						path.Join(runRoot, "validator", fmt.Sprintf("%s.err", caseData.Name)),
						path.Join(runRoot, "validator", fmt.Sprintf("%s.meta", caseData.Name)),
						"validator",
						&originalInputFile,
						&originalOutputFile,
						&runMetaFile,
						[]string{caseData.Name, run.Language},
						map[string]string{},
					)
					if err != nil {
						ctx.Log.Error(
							"failed to validate",
							"case name", caseData.Name,
							"err", err,
						)
					}
					caseResults.IndividualMeta["validator"] = *validateMeta
					generatedFiles = append(
						generatedFiles,
						fmt.Sprintf("validator/%s.out", caseData.Name),
						fmt.Sprintf("validator/%s.err", caseData.Name),
						fmt.Sprintf("validator/%s.meta", caseData.Name),
					)
					if validateMeta.Verdict != "OK" {
						// If the validator did not exit cleanly, assume an empty output.
						ctx.Log.Info(
							"validator verdict not OK. Using /dev/null",
							"case name", caseData.Name,
							"meta", validateMeta,
						)
						contestantPath = "/dev/null"
					} else {
						contestantPath = path.Join(
							runRoot,
							"validator",
							fmt.Sprintf("%s.out", caseData.Name),
						)
					}
				}
				contestantFd, err := os.Open(contestantPath)
				if err != nil {
					ctx.Log.Warn("Error opening contestant file", "path", contestantPath, "err", err)
					continue
				}
				defer contestantFd.Close()
				expectedPath := path.Join(
					input.Path(), "cases", fmt.Sprintf("%s.out", caseData.Name),
				)
				if settings.Validator.Name == "custom" {
					// No need to open the actual file. It might not even exist.
					expectedPath = "/dev/null"
				}
				expectedFd, err := os.Open(expectedPath)
				if err != nil {
					ctx.Log.Warn("Error opening expected file", "path", expectedPath, "err", err)
					continue
				}
				defer expectedFd.Close()
				runScore, _, err := CalculateScore(
					&settings.Validator,
					expectedFd,
					contestantFd,
				)
				if err != nil {
					ctx.Log.Debug(
						"error comparing values",
						"case", caseData.Name,
						"err", err,
					)
				}
				caseResults.Score.Add(caseResults.Score, runScore)
				caseResults.ContestScore = new(big.Rat).Mul(
					new(big.Rat).Mul(
						runResult.MaxScore,
						new(big.Rat).Mul(caseData.Weight, totalWeightFactor),
					),
					caseResults.Score,
				)
				score.Add(
					score,
					new(big.Rat).Mul(
						runScore,
						new(big.Rat).Mul(caseData.Weight, totalWeightFactor),
					),
				)
				if runScore.Cmp(big.NewRat(1, 1)) == 0 {
					caseResults.Verdict = "AC"
				} else {
					runResult.Verdict = worseVerdict(runResult.Verdict, "PA")
					if runScore.Cmp(&big.Rat{}) == 0 {
						correct = false
						caseResults.Verdict = "WA"
					} else {
						caseResults.Verdict = "PA"
					}
				}
			} else {
				correct = false
			}
		}
		if correct {
			runResult.Score.Add(runResult.Score, score)

			groupResults[i].Score.Add(groupResults[i].Score, score)
			groupResults[i].ContestScore = new(big.Rat).Mul(
				runResult.MaxScore,
				score,
			)
		}
	}
	ctx.EventCollector.Add(ctx.EventFactory.NewEvent("validate", common.EventEnd))

	runResult.Groups = groupResults

	if runResult.Verdict == "PA" && runResult.Score.Cmp(&big.Rat{}) == 0 {
		runResult.Verdict = "WA"
	} else if runResult.Verdict == "OK" {
		runResult.Verdict = "AC"
		runResult.Score = big.NewRat(1, 1)
	}
	runResult.ContestScore = new(big.Rat).Mul(
		runResult.MaxScore,
		runResult.Score,
	)

	ctx.Log.Debug(
		"Finished running",
		"id", run.AttemptID,
		"verdict", runResult.Verdict,
		"score", runResult.Score,
	)
	uploadEvent := ctx.EventFactory.NewCompleteEvent("upload")
	defer ctx.EventCollector.Add(uploadEvent)
	if err := uploadFiles(
		ctx,
		filesWriter,
		runRoot,
		input,
		generatedFiles,
	); err != nil {
		ctx.Log.Error("uploadFiles failed", "err", err)
		return runResult, err
	}

	return runResult, nil
}

func uploadFiles(
	ctx *common.Context,
	filesWriter io.Writer,
	runRoot string,
	input common.Input,
	files []string,
) error {
	if filesWriter == nil {
		return nil
	}
	path, err := createZipFile(runRoot, files)
	if err != nil {
		return err
	}

	fd, err := os.Open(path)
	if err != nil {
		return err
	}
	defer fd.Close()

	_, err = io.Copy(filesWriter, fd)
	return err
}

func createZipFile(runRoot string, files []string) (string, error) {
	zipFd, err := ioutil.TempFile(runRoot, ".results_zip")
	if err != nil {
		return "", err
	}
	defer zipFd.Close()

	zipPath := zipFd.Name()
	zip := zip.NewWriter(zipFd)
	for _, file := range files {
		f, err := os.Open(path.Join(runRoot, file))
		if err != nil {
			continue
		}
		defer f.Close()
		zf, err := zip.Create(file)
		if err != nil {
			zip.Close()
			return zipPath, err
		}
		if _, err := io.Copy(zf, f); err != nil {
			zip.Close()
			return zipPath, err
		}
	}
	return zipPath, zip.Close()
}

func getCompileError(errorFile string) string {
	fd, err := os.Open(errorFile)
	if err != nil {
		return err.Error()
	}
	defer fd.Close()
	bytes, err := ioutil.ReadAll(fd)
	if err != nil {
		return err.Error()
	}
	return string(bytes)
}

func worseVerdict(a, b string) string {
	idxA := sliceIndex(len(common.VerdictList),
		func(i int) bool { return common.VerdictList[i] == a })
	idxB := sliceIndex(len(common.VerdictList),
		func(i int) bool { return common.VerdictList[i] == b })
	return common.VerdictList[min(idxA, idxB)]
}

func sliceIndex(limit int, predicate func(i int) bool) int {
	for i := 0; i < limit; i++ {
		if predicate(i) {
			return i
		}
	}
	return -1
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
