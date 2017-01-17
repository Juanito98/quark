package v1compat

import (
	"archive/tar"
	"bufio"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/lhchavez/quark/common"
	git "github.com/libgit2/git2go"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

type SettingsLoader interface {
	Load(problemName string) (*common.ProblemSettings, error)
}

type graderBaseInput struct {
	common.BaseInput
	archivePath      string
	storedHash       string
	uncompressedSize int64
}

func (input *graderBaseInput) Verify() error {
	stat, err := os.Stat(input.archivePath)
	if err != nil {
		return err
	}
	hash, err := common.Sha1sum(input.archivePath)
	if err != nil {
		return err
	}
	storedHash, err := input.getStoredHash()
	if err != nil {
		return err
	}
	if storedHash != fmt.Sprintf("%0x", hash) {
		return errors.New("Hash verification failed")
	}
	uncompressedSize, err := input.getStoredLength()
	if err != nil {
		return err
	}

	input.storedHash = storedHash
	input.uncompressedSize = uncompressedSize
	input.Commit(stat.Size())
	return nil
}

func (input *graderBaseInput) getStoredHash() (string, error) {
	hashFd, err := os.Open(fmt.Sprintf("%s.sha1", input.archivePath))
	if err != nil {
		return "", err
	}
	defer hashFd.Close()
	scanner := bufio.NewScanner(hashFd)
	scanner.Split(bufio.ScanWords)
	if !scanner.Scan() {
		if scanner.Err() != nil {
			return "", scanner.Err()
		}
		return "", io.ErrUnexpectedEOF
	}
	return scanner.Text(), nil
}

func (input *graderBaseInput) getStoredLength() (int64, error) {
	lenFd, err := os.Open(fmt.Sprintf("%s.len", input.archivePath))
	if err != nil {
		return 0, err
	}
	defer lenFd.Close()
	scanner := bufio.NewScanner(lenFd)
	scanner.Split(bufio.ScanLines)
	if !scanner.Scan() {
		if scanner.Err() != nil {
			return 0, scanner.Err()
		}
		return 0, io.ErrUnexpectedEOF
	}
	return strconv.ParseInt(scanner.Text(), 10, 64)
}

func (input *graderBaseInput) Delete() error {
	os.Remove(fmt.Sprintf("%s.tmp", input.archivePath))
	os.Remove(fmt.Sprintf("%s.sha1", input.archivePath))
	os.Remove(fmt.Sprintf("%s.len", input.archivePath))
	return os.Remove(input.archivePath)
}

// Transmit sends a serialized version of the Input to the runner. It sends a
// .tar.gz file with the Content-SHA1 header with the hexadecimal
// representation of its SHA-1 hash.
func (input *graderBaseInput) Transmit(w http.ResponseWriter) error {
	fd, err := os.Open(input.archivePath)
	if err != nil {
		return err
	}
	defer fd.Close()
	w.Header().Add("Content-Type", "application/x-gzip")
	w.Header().Add("Content-SHA1", input.storedHash)
	w.Header().Add(
		"X-Content-Uncompressed-Size", strconv.FormatInt(input.uncompressedSize, 10),
	)
	w.WriteHeader(http.StatusOK)
	_, err = io.Copy(w, fd)
	return err
}

// graderInput is an Input generated from a git repository that is then stored
// in a .tar.gz file that can be sent to a runner.
type graderInput struct {
	graderBaseInput
	repositoryPath string
	problemName    string
	loader         SettingsLoader
}

func (input *graderInput) Persist() error {
	if err := os.MkdirAll(path.Dir(input.archivePath), 0755); err != nil {
		return err
	}
	tmpPath := fmt.Sprintf("%s.tmp", input.archivePath)
	defer os.Remove(tmpPath)
	settings, uncompressedSize, err := CreateArchiveFromGit(
		input.problemName,
		tmpPath,
		input.repositoryPath,
		input.Hash(),
		input.loader,
	)
	if err != nil {
		return err
	}

	stat, err := os.Stat(tmpPath)
	if err != nil {
		return err
	}

	hash, err := common.Sha1sum(tmpPath)
	if err != nil {
		return err
	}

	hashFd, err := os.Create(fmt.Sprintf("%s.sha1", input.archivePath))
	if err != nil {
		return err
	}
	defer hashFd.Close()

	if _, err := fmt.Fprintf(
		hashFd,
		"%0x *%s\n",
		hash,
		path.Base(input.archivePath),
	); err != nil {
		return err
	}

	sizeFd, err := os.Create(fmt.Sprintf("%s.len", input.archivePath))
	if err != nil {
		return err
	}
	defer sizeFd.Close()

	if _, err := fmt.Fprintf(sizeFd, "%d\n", uncompressedSize); err != nil {
		return err
	}

	if err := os.Rename(tmpPath, input.archivePath); err != nil {
		return err
	}

	*input.Settings() = *settings
	input.storedHash = fmt.Sprintf("%0x", hash)
	input.uncompressedSize = uncompressedSize
	input.Commit(stat.Size())
	return nil
}

func getLibinteractiveSettings(
	contents []byte,
	moduleName string,
	parentLang string,
) (*common.InteractiveSettings, error) {
	cmd := exec.Command(
		"/usr/bin/java",
		"-jar", "/usr/share/java/libinteractive.jar",
		"json",
		"--module-name", moduleName,
		"--parent-lang", parentLang,
		"--omit-debug-targets",
	)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	go (func() {
		stdin.Write(contents)
		stdin.Close()
	})()
	settings := common.InteractiveSettings{}
	if err := json.NewDecoder(stdout).Decode(&settings); err != nil {
		return nil, err
	}
	if err := cmd.Wait(); err != nil {
		return nil, err
	}
	return &settings, nil
}

func CreateArchiveFromGit(
	problemName string,
	archivePath string,
	repositoryPath string,
	inputHash string,
	loader SettingsLoader,
) (*common.ProblemSettings, int64, error) {
	settings, err := loader.Load(problemName)
	if err != nil {
		return nil, 0, err
	}
	if settings.Validator.Name == "token-numeric" {
		tolerance := 1e-6
		settings.Validator.Tolerance = &tolerance
	}

	repository, err := git.OpenRepository(repositoryPath)
	if err != nil {
		return nil, 0, err
	}
	defer repository.Free()

	treeOid, err := git.NewOid(inputHash)
	if err != nil {
		return nil, 0, err
	}

	tree, err := repository.LookupTree(treeOid)
	if err != nil {
		return nil, 0, err
	}
	defer tree.Free()
	odb, err := repository.Odb()
	if err != nil {
		return nil, 0, err
	}
	defer odb.Free()

	tmpFd, err := os.Create(archivePath)
	if err != nil {
		return nil, 0, err
	}
	defer tmpFd.Close()

	gz := gzip.NewWriter(tmpFd)
	defer gz.Close()

	archive := tar.NewWriter(gz)
	defer archive.Close()

	var walkErr error = nil
	var uncompressedSize int64 = 0
	rawCaseWeights := make(map[string]float64)
	var libinteractiveIdlContents []byte
	var libinteractiveModuleName string
	var libinteractiveParentLang string
	tree.Walk(func(parent string, entry *git.TreeEntry) int {
		untrimmedPath := path.Join(parent, entry.Name)
		if strings.HasPrefix(untrimmedPath, "interactive/") {
			if strings.HasSuffix(untrimmedPath, ".idl") &&
				entry.Type == git.ObjectBlob {
				var blob *git.Blob
				blob, walkErr = repository.LookupBlob(entry.Id)
				if walkErr != nil {
					return -1
				}
				defer blob.Free()
				libinteractiveIdlContents = blob.Contents()
				libinteractiveModuleName = strings.TrimSuffix(entry.Name, ".idl")
				hdr := &tar.Header{
					Name:     untrimmedPath,
					Typeflag: tar.TypeReg,
					Mode:     0644,
					Size:     blob.Size(),
				}
				uncompressedSize += blob.Size()
				if walkErr = archive.WriteHeader(hdr); walkErr != nil {
					return -1
				}
				if _, walkErr = archive.Write(libinteractiveIdlContents); walkErr != nil {
					return -1
				}
			} else if strings.HasPrefix(entry.Name, "Main.") &&
				!strings.HasPrefix(entry.Name, "Main.distrib.") &&
				entry.Type == git.ObjectBlob {
				var blob *git.Blob
				blob, walkErr = repository.LookupBlob(entry.Id)
				if walkErr != nil {
					return -1
				}
				defer blob.Free()
				libinteractiveParentLang = strings.TrimPrefix(entry.Name, "Main.")
				hdr := &tar.Header{
					Name:     untrimmedPath,
					Typeflag: tar.TypeReg,
					Mode:     0644,
					Size:     blob.Size(),
				}
				uncompressedSize += blob.Size()
				if walkErr = archive.WriteHeader(hdr); walkErr != nil {
					return -1
				}
				if _, walkErr = archive.Write(blob.Contents()); walkErr != nil {
					return -1
				}
			}
			return 0
		}
		if untrimmedPath == "testplan" && entry.Type == git.ObjectBlob {
			var blob *git.Blob
			blob, walkErr = repository.LookupBlob(entry.Id)
			if walkErr != nil {
				return -1
			}
			defer blob.Free()
			testplanRe := regexp.MustCompile(`^\s*([^# \t]+)\s+([0-9.]+).*$`)
			for _, line := range strings.Split(string(blob.Contents()), "\n") {
				m := testplanRe.FindStringSubmatch(line)
				if m == nil {
					continue
				}
				rawCaseWeights[m[1]], walkErr = strconv.ParseFloat(m[2], 64)
				if walkErr != nil {
					return -1
				}
			}
		}
		if strings.HasPrefix(untrimmedPath, "validator.") &&
			settings.Validator.Name == "custom" &&
			entry.Type == git.ObjectBlob {
			lang := strings.Trim(filepath.Ext(untrimmedPath), ".")
			settings.Validator.Lang = &lang
			var blob *git.Blob
			blob, walkErr = repository.LookupBlob(entry.Id)
			if walkErr != nil {
				return -1
			}
			defer blob.Free()
			hdr := &tar.Header{
				Name:     untrimmedPath,
				Typeflag: tar.TypeReg,
				Mode:     0644,
				Size:     blob.Size(),
			}
			uncompressedSize += blob.Size()
			if walkErr = archive.WriteHeader(hdr); walkErr != nil {
				return -1
			}
			if _, walkErr = archive.Write(blob.Contents()); walkErr != nil {
				return -1
			}
		}
		if !strings.HasPrefix(untrimmedPath, "cases/") {
			return 0
		}
		entryPath := strings.TrimPrefix(untrimmedPath, "cases/")
		if strings.HasPrefix(entryPath, "in/") {
			caseName := strings.TrimSuffix(strings.TrimPrefix(entryPath, "in/"), ".in")
			if _, ok := rawCaseWeights[caseName]; !ok {
				rawCaseWeights[caseName] = 1.0
			}
		}
		switch entry.Type {
		case git.ObjectTree:
			hdr := &tar.Header{
				Name:     entryPath,
				Typeflag: tar.TypeDir,
				Mode:     0755,
				Size:     0,
			}
			if walkErr = archive.WriteHeader(hdr); walkErr != nil {
				return -1
			}
		case git.ObjectBlob:
			var blob *git.Blob
			blob, walkErr = repository.LookupBlob(entry.Id)
			if walkErr != nil {
				return -1
			}
			defer blob.Free()

			hdr := &tar.Header{
				Name:     entryPath,
				Typeflag: tar.TypeReg,
				Mode:     0644,
				Size:     blob.Size(),
			}
			uncompressedSize += blob.Size()
			if walkErr = archive.WriteHeader(hdr); walkErr != nil {
				return -1
			}

			stream, err := odb.NewReadStream(entry.Id)
			if err == nil {
				defer stream.Free()
				if _, walkErr = io.Copy(archive, stream); walkErr != nil {
					return -1
				}
			} else {
				// That particular object cannot be streamed. Allocate the blob in
				// memory and write it to the archive.
				if _, walkErr = archive.Write(blob.Contents()); walkErr != nil {
					return -1
				}
			}
		}
		return 0
	})
	if walkErr != nil {
		return nil, 0, walkErr
	}

	// Generate the group/case settings.
	cases := make(map[string][]common.CaseSettings)
	groupWeights := make(map[string]float64)
	totalWeight := 0.0
	for _, weight := range rawCaseWeights {
		totalWeight += weight
	}
	for caseName, weight := range rawCaseWeights {
		components := strings.SplitN(caseName, ".", 2)
		groupName := components[0]
		if _, ok := groupWeights[groupName]; !ok {
			groupWeights[groupName] = 0
		}
		groupWeights[groupName] += weight / totalWeight
		if _, ok := cases[groupName]; !ok {
			cases[groupName] = make([]common.CaseSettings, 0)
		}
		cases[groupName] = append(cases[groupName], common.CaseSettings{
			Name:   caseName,
			Weight: weight / totalWeight,
		})
	}
	settings.Cases = make([]common.GroupSettings, 0)
	for groupName, cases := range cases {
		sort.Sort(common.ByCaseName(cases))
		settings.Cases = append(settings.Cases, common.GroupSettings{
			Cases:  cases,
			Name:   groupName,
			Weight: groupWeights[groupName],
		})
	}
	sort.Sort(common.ByGroupName(settings.Cases))

	if libinteractiveIdlContents != nil && libinteractiveParentLang != "" {
		settings.Interactive, err = getLibinteractiveSettings(
			libinteractiveIdlContents,
			libinteractiveModuleName,
			libinteractiveParentLang,
		)
		if err != nil {
			return nil, 0, err
		}
	}

	// Finally, write settings.json.
	settingsBlob, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return nil, 0, err
	}
	hdr := &tar.Header{
		Name:     "settings.json",
		Typeflag: tar.TypeReg,
		Mode:     0644,
		Size:     int64(len(settingsBlob)),
	}
	uncompressedSize += int64(len(settingsBlob))
	if err = archive.WriteHeader(hdr); err != nil {
		return nil, 0, err
	}
	if _, err = archive.Write(settingsBlob); err != nil {
		return nil, 0, err
	}

	return settings, uncompressedSize, nil
}

// graderInputFactory is an InputFactory that can store specific versions of a
// problem's git repository into a .tar.gz file that can be easily shipped to
// runners.
type graderInputFactory struct {
	problemName string
	config      *common.Config
	loader      SettingsLoader
}

func NewGraderInputFactory(
	problemName string,
	config *common.Config,
	loader SettingsLoader,
) common.InputFactory {
	return &graderInputFactory{
		problemName: problemName,
		config:      config,
		loader:      loader,
	}
}

func (factory *graderInputFactory) NewInput(
	hash string,
	mgr *common.InputManager,
) common.Input {
	return &graderInput{
		graderBaseInput: graderBaseInput{
			BaseInput: *common.NewBaseInput(
				hash,
				mgr,
			),
			archivePath: path.Join(
				factory.config.Grader.RuntimePath,
				"cache",
				fmt.Sprintf("%s/%s.tar.gz", hash[:2], hash[2:]),
			),
		},
		repositoryPath: path.Join(
			factory.config.Grader.V1.RuntimePath,
			"problems.git",
			factory.problemName,
		),
		loader:      factory.loader,
		problemName: factory.problemName,
	}
}