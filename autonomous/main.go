package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unicode/utf16"
)

const (
	appName     = "cfe2cf"
	version     = "1.0.0"
	stepTimeout = 10 * time.Minute
)

var versionRe = regexp.MustCompile(`^(\d+)\.(\d+)\.(\d+)\.(\d+)$`)

func main() {
	fmt.Printf("%s v%s: convert extension .cfe to configuration .cf\n\n", appName, version)

	if len(os.Args) != 2 && len(os.Args) != 4 {
		usage()
		os.Exit(2)
	}

	src, err := filepath.Abs(os.Args[1])
	if err != nil {
		usage()
		os.Exit(2)
	}

	var extName, outCf string
	if len(os.Args) == 4 {
		extName = os.Args[2]
		outCf, err = filepath.Abs(os.Args[3])
		if err != nil {
			usage()
			os.Exit(2)
		}
	} else {
		fileName := strings.TrimSuffix(filepath.Base(src), filepath.Ext(src))
		extName = deriveExtName(fileName)
		outCf = filepath.Join(filepath.Dir(src), fileName+".cf")
	}

	if _, err := os.Stat(src); err != nil {
		failf("File not found: %s", src)
	}
	if !strings.EqualFold(filepath.Ext(src), ".cfe") {
		failf("A .cfe file is required, got: %s", filepath.Ext(src))
	}
	if extName == "" {
		failf("Empty extension name")
	}

	v8 := findPlatform()
	if v8 == "" {
		fmt.Println("[ERROR] 1C:Enterprise 8 platform not found.")
		fmt.Println("Searched: <ProgramFiles>\\1cv8, C:\\1cv8, D:\\1cv8, E:\\1cv8, F:\\1cv8")
		fmt.Println("Set CFE2CF_V8 env var to the version folder or bin\\1cv8.exe to override")
		os.Exit(1)
	}

	fmt.Println("Source file:    " + src)
	fmt.Println("Extension name: " + extName)
	fmt.Println("Output file:    " + outCf)
	fmt.Println("Platform:       " + v8)
	fmt.Println("------------------------------------------------------------")

	work, err := os.MkdirTemp("", "cfe2cf_*")
	if err != nil {
		failf("Cannot create temp dir: %v", err)
	}
	db := filepath.Join(work, "database")
	srcDir := filepath.Join(work, "src")
	if err := os.MkdirAll(db, 0o755); err != nil {
		failf("Cannot create temp database dir: %v", err)
	}
	if err := os.MkdirAll(srcDir, 0o755); err != nil {
		failf("Cannot create temp src dir: %v", err)
	}

	err = convert(v8, src, extName, outCf, work, db, srcDir)
	if err != nil {
		fmt.Println("[ERROR] " + err.Error())
		fmt.Printf("Temp files kept for diagnostics: %s\n", work)
		os.Exit(1)
	}

	fmt.Println("------------------------------------------------------------")
	fmt.Println("Done! Created file: " + outCf)
	os.Exit(0)
}

func convert(v8, src, extName, outCf, work, db, srcDir string) error {
	fmt.Println("[1/5] Creating temporary infobase...")
	log0 := filepath.Join(work, "0_create.log")
	if err := run(v8, []string{"CREATEINFOBASE", "File=" + db + ";", "/Out", log0}); err != nil {
		return fmt.Errorf("CREATEINFOBASE failed: %v\n%s", err, readLog(log0))
	}
	if _, err := os.Stat(filepath.Join(db, "1Cv8.1CD")); err != nil {
		return fmt.Errorf("CREATEINFOBASE failed (no 1Cv8.1CD)\n%s", readLog(log0))
	}

	fmt.Println("[2/5] Loading extension into temporary infobase...")
	log1 := filepath.Join(work, "1_load.log")
	if err := run(v8, []string{"DESIGNER", "/F", db, "/LoadCfg", src, "-Extension", extName, "/Out", log1}); err != nil {
		return fmt.Errorf("Extension load failed: %v\n%s", err, readLog(log1))
	}

	fmt.Println("[3/5] Dumping extension to source files...")
	log2 := filepath.Join(work, "2_dump.log")
	if err := run(v8, []string{"DESIGNER", "/F", db, "/DumpConfigToFiles", srcDir, "-Extension", extName, "/Out", log2}); err != nil {
		return fmt.Errorf("Dump to source files failed: %v\n%s", err, readLog(log2))
	}
	if _, err := os.Stat(filepath.Join(srcDir, "Configuration.xml")); err != nil {
		return fmt.Errorf("Dump to source files failed (no Configuration.xml)\n%s", readLog(log2))
	}

	fmt.Println("[4/5] Patching XML: removing extension markers...")
	if err := editSources(srcDir); err != nil {
		return err
	}

	fmt.Println("[5/5] Loading sources as configuration and saving .cf ...")
	log3 := filepath.Join(work, "3_load.log")
	if err := run(v8, []string{"DESIGNER", "/F", db, "/LoadConfigFromFiles", srcDir, "/Out", log3}); err != nil {
		return fmt.Errorf("Load of sources as configuration failed: %v\n%s", err, readLog(log3))
	}

	log4 := filepath.Join(work, "4_dump.log")
	if err := run(v8, []string{"DESIGNER", "/F", db, "/DumpCfg", outCf, "/Out", log4}); err != nil {
		return fmt.Errorf("Saving .cf failed: %v\n%s", err, readLog(log4))
	}
	if _, err := os.Stat(outCf); err != nil {
		return fmt.Errorf("Saving .cf failed (file not created)\n%s", readLog(log4))
	}

	os.RemoveAll(work)
	return nil
}

