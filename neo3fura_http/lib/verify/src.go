package verify

import (
	"bytes"
	"context"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"mime"
	"mime/multipart"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"neo3fura_http/lib/cli"
	log2 "neo3fura_http/lib/log"

	"github.com/nspcc-dev/neo-go/pkg/smartcontract/nef"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// T is the verification handler.
type T struct {
	Client *cli.T
}

// Resource limits.
const (
	maxRequestBytes int64 = 1_258_291 // 1.2 MiB, including multipart overhead.
	maxFileBytes    int64 = 786_432   // 768 KiB per file.
	maxTotalBytes   int64 = 1_048_576 // 1 MiB across all uploaded files.
	maxFieldBytes   int64 = 256       // 256 B per text field.
	maxFiles              = 20        // Max number of uploaded source files.
	maxRPCBytes     int64 = 1_048_576 // 1 MiB RPC response.
	maxNEFBytes     int64 = 131_072   // 130 KiB compiled NEF artifact.
	maxCompilerLog  int64 = 65_536    // 64 KiB captured compiler output.

	compileTimeout = 45 * time.Second
	rpcTimeout     = 8 * time.Second
	dbTimeout      = 5 * time.Second

	rateWindow            = time.Minute
	rateLimit             = 1
	maxConcurrentCompiles = 2 // Simultaneous compilations across the process.
)

var (
	contractHashRe = regexp.MustCompile(`^0x[0-9a-fA-F]{40}$`)
	fileNameRe     = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_.-]{0,62}\.(cs|py|csproj)$`)
	errTooLarge    = errors.New("size limit exceeded")

	// Allowlisted values extracted from an uploaded .csproj before it is
	// regenerated server-side. Restricting them to these charsets means the
	// values can be embedded verbatim in the generated XML without injection.
	frameworkRe      = regexp.MustCompile(`^[A-Za-z0-9.]{1,32}$`)
	packageNameRe    = regexp.MustCompile(`^[A-Za-z0-9.]{1,128}$`)
	packageVersionRe = regexp.MustCompile(`^[0-9][A-Za-z0-9.\-]{0,63}$`)

	// Global concurrency gate for the compilation step.
	compileSlots = make(chan struct{}, maxConcurrentCompiles)
	limiter      = newRateLimiter(rateLimit, rateWindow)
)

type compilerSpec struct {
	language         string              // "csharp" or "python".
	executable       string              // Absolute path to a trusted binary.
	frameworkVersion string              // Pinned Neo.SmartContract.Framework version (C#).
	commands         map[string][]string // Allowed CompileCommand -> fixed args.
}

var compilers = map[string]compilerSpec{
	"neo3-boa": {
		language:   "python",
		executable: "/go/application/venv/bin/neo3-boa",
		commands: map[string][]string{
			"neo3-boa": {},
		},
	},
	"Neo.Compiler.CSharp 3.0.0": {
		language:         "csharp",
		executable:       "/go/application/compiler/a/nccs",
		frameworkVersion: "3.0.0",
		commands: map[string][]string{
			"nccs":               {},
			"nccs --no-optimize": {"--no-optimize"},
		},
	},
	"Neo.Compiler.CSharp 3.0.2": {
		language:         "csharp",
		executable:       "/go/application/compiler/c/nccs",
		frameworkVersion: "3.0.2",
		commands: map[string][]string{
			"nccs":               {},
			"nccs --no-optimize": {"--no-optimize"},
		},
	},
	"Neo.Compiler.CSharp 3.0.3": {
		language:         "csharp",
		executable:       "/go/application/compiler/b/nccs",
		frameworkVersion: "3.0.3",
		commands: map[string][]string{
			"nccs":               {},
			"nccs --no-optimize": {"--no-optimize"},
		},
	},
	"Neo.Compiler.CSharp 3.1.0": {
		language:         "csharp",
		executable:       "/go/application/compiler/d/nccs",
		frameworkVersion: "3.1.0",
		commands: map[string][]string{
			"nccs":               {},
			"nccs --no-optimize": {"--no-optimize"},
		},
	},
}

type contractUpload struct {
	contract       string
	version        string
	compileCommand string
	files          []sourceFile
}

type sourceFile struct {
	name string
	code []byte
}

type chainState struct {
	compiler      string
	script        string
	id            int
	updateCounter int
}

type verifiedContract struct {
	Hash          string `bson:"hash"`
	ID            int    `bson:"id"`
	UpdateCounter int    `bson:"updatecounter"`
}

type contractSource struct {
	Hash          string `bson:"hash"`
	UpdateCounter int    `bson:"updatecounter"`
	FileName      string `bson:"filename"`
	Code          string `bson:"code"`
}

type response struct {
	Code int
	Msg  string
}

type apiError struct {
	status int
	code   int
	public string
	cause  error
}

func (e *apiError) Error() string {
	if e.cause == nil {
		return e.public
	}
	return e.public + ": " + e.cause.Error()
}

// outcome builds a business-outcome error: HTTP 200 plus a legacy numeric code
func outcome(code int, msg string) *apiError {
	return &apiError{status: http.StatusOK, code: code, public: msg}
}

// tooLarge builds a payload-size rejection (HTTP 413).
func tooLarge(msg string) *apiError {
	return &apiError{status: http.StatusRequestEntityTooLarge, public: msg}
}

// internalError builds a masked internal failure (HTTP 500).
func internalError(msg string, cause error) *apiError {
	return &apiError{status: http.StatusInternalServerError, public: msg, cause: cause}
}

// MultipleFile is the contract verification handler wired to "/upload".
func (me *T) MultipleFile(w http.ResponseWriter, r *http.Request) {
	setSecurityHeaders(w)

	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		writeJSON(w, http.StatusMethodNotAllowed, 0, "Only POST is allowed")
		return
	}
	if me == nil || me.Client == nil || me.Client.C_online == nil {
		log2.Info("Contract verification unavailable: database client is nil")
		writeJSON(w, http.StatusServiceUnavailable, 0, "Contract verification is temporarily unavailable")
		return
	}
	if !limiter.allow(remoteIP(r.RemoteAddr), time.Now()) {
		w.Header().Set("Retry-After", "60")
		writeJSON(w, http.StatusTooManyRequests, 0, "Too many verification requests")
		return
	}
	if r.ContentLength > maxRequestBytes {
		writeJSON(w, http.StatusRequestEntityTooLarge, 0, "Multipart request is too large")
		return
	}

	// Hard cap on the number of bytes read from the network.
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBytes)
	defer r.Body.Close()

	runtimeName, dbName, rpcURL, err := environment()
	if err != nil {
		log2.Infof("Invalid verifier environment: %v", err)
		writeJSON(w, http.StatusServiceUnavailable, 0, "Contract verification is not configured")
		return
	}

	// The temporary workspace is always removed, whatever happens next.
	upload, workDir, err := parseUpload(r)
	if workDir != "" {
		defer func() {
			if rmErr := os.RemoveAll(workDir); rmErr != nil {
				log2.Infof("Could not remove verification workspace: %v", rmErr)
			}
		}()
	}
	if err != nil {
		writeError(w, err)
		return
	}

	spec, ok := compilers[upload.version]
	if !ok {
		writeJSON(w, http.StatusOK, 0, "Unsupported compiler version")
		return
	}
	args, ok := spec.commands[upload.compileCommand]
	if !ok {
		writeJSON(w, http.StatusOK, 0, "Unsupported compile command")
		return
	}
	if err := validateFiles(upload.files, spec); err != nil {
		writeError(w, err)
		return
	}

	// Validate the contract on-chain BEFORE compiling: cheap RPC first, and it
	// also gives us the expected compiler and script for later comparison.
	chain, err := contractStateRPC(r.Context(), rpcURL, upload.contract)
	if err != nil {
		log2.Infof("Contract RPC validation failed: %v", err)
		writeError(w, err)
		return
	}
	if chain.compiler != upload.version {
		writeJSON(w, http.StatusOK, 7, "Compiler version error, Compiler version should be "+chain.compiler)
		return
	}

	exists, err := verificationExists(r.Context(), me.Client.C_online, dbName, upload.contract, chain.updateCounter)
	if err != nil {
		log2.Infof("Could not query existing verification: %v", err)
		writeJSON(w, http.StatusInternalServerError, 0, "Could not query contract verification")
		return
	}
	if exists {
		writeJSON(w, http.StatusOK, 6, "This contract has already been verified")
		return
	}

	// For C#, if a .csproj was uploaded it is sanitized and regenerated
	// server-side before compilation, stripping any MSBuild execution
	// directives.
	if spec.language == "csharp" {
		if err := prepareCSharpProject(workDir, upload.files, spec); err != nil {
			log2.Infof("Could not prepare project for %s: %v", upload.contract, err)
			writeError(w, err)
			return
		}
	}

	// Bound concurrent compilations process-wide.
	select {
	case compileSlots <- struct{}{}:
		defer func() { <-compileSlots }()
	case <-r.Context().Done():
		writeJSON(w, http.StatusOK, 1, "Request cancelled before compilation")
		return
	}

	compiled, err := compileContract(r.Context(), workDir, spec, args, upload.files)
	if err != nil {
		log2.Infof("Compilation failed for %s: %v", upload.contract, err)
		writeError(w, err)
		return
	}
	if subtle.ConstantTimeCompare([]byte(compiled), []byte(chain.script)) != 1 {
		writeJSON(w, http.StatusOK, 8, "Contract Source Code Verification error!")
		return
	}

	inserted, err := persistVerification(r.Context(), me.Client.C_online, dbName, upload, chain)
	if err != nil {
		log2.Infof("Could not persist verified contract: %v", err)
		writeJSON(w, http.StatusInternalServerError, 0, "Could not persist contract verification")
		return
	}
	if !inserted {
		writeJSON(w, http.StatusOK, 6, "This contract has already been verified")
		return
	}

	log2.Infof("Contract %s verified and stored in %s (%s)", upload.contract, dbName, runtimeName)
	writeJSON(w, http.StatusOK, 5, "Verify done and record verified contract in database!")
}

// parseUpload streams the multipart body, enforcing every size/count/format
// limit and writing each accepted file into a fresh 0700 temp directory. The
// workspace path is returned even on error so the caller can always clean up.
func parseUpload(r *http.Request) (contractUpload, string, error) {
	var upload contractUpload

	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || !strings.EqualFold(mediaType, "multipart/form-data") {
		return upload, "", outcome(0, "Content-Type must be multipart/form-data")
	}

	reader, err := r.MultipartReader()
	if err != nil {
		return upload, "", outcome(0, "Malformed multipart request")
	}

	workDir, err := os.MkdirTemp("", "neo-verify-*")
	if err != nil {
		return upload, "", internalError("Could not create workspace", err)
	}
	if err := os.Chmod(workDir, 0o700); err != nil {
		return upload, workDir, internalError("Could not secure workspace", err)
	}

	seenField := make(map[string]bool, 3)
	seenFile := make(map[string]bool, maxFiles)
	var totalBytes int64

	for {
		part, nextErr := reader.NextPart()
		if errors.Is(nextErr, io.EOF) {
			break
		}
		if nextErr != nil {
			return upload, workDir, classifyMultipartErr(nextErr)
		}

		if part.FileName() == "" {
			err = readField(part, &upload, seenField)
		} else {
			var f sourceFile
			f, err = readSourceFile(part, workDir, seenFile, &totalBytes)
			if err == nil {
				upload.files = append(upload.files, f)
			}
		}
		closeErr := part.Close()
		if err != nil {
			return upload, workDir, err
		}
		if closeErr != nil {
			return upload, workDir, classifyMultipartErr(closeErr)
		}
	}

	if !seenField["Contract"] || !seenField["Version"] || !seenField["CompileCommand"] {
		return upload, workDir, outcome(0, "Contract, Version and CompileCommand are required")
	}
	if !contractHashRe.MatchString(upload.contract) {
		return upload, workDir, outcome(0, "Contract must be a 20-byte hexadecimal script hash (0x + 40 hex)")
	}
	if len(upload.files) == 0 {
		return upload, workDir, outcome(0, "At least one source file is required")
	}
	return upload, workDir, nil
}

func readField(part *multipart.Part, upload *contractUpload, seen map[string]bool) error {
	name := part.FormName()
	if name != "Contract" && name != "Version" && name != "CompileCommand" {
		return outcome(0, "Unexpected multipart field: "+name)
	}
	if seen[name] {
		return outcome(0, "Duplicate multipart field: "+name)
	}

	raw, err := readLimited(part, maxFieldBytes)
	if err != nil {
		return tooLarge("Multipart field is too large")
	}
	text := strings.TrimSpace(string(raw))
	if text == "" || !utf8.ValidString(text) || strings.IndexByte(text, 0) >= 0 {
		return outcome(0, "Multipart field contains an invalid value")
	}

	seen[name] = true
	switch name {
	case "Contract":
		upload.contract = text
	case "Version":
		upload.version = text
	case "CompileCommand":
		upload.compileCommand = text
	}
	return nil
}

func readSourceFile(part *multipart.Part, workDir string, seen map[string]bool, total *int64) (sourceFile, error) {
	var f sourceFile

	name := part.FileName()
	if !validFileName(name) {
		return f, outcome(0, "Source file name is not allowed: "+name)
	}
	key := strings.ToLower(name)
	if seen[key] {
		return f, outcome(0, "Duplicate source file name: "+name)
	}
	if len(seen) >= maxFiles {
		return f, tooLarge("Too many source files")
	}

	code, err := readLimited(part, maxFileBytes)
	if err != nil {
		return f, tooLarge("Source file is too large: " + name)
	}
	// Reject empty and binary uploads. A NUL byte is a reliable, cheap binary
	// marker for the text-based source files this endpoint accepts.
	if len(code) == 0 || bytes.IndexByte(code, 0) >= 0 {
		return f, outcome(0, "Source file must be non-empty text (binary files are not allowed): "+name)
	}
	*total += int64(len(code))
	if *total > maxTotalBytes {
		return f, tooLarge("Combined source files are too large")
	}

	// re-verify the resolved path never escapes the workspace.
	dst, err := safeJoin(workDir, name)
	if err != nil {
		return f, outcome(0, "Invalid source file path")
	}
	if err := os.WriteFile(dst, code, 0o600); err != nil {
		return f, internalError("Could not store source file", err)
	}

	seen[key] = true
	f.name = name
	f.code = code
	return f, nil
}

// safeJoin joins base and name and guarantees the result stays inside base.
func safeJoin(base, name string) (string, error) {
	if name != filepath.Base(name) || strings.ContainsAny(name, `/\`) {
		return "", errors.New("name is not a bare file name")
	}
	joined := filepath.Join(base, name)
	rel, err := filepath.Rel(base, joined)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) || filepath.IsAbs(rel) {
		return "", errors.New("path escapes workspace")
	}
	return joined, nil
}

func validFileName(name string) bool {
	if name == "" || name != filepath.Base(name) || strings.ContainsAny(name, `/\:`) {
		return false
	}
	if strings.Contains(name, "..") {
		return false
	}
	return fileNameRe.MatchString(name)
}

// validateFiles enforces the per-language extension policy.
func validateFiles(files []sourceFile, spec compilerSpec) error {
	// Python.
	if spec.language == "python" {
		pyCount := 0
		for _, f := range files {
			if strings.ToLower(filepath.Ext(f.name)) != ".py" {
				return outcome(0, "Python contracts only accept .py files")
			}
			pyCount++
		}
		if pyCount == 0 {
			return outcome(0, "At least one .py source file is required")
		}
		return nil
	}

	// C#: at least one source file; the .csproj project file is optional.
	csProjCount, csFileCount := 0, 0
	for _, f := range files {
		switch strings.ToLower(filepath.Ext(f.name)) {
		case ".cs":
			csFileCount++
		case ".csproj":
			csProjCount++
		default:
			return outcome(0, "C# contracts only accept .cs and .csproj files")
		}
	}
	if csProjCount > 1 {
		return outcome(0, "At most one .csproj project file is allowed")
	}
	if csFileCount == 0 {
		return outcome(0, "At least one .cs source file is required")
	}
	return nil
}

type neoPackage struct {
	include string
	version string
}

// prepareCSharpProject sanitizes an uploaded C# project, when present.
// It is parsed only to extract the TargetFramework and the "Neo*"
// PackageReferences, and a clean project file is written back under the SAME name
// containing no MSBuild Target/Exec/Import/UsingTask extension points.
func prepareCSharpProject(workDir string, files []sourceFile, spec compilerSpec) error {
	var proj *sourceFile
	for i := range files {
		if strings.EqualFold(filepath.Ext(files[i].name), ".csproj") {
			proj = &files[i]
			break
		}
	}
	if proj == nil {
		// No project file uploaded: nothing to sanitize.
		return nil
	}

	framework, packages, err := extractCSProjData(proj.code, spec.frameworkVersion)
	if err != nil {
		return err
	}

	dst, err := safeJoin(workDir, proj.name)
	if err != nil {
		return outcome(0, "Invalid project file path")
	}
	// os.WriteFile truncates, replacing the untrusted upload on disk before the
	// compiler ever runs.
	if err := os.WriteFile(dst, buildCSProj(framework, packages), 0o600); err != nil {
		return internalError("Could not write controlled project file", err)
	}
	return nil
}

// extractCSProjData parses an uploaded .csproj and returns the TargetFramework
// and the allowlisted "Neo*" package references. Every value is validated so it
// can be embedded verbatim in the regenerated XML.
func extractCSProjData(data []byte, defaultFrameworkVersion string) (string, []neoPackage, error) {
	var doc struct {
		PropertyGroups []struct {
			TargetFramework string `xml:"TargetFramework"`
		} `xml:"PropertyGroup"`
		ItemGroups []struct {
			PackageReferences []struct {
				Include string `xml:"Include,attr"`
				Version string `xml:"Version,attr"`
			} `xml:"PackageReference"`
		} `xml:"ItemGroup"`
	}
	if err := xml.Unmarshal(data, &doc); err != nil {
		return "", nil, outcome(0, "Contract project file must be well-formed XML")
	}

	framework := "net5.0"
	for _, pg := range doc.PropertyGroups {
		if tf := strings.TrimSpace(pg.TargetFramework); tf != "" {
			framework = tf
			break
		}
	}
	if !frameworkRe.MatchString(framework) {
		return "", nil, outcome(0, "Contract project declares an invalid TargetFramework")
	}

	var packages []neoPackage
	for _, ig := range doc.ItemGroups {
		for _, pr := range ig.PackageReferences {
			include := strings.TrimSpace(pr.Include)
			if !strings.HasPrefix(include, "Neo") {
				continue
			}
			version := strings.TrimSpace(pr.Version)
			if !packageNameRe.MatchString(include) || !packageVersionRe.MatchString(version) {
				return "", nil, outcome(0, "Contract project declares an invalid Neo package reference")
			}
			packages = append(packages, neoPackage{include: include, version: version})
		}
	}
	// Ensure the contract framework is always present, even if the upload omitted
	// it, so the project can be restored and built.
	if len(packages) == 0 {
		packages = append(packages, neoPackage{include: "Neo.SmartContract.Framework", version: defaultFrameworkVersion})
	}
	return framework, packages, nil
}

// buildCSProj renders a minimal, safe SDK-style project.
func buildCSProj(framework string, packages []neoPackage) []byte {
	var b strings.Builder
	b.WriteString("<Project Sdk=\"Microsoft.NET.Sdk\">\n")
	b.WriteString("  <PropertyGroup>\n")
	b.WriteString("    <TargetFramework>" + framework + "</TargetFramework>\n")
	b.WriteString("    <RestoreIgnoreFailedSources>true</RestoreIgnoreFailedSources>\n")
	b.WriteString("  </PropertyGroup>\n")
	b.WriteString("  <ItemGroup>\n")
	for _, p := range packages {
		b.WriteString(fmt.Sprintf("    <PackageReference Include=\"%s\" Version=\"%s\" />\n", p.include, p.version))
	}
	b.WriteString("  </ItemGroup>\n")
	b.WriteString("</Project>\n")
	return []byte(b.String())
}

// compileContract runs the allowlisted compiler under a timeout, with no shell,
// bounded output, and returns the base64-encoded NEF script on success.
func compileContract(parent context.Context, workDir string, spec compilerSpec, args []string, files []sourceFile) (string, error) {
	ctx, cancel := context.WithTimeout(parent, compileTimeout)
	defer cancel()

	// Build the target list from validated, allowlisted file names only.
	cmdArgs := append([]string(nil), args...)
	switch spec.language {
	case "python":
		// neo3-boa needs the source paths (no shell glob is involved).
		for _, f := range files {
			if strings.EqualFold(filepath.Ext(f.name), ".py") {
				cmdArgs = append(cmdArgs, f.name)
			}
		}
	case "csharp":
		// With a project file, nccs discovers it in the working directory. Without
		// one, the .cs sources are passed explicitly so they are built directly.
		if !hasExt(files, ".csproj") {
			for _, f := range files {
				if strings.EqualFold(filepath.Ext(f.name), ".cs") {
					cmdArgs = append(cmdArgs, f.name)
				}
			}
		}
	}

	cmd := exec.CommandContext(ctx, spec.executable, cmdArgs...)
	cmd.Dir = workDir
	cmd.Stdin = nil
	cmd.Env = compilerEnv()

	out := &limitedBuffer{limit: maxCompilerLog}
	cmd.Stdout = out
	cmd.Stderr = out

	err := cmd.Run()
	if ctx.Err() != nil {
		return "", &apiError{status: http.StatusOK, code: 1, public: "Cmd execution failed ", cause: ctx.Err()}
	}
	if err != nil {
		fmt.Printf("compile failed: %v; output:\n%s\n", err, out.String())
		return "", &apiError{status: http.StatusOK, code: 1, public: "Cmd execution failed ",
			cause: fmt.Errorf("%w; output: %s", err, out.String())}
	}

	nefPath, err := findCompiledNEF(workDir)
	if err != nil {
		return "", err
	}
	return readNEF(nefPath)
}

// hasExt reports whether any uploaded file has the given extension.
func hasExt(files []sourceFile, ext string) bool {
	for _, f := range files {
		if strings.EqualFold(filepath.Ext(f.name), ext) {
			return true
		}
	}
	return false
}

// findCompiledNEF locates the single NEF artifact produced in the workspace,
// independent of the project/source file naming.
func findCompiledNEF(workDir string) (string, error) {
	var matches []string
	walkErr := filepath.WalkDir(workDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.EqualFold(filepath.Ext(d.Name()), ".nef") {
			matches = append(matches, path)
		}
		return nil
	})
	if walkErr != nil {
		return "", &apiError{status: http.StatusOK, code: 2, public: ".nef file doesn't exist ", cause: walkErr}
	}
	switch len(matches) {
	case 0:
		return "", &apiError{status: http.StatusOK, code: 2, public: ".nef file doesn't exist "}
	case 1:
		return matches[0], nil
	default:
		// Prefer the SDK build output for determinism when several are present.
		for _, m := range matches {
			if strings.Contains(filepath.ToSlash(m), "/bin/sc/") {
				return m, nil
			}
		}
		return "", &apiError{status: http.StatusOK, code: 2, public: "Multiple .nef files produced "}
	}
}

// compilerEnv returns a minimal environment for the compiler process.
func compilerEnv() []string {
	env := []string{
		"PATH=/usr/local/bin:/usr/bin:/bin",
		"HOME=/tmp",
		"DOTNET_CLI_TELEMETRY_OPTOUT=1",
		"DOTNET_SKIP_FIRST_TIME_EXPERIENCE=1",
		"DOTNET_NOLOGO=1",
	}
	if home := os.Getenv("DOTNET_CLI_HOME"); home != "" {
		env = append(env, "DOTNET_CLI_HOME="+home)
	}
	return env
}

func readNEF(nefPath string) (string, error) {
	info, err := os.Lstat(nefPath)
	if err != nil {
		return "", &apiError{status: http.StatusOK, code: 2, public: ".nef file doesn't exist ", cause: err}
	}
	if !info.Mode().IsRegular() || info.Size() <= 0 || info.Size() > maxNEFBytes {
		return "", &apiError{status: http.StatusOK, code: 2, public: ".nef file is invalid "}
	}
	data, err := os.ReadFile(nefPath)
	if err != nil {
		return "", &apiError{status: http.StatusOK, code: 2, public: ".nef file could not be read ", cause: err}
	}
	parsed, err := nef.FileFromBytes(data)
	if err != nil {
		return "", &apiError{status: http.StatusOK, code: 2, public: ".nef file is malformed ", cause: err}
	}
	return base64.StdEncoding.EncodeToString(parsed.Script), nil
}

// contractStateRPC queries the rpc node for the contract state.
func contractStateRPC(parent context.Context, rpcURL, contract string) (chainState, error) {
	var state chainState

	payload, err := json.Marshal(map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "getcontractstate",
		"params":  []string{contract},
		"id":      1,
	})
	if err != nil {
		return state, internalError("Could not build RPC request", err)
	}

	ctx, cancel := context.WithTimeout(parent, rpcTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, rpcURL, bytes.NewReader(payload))
	if err != nil {
		return state, internalError("Could not build RPC request", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: rpcTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return state, &apiError{status: http.StatusOK, code: 3, public: "RPC Node error ", cause: err}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
		return state, &apiError{status: http.StatusOK, code: 3, public: "RPC Node error "}
	}

	body, err := readLimited(resp.Body, maxRPCBytes)
	if err != nil {
		return state, &apiError{status: http.StatusOK, code: 3, public: "RPC Node error ", cause: err}
	}

	var rpc struct {
		Result struct {
			NEF struct {
				Script   string `json:"script"`
				Compiler string `json:"compiler"`
			} `json:"nef"`
			UpdateCounter int `json:"updatecounter"`
			ID            int `json:"id"`
		} `json:"result"`
		Error *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &rpc); err != nil {
		return state, &apiError{status: http.StatusOK, code: 3, public: "RPC Node error ", cause: err}
	}
	if rpc.Error != nil {
		return state, &apiError{status: http.StatusOK, code: 4, public: rpc.Error.Message}
	}
	if rpc.Result.NEF.Script == "" || rpc.Result.NEF.Compiler == "" {
		return state, &apiError{status: http.StatusOK, code: 3, public: "RPC Node error "}
	}
	if _, err := base64.StdEncoding.DecodeString(rpc.Result.NEF.Script); err != nil {
		return state, &apiError{status: http.StatusOK, code: 3, public: "RPC Node error ", cause: err}
	}

	state.compiler = rpc.Result.NEF.Compiler
	state.script = rpc.Result.NEF.Script
	state.id = rpc.Result.ID
	state.updateCounter = rpc.Result.UpdateCounter
	return state, nil
}

func verificationExists(parent context.Context, client *mongo.Client, dbName, contract string, updateCounter int) (bool, error) {
	ctx, cancel := context.WithTimeout(parent, dbTimeout)
	defer cancel()

	err := client.Database(dbName).
		Collection("VerifyContractModel").
		FindOne(ctx, bson.M{"hash": contract, "updatecounter": updateCounter}).
		Err()
	if errors.Is(err, mongo.ErrNoDocuments) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// persistVerification atomically records the verification via an upsert, then stores the source files.
// If storing the source fails, the marker is rolled back so it is never left orphaned.
func persistVerification(parent context.Context, client *mongo.Client, dbName string, upload contractUpload, chain chainState) (bool, error) {
	ctx, cancel := context.WithTimeout(parent, dbTimeout)
	defer cancel()

	verified := client.Database(dbName).Collection("VerifyContractModel")
	sources := client.Database(dbName).Collection("ContractSourceCode")
	filter := bson.M{"hash": upload.contract, "updatecounter": chain.updateCounter}

	res, err := verified.UpdateOne(
		ctx,
		filter,
		bson.M{"$setOnInsert": verifiedContract{Hash: upload.contract, ID: chain.id, UpdateCounter: chain.updateCounter}},
		options.Update().SetUpsert(true),
	)
	if err != nil {
		return false, err
	}
	if res.UpsertedCount == 0 {
		return false, nil // Concurrently verified by another request.
	}

	docs := make([]interface{}, 0, len(upload.files))
	for _, f := range upload.files {
		docs = append(docs, contractSource{
			Hash:          upload.contract,
			UpdateCounter: chain.updateCounter,
			FileName:      f.name,
			Code:          string(f.code),
		})
	}
	if _, err := sources.InsertMany(ctx, docs); err != nil {
		rbCtx, rbCancel := context.WithTimeout(context.Background(), dbTimeout)
		defer rbCancel()
		if _, rbErr := verified.DeleteOne(rbCtx, filter); rbErr != nil {
			log2.Infof("Could not roll back incomplete verification record: %v", rbErr)
		}
		return false, err
	}
	return true, nil
}

func environment() (runtimeName, dbName, rpcURL string, err error) {
	switch os.Getenv("RUNTIME") {
	case "staging":
		return "staging", "neofura", "https://neofura.ngd.network", nil
	case "test":
		return "test", "testneofura", "https://testneofura.ngd.network:444", nil
	default:
		return "", "", "", errors.New("RUNTIME must be 'staging' or 'test'")
	}
}

// readLimited reads at most limit bytes and reports an error if the source would exceed it.
func readLimited(reader io.Reader, limit int64) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(reader, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, errTooLarge
	}
	return data, nil
}

func classifyMultipartErr(err error) error {
	if errors.Is(err, errTooLarge) ||
		strings.Contains(strings.ToLower(err.Error()), "request body too large") {
		return tooLarge("Multipart request is too large")
	}
	return outcome(0, "Malformed multipart request")
}

func writeError(w http.ResponseWriter, err error) {
	var reqErr *apiError
	if errors.As(err, &reqErr) {
		if reqErr.status >= http.StatusInternalServerError && reqErr.cause != nil {
			log2.Infof("Contract verification internal error: %v", reqErr)
		}
		writeJSON(w, reqErr.status, reqErr.code, reqErr.public)
		return
	}
	log2.Infof("Unexpected contract verification error: %v", err)
	writeJSON(w, http.StatusInternalServerError, 0, "Unexpected contract verification error")
}

func writeJSON(w http.ResponseWriter, status, code int, message string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(response{Code: code, Msg: message}); err != nil {
		log2.Infof("Could not write verification response: %v", err)
	}
}

func setSecurityHeaders(w http.ResponseWriter) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
}

func remoteIP(remoteAddr string) string {
	if host, _, err := net.SplitHostPort(remoteAddr); err == nil {
		return host
	}
	if remoteAddr == "" {
		return "unknown"
	}
	return remoteAddr
}

// limitedBuffer captures at most `limit` bytes of compiler output and marks the
// result as truncated. It is safe for concurrent Stdout/Stderr writers.
type limitedBuffer struct {
	mu        sync.Mutex
	buf       bytes.Buffer
	limit     int64
	truncated bool
}

func (b *limitedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	n := len(p)
	remaining := b.limit - int64(b.buf.Len())
	if remaining <= 0 {
		b.truncated = true
		return n, nil
	}
	if int64(len(p)) > remaining {
		p = p[:remaining]
		b.truncated = true
	}
	_, _ = b.buf.Write(p)
	return n, nil
}

func (b *limitedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()

	s := b.buf.String()
	if b.truncated {
		s += " [output truncated]"
	}
	return s
}

// rateLimiter is a small fixed-window per-key limiter with opportunistic
// eviction to keep memory bounded without a background goroutine.
type rateEntry struct {
	windowStart time.Time
	count       int
	lastSeen    time.Time
}

type rateLimiter struct {
	mu      sync.Mutex
	entries map[string]rateEntry
	limit   int
	window  time.Duration
}

func newRateLimiter(limit int, window time.Duration) *rateLimiter {
	return &rateLimiter{entries: make(map[string]rateEntry), limit: limit, window: window}
}

func (l *rateLimiter) allow(key string, now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	e := l.entries[key]
	if e.windowStart.IsZero() || now.Sub(e.windowStart) >= l.window {
		e.windowStart = now
		e.count = 0
	}
	e.lastSeen = now
	if e.count >= l.limit {
		l.entries[key] = e
		return false
	}
	e.count++
	l.entries[key] = e

	if len(l.entries) > 1024 {
		for k, v := range l.entries {
			if now.Sub(v.lastSeen) > 2*l.window {
				delete(l.entries, k)
			}
		}
	}
	return true
}