func usage() {
	fmt.Println("Usage:")
	fmt.Println("  cfe2cf.exe <file.cfe>")
	fmt.Println("      extension name is taken from the file name without _YYYYMMDD suffix,")
	fmt.Println("      the result is saved next to the source file")
	fmt.Println("  cfe2cf.exe <file.cfe> <ExtensionName> <output.cf>")
	fmt.Println("      explicit extension name and output path")
}

func failf(format string, args ...any) {
	fmt.Printf("[ERROR] "+format+"\n", args...)
	os.Exit(1)
}

func run(exe string, args []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), stepTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, exe, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: 0x08000000} // CREATE_NO_WINDOW
	err := cmd.Run()
	if ctx.Err() == context.DeadlineExceeded {
		return fmt.Errorf("timeout after %s", stepTimeout)
	}
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return fmt.Errorf("exit code %d", ee.ExitCode())
		}
		return err
	}
	return nil
}

func readLog(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	if len(b) >= 2 {
		if b[0] == 0xFF && b[1] == 0xFE {
			return decodeUTF16LE(b[2:])
		}
		if b[0] == 0xFE && b[1] == 0xFF {
			return decodeUTF16BE(b[2:])
		}
	}
	return string(bytes.TrimPrefix(b, []byte{0xEF, 0xBB, 0xBF}))
}

func decodeUTF16LE(b []byte) string {
	units := make([]uint16, 0, len(b)/2)
	for i := 0; i+1 < len(b); i += 2 {
		units = append(units, uint16(b[i])|uint16(b[i+1])<<8)
	}
	return string(utf16.Decode(units))
}

func decodeUTF16BE(b []byte) string {
	units := make([]uint16, 0, len(b)/2)
	for i := 0; i+1 < len(b); i += 2 {
		units = append(units, uint16(b[i])<<8|uint16(b[i+1]))
	}
	return string(utf16.Decode(units))
}

func readText(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		failf("Cannot read file %s: %v", path, err)
	}
	return string(bytes.TrimPrefix(b, []byte{0xEF, 0xBB, 0xBF}))
}

func writeTextBOM(path, s string) {
	var buf bytes.Buffer
	buf.Write([]byte{0xEF, 0xBB, 0xBF})
	buf.WriteString(s)
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		failf("Cannot write file %s: %v", path, err)
	}
}

func editSources(dir string) error {
	cfg := filepath.Join(dir, "Configuration.xml")
	if _, err := os.Stat(cfg); err != nil {
		return fmt.Errorf("Configuration.xml not found in %s", dir)
	}
	t := readText(cfg)
	for _, purpose := range []string{"Customization", "AddOn", "Patch"} {
		t = strings.ReplaceAll(t,
			"<ConfigurationExtensionPurpose>"+purpose+"</ConfigurationExtensionPurpose>", "")
	}
	t = strings.ReplaceAll(t, "ConfigurationExtensionCompatibilityMode", "CompatibilityMode")
	writeTextBOM(cfg, t)

	return filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.EqualFold(filepath.Ext(path), ".xml") {
			return nil
		}
		x := readText(path)
		y := strings.ReplaceAll(x, "<ObjectBelonging>Adopted</ObjectBelonging>", "")
		if y != x {
			writeTextBOM(path, y)
		}
		return nil
	})
}

func deriveExtName(name string) string {
	if len(name) > 9 && name[len(name)-9] == '_' {
		digits := true
		for _, r := range name[len(name)-8:] {
			if r < '0' || r > '9' {
				digits = false
				break
			}
		}
		if digits {
			return name[:len(name)-9]
		}
	}
	return name
}

func findPlatform() string {
	if env := os.Getenv("CFE2CF_V8"); env != "" {
		if st, err := os.Stat(env); err == nil && !st.IsDir() {
			return env
		}
		if cand := filepath.Join(env, "bin", "1cv8.exe"); fileExists(cand) {
			return cand
		}
	}

	roots := []string{
		os.Getenv("ProgramFiles"),
		os.Getenv("ProgramFiles(x86)"),
		os.Getenv("ProgramW6432"),
		`C:\`, `D:\`, `E:\`, `F:\`,
	}
	best := ""
	var bestV [4]int
	for _, r := range roots {
		if r == "" {
			continue
		}
		root := filepath.Join(r, "1cv8")
		entries, err := os.ReadDir(root)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			v, ok := parseVersion(e.Name())
			if !ok {
				continue
			}
			exe := filepath.Join(root, e.Name(), "bin", "1cv8.exe")
			if !fileExists(exe) {
				continue
			}
			if best == "" || versionGreater(v, bestV) {
				best, bestV = exe, v
			}
		}
	}
	return best
}

func parseVersion(s string) ([4]int, bool) {
	var v [4]int
	m := versionRe.FindStringSubmatch(s)
	if m == nil {
		return v, false
	}
	for i := 0; i < 4; i++ {
		n, err := strconv.Atoi(m[i+1])
		if err != nil {
			return v, false
		}
		v[i] = n
	}
	return v, true
}

func versionGreater(a, b [4]int) bool {
	for i := 0; i < 4; i++ {
		if a[i] != b[i] {
			return a[i] > b[i]
		}
	}
	return false
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
